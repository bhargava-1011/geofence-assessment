package main

import (
	"net/http"
	"strconv"
	"time"
)

var validEventTypes = map[string]bool{"entry": true, "exit": true, "both": true}

func (s *Server) handleConfigureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var in struct {
		GeofenceID string  `json:"geofence_id"`
		VehicleID  *string `json:"vehicle_id"`
		EventType  string  `json:"event_type"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, start, "request body must be valid JSON")
		return
	}
	if in.GeofenceID == "" {
		writeError(w, http.StatusBadRequest, start, "geofence_id is required")
		return
	}
	if !validEventTypes[in.EventType] {
		writeError(w, http.StatusBadRequest, start, "event_type must be one of: entry, exit, both")
		return
	}
	if in.VehicleID != nil && *in.VehicleID == "" {
		in.VehicleID = nil
	}

	ctx := r.Context()
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM geofences WHERE id = $1)`, in.GeofenceID).Scan(&exists); err != nil {
		serverError(w, start, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, start, "geofence not found")
		return
	}
	if in.VehicleID != nil {
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vehicles WHERE id = $1)`, *in.VehicleID).Scan(&exists); err != nil {
			serverError(w, start, err)
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, start, "vehicle not found")
			return
		}
	}

	id := newID("alert")
	if _, err := s.db.Exec(ctx,
		`INSERT INTO alert_rules (id, geofence_id, vehicle_id, event_type) VALUES ($1,$2,$3,$4)`,
		id, in.GeofenceID, in.VehicleID, in.EventType); err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, start, map[string]any{
		"alert_id": id, "geofence_id": in.GeofenceID, "vehicle_id": in.VehicleID,
		"event_type": in.EventType, "status": "active",
	})
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()
	var f filter
	if v := q.Get("geofence_id"); v != "" {
		f.add("a.geofence_id = ?", v)
	}
	if v := q.Get("vehicle_id"); v != "" {
		f.add("a.vehicle_id = ?", v)
	}
	rows, err := s.db.Query(r.Context(),
		`SELECT a.id, a.geofence_id, g.name, a.vehicle_id, v.vehicle_number, a.event_type, a.status, a.created_at
		 FROM alert_rules a
		 JOIN geofences g ON g.id = a.geofence_id
		 LEFT JOIN vehicles v ON v.id = a.vehicle_id`+f.where()+` ORDER BY a.created_at DESC`, f.args...)
	if err != nil {
		serverError(w, start, err)
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, gid, gname, etype, status string
		var vid, vnum *string
		var created time.Time
		if err := rows.Scan(&id, &gid, &gname, &vid, &vnum, &etype, &status, &created); err != nil {
			serverError(w, start, err)
			return
		}
		out = append(out, map[string]any{
			"alert_id": id, "geofence_id": gid, "geofence_name": gname, "vehicle_id": vid,
			"vehicle_number": vnum, "event_type": etype, "status": status, "created_at": created,
		})
	}
	if err := rows.Err(); err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusOK, start, map[string]any{"alerts": out})
}

func (s *Server) handleViolationHistory(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	limit, offset := 50, 0
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, start, "limit must be a positive integer")
			return
		}
		limit = min(n, 500)
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, start, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	var f filter
	if v := q.Get("vehicle_id"); v != "" {
		f.add("vi.vehicle_id = ?", v)
	}
	if v := q.Get("geofence_id"); v != "" {
		f.add("vi.geofence_id = ?", v)
	}
	for param, op := range map[string]string{"start_date": "vi.recorded_at >= ?", "end_date": "vi.recorded_at <= ?"} {
		if v := q.Get(param); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				writeError(w, http.StatusBadRequest, start, param+" must be ISO 8601, e.g. 2025-01-01T00:00:00Z")
				return
			}
			f.add(op, t)
		}
	}

	ctx := r.Context()
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM violations vi`+f.where(), f.args...).Scan(&total); err != nil {
		serverError(w, start, err)
		return
	}

	args := append(append([]any{}, f.args...), limit, offset)
	rows, err := s.db.Query(ctx,
		`SELECT vi.id, vi.vehicle_id, v.vehicle_number, vi.geofence_id, g.name, vi.event_type,
		        vi.latitude, vi.longitude, vi.recorded_at
		 FROM violations vi
		 JOIN vehicles v ON v.id = vi.vehicle_id
		 JOIN geofences g ON g.id = vi.geofence_id`+f.where()+
			` ORDER BY vi.recorded_at DESC, vi.id DESC LIMIT $`+strconv.Itoa(len(f.args)+1)+` OFFSET $`+strconv.Itoa(len(f.args)+2), args...)
	if err != nil {
		serverError(w, start, err)
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, vid, vnum, gid, gname, etype string
		var lat, lng float64
		var ts time.Time
		if err := rows.Scan(&id, &vid, &vnum, &gid, &gname, &etype, &lat, &lng, &ts); err != nil {
			serverError(w, start, err)
			return
		}
		out = append(out, map[string]any{
			"id": id, "vehicle_id": vid, "vehicle_number": vnum, "geofence_id": gid, "geofence_name": gname,
			"event_type": etype, "latitude": lat, "longitude": lng, "timestamp": ts.UTC().Format(time.RFC3339),
		})
	}
	if err := rows.Err(); err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusOK, start, map[string]any{"violations": out, "total_count": total})
}

