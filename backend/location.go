package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type geofenceRef struct {
	ID       string
	Name     string
	Category string
}

type geoEvent struct {
	ViolationID string
	EventID     string
	EventType   string
	Timestamp   time.Time
	Geofence    geofenceRef
	Lat, Lng    float64
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// geofencesContaining runs the point-in-polygon test in PostGIS using the GiST index.
// ST_Covers counts points on the boundary as inside. Note PostGIS order is (lng, lat).
func geofencesContaining(ctx context.Context, q querier, lat, lng float64) ([]geofenceRef, error) {
	rows, err := q.Query(ctx,
		`SELECT id, name, category FROM geofences
		 WHERE status = 'active' AND ST_Covers(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		 ORDER BY id`, lng, lat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []geofenceRef{}
	for rows.Next() {
		var g geofenceRef
		if err := rows.Scan(&g.ID, &g.Name, &g.Category); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// detectEvents compares the previous and current geofence sets to produce entry/exit events.
func detectEvents(ctx context.Context, q querier, prevIDs []string, current []geofenceRef, ts time.Time, lat, lng float64) ([]geoEvent, error) {
	curSet := map[string]bool{}
	for _, g := range current {
		curSet[g.ID] = true
	}
	prevSet := map[string]bool{}
	for _, id := range prevIDs {
		prevSet[id] = true
	}

	var events []geoEvent
	mk := func(t string, g geofenceRef) geoEvent {
		return geoEvent{ViolationID: newID("viol"), EventID: newID("evt"), EventType: t, Timestamp: ts, Geofence: g, Lat: lat, Lng: lng}
	}
	for _, g := range current {
		if !prevSet[g.ID] {
			events = append(events, mk("entry", g))
		}
	}

	var exited []string
	for _, id := range prevIDs {
		if !curSet[id] {
			exited = append(exited, id)
		}
	}
	if len(exited) > 0 {
		rows, err := q.Query(ctx, `SELECT id, name, category FROM geofences WHERE id = ANY($1) ORDER BY id`, exited)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var g geofenceRef
			if err := rows.Scan(&g.ID, &g.Name, &g.Category); err != nil {
				return nil, err
			}
			events = append(events, mk("exit", g))
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (s *Server) handleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req struct {
		VehicleID string   `json:"vehicle_id"`
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		Timestamp string   `json:"timestamp"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, start, "request body must be valid JSON")
		return
	}
	if req.VehicleID == "" {
		writeError(w, http.StatusBadRequest, start, "vehicle_id is required")
		return
	}
	if req.Latitude == nil || req.Longitude == nil {
		writeError(w, http.StatusBadRequest, start, "latitude and longitude are required")
		return
	}
	lat, lng := *req.Latitude, *req.Longitude
	if lat < -90 || lat > 90 {
		writeError(w, http.StatusBadRequest, start, "latitude must be between -90 and 90")
		return
	}
	if lng < -180 || lng > 180 {
		writeError(w, http.StatusBadRequest, start, "longitude must be between -180 and 180")
		return
	}
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		writeError(w, http.StatusBadRequest, start, "timestamp must be ISO 8601, e.g. 2025-01-15T10:35:00Z")
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		serverError(w, start, err)
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var number, driver string
	err = tx.QueryRow(ctx, `SELECT vehicle_number, driver_name FROM vehicles WHERE id = $1`, req.VehicleID).Scan(&number, &driver)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, start, "vehicle not found")
		return
	} else if err != nil {
		serverError(w, start, err)
		return
	}

	// Lock this vehicle's state row so concurrent updates cannot produce duplicate/missed events.
	if _, err := tx.Exec(ctx, `INSERT INTO vehicle_current (vehicle_id) VALUES ($1) ON CONFLICT DO NOTHING`, req.VehicleID); err != nil {
		serverError(w, start, err)
		return
	}
	var prevIDs []string
	var prevTS *time.Time
	if err := tx.QueryRow(ctx, `SELECT geofence_ids, recorded_at FROM vehicle_current WHERE vehicle_id = $1 FOR UPDATE`, req.VehicleID).Scan(&prevIDs, &prevTS); err != nil {
		serverError(w, start, err)
		return
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, recorded_at) VALUES ($1,$2,$3,$4)`,
		req.VehicleID, lat, lng, ts); err != nil {
		serverError(w, start, err)
		return
	}

	current, err := geofencesContaining(ctx, tx, lat, lng)
	if err != nil {
		serverError(w, start, err)
		return
	}

	// Out-of-order (older) updates are stored in history but do not change current state or fire events.
	var events []geoEvent
	if prevTS == nil || !ts.Before(*prevTS) {
		ids := make([]string, 0, len(current))
		for _, g := range current {
			ids = append(ids, g.ID)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE vehicle_current SET latitude=$2, longitude=$3, recorded_at=$4, geofence_ids=$5 WHERE vehicle_id=$1`,
			req.VehicleID, lat, lng, ts, ids); err != nil {
			serverError(w, start, err)
			return
		}
		events, err = detectEvents(ctx, tx, prevIDs, current, ts, lat, lng)
		if err != nil {
			serverError(w, start, err)
			return
		}
		for _, ev := range events {
			if _, err := tx.Exec(ctx,
				`INSERT INTO violations (id, event_id, vehicle_id, geofence_id, event_type, latitude, longitude, recorded_at)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				ev.ViolationID, ev.EventID, req.VehicleID, ev.Geofence.ID, ev.EventType, lat, lng, ts); err != nil {
				serverError(w, start, err)
				return
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverError(w, start, err)
		return
	}

	// Alert delivery is asynchronous so it never blocks the HTTP response.
	if len(events) > 0 {
		go s.dispatchAlerts(req.VehicleID, number, driver, events)
	}

	cg := make([]map[string]any, 0, len(current))
	for _, g := range current {
		cg = append(cg, map[string]any{"geofence_id": g.ID, "geofence_name": g.Name, "status": "inside"})
	}
	writeJSON(w, http.StatusOK, start, map[string]any{
		"vehicle_id": req.VehicleID, "location_updated": true, "current_geofences": cg,
	})
}

// dispatchAlerts checks alert rules for each event and broadcasts matches over WebSocket.
func (s *Server) dispatchAlerts(vehicleID, number, driver string, events []geoEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ev := range events {
		var matched bool
		err := s.db.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM alert_rules
			   WHERE geofence_id = $1 AND status = 'active'
			     AND (vehicle_id IS NULL OR vehicle_id = $2)
			     AND (event_type = 'both' OR event_type = $3))`,
			ev.Geofence.ID, vehicleID, ev.EventType).Scan(&matched)
		if err != nil {
			log.Printf("alert rule lookup: %v", err)
			continue
		}
		if !matched {
			continue
		}
		if _, err := s.db.Exec(ctx, `UPDATE violations SET alerted = true WHERE id = $1`, ev.ViolationID); err != nil {
			log.Printf("mark alerted: %v", err)
		}
		s.hub.Broadcast(map[string]any{
			"event_id":   ev.EventID,
			"event_type": ev.EventType,
			"timestamp":  ev.Timestamp.UTC().Format(time.RFC3339),
			"vehicle":    map[string]any{"vehicle_id": vehicleID, "vehicle_number": number, "driver_name": driver},
			"geofence":   map[string]any{"geofence_id": ev.Geofence.ID, "geofence_name": ev.Geofence.Name, "category": ev.Geofence.Category},
			"location":   map[string]any{"latitude": ev.Lat, "longitude": ev.Lng},
		})
	}
}

func (s *Server) handleGetLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("vehicle_id")
	ctx := r.Context()

	var number string
	var lat, lng *float64
	var ts *time.Time
	var ids []string
	err := s.db.QueryRow(ctx,
		`SELECT v.vehicle_number, c.latitude, c.longitude, c.recorded_at, COALESCE(c.geofence_ids, '{}')
		 FROM vehicles v LEFT JOIN vehicle_current c ON c.vehicle_id = v.id WHERE v.id = $1`, id).
		Scan(&number, &lat, &lng, &ts, &ids)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, start, "vehicle not found")
		return
	} else if err != nil {
		serverError(w, start, err)
		return
	}

	var loc any
	if ts != nil && lat != nil && lng != nil {
		loc = map[string]any{"latitude": *lat, "longitude": *lng, "timestamp": ts.UTC().Format(time.RFC3339)}
	}

	cg := []map[string]any{}
	if len(ids) > 0 {
		rows, err := s.db.Query(ctx, `SELECT id, name, category FROM geofences WHERE id = ANY($1) ORDER BY name`, ids)
		if err != nil {
			serverError(w, start, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var g geofenceRef
			if err := rows.Scan(&g.ID, &g.Name, &g.Category); err != nil {
				serverError(w, start, err)
				return
			}
			cg = append(cg, map[string]any{"geofence_id": g.ID, "geofence_name": g.Name, "category": g.Category})
		}
	}
	writeJSON(w, http.StatusOK, start, map[string]any{
		"vehicle_id": id, "vehicle_number": number, "current_location": loc, "current_geofences": cg,
	})
}
