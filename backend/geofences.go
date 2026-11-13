package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var validCategories = map[string]bool{
	"delivery_zone": true, "restricted_zone": true, "toll_zone": true, "customer_area": true,
}

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

func (s *Server) handleCreateGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var in createGeofenceRequest
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, start, "request body must be valid JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, start, "name is required")
		return
	}
	if !validCategories[in.Category] {
		writeError(w, http.StatusBadRequest, start, "category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	if err := validateCoordinates(in.Coordinates); err != nil {
		writeError(w, http.StatusBadRequest, start, err.Error())
		return
	}

	ctx := r.Context()
	wkt := polygonWKT(in.Coordinates)
	var valid bool
	if err := s.db.QueryRow(ctx, `SELECT ST_IsValid(ST_GeomFromText($1, 4326))`, wkt).Scan(&valid); err != nil {
		serverError(w, start, err)
		return
	}
	if !valid {
		writeError(w, http.StatusBadRequest, start, "polygon is invalid (self-intersecting or degenerate)")
		return
	}

	coordsJSON, _ := json.Marshal(in.Coordinates)
	id := newID("geo")
	_, err := s.db.Exec(ctx,
		`INSERT INTO geofences (id, name, description, category, coordinates, geom)
		 VALUES ($1, $2, $3, $4, $5, ST_GeomFromText($6, 4326))`,
		id, in.Name, strings.TrimSpace(in.Description), in.Category, string(coordsJSON), wkt)
	if err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, start, map[string]any{"id": id, "name": in.Name, "status": "active"})
}

func (s *Server) handleListGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	category := r.URL.Query().Get("category")
	if category != "" && !validCategories[category] {
		writeError(w, http.StatusBadRequest, start, "unknown category")
		return
	}
	rows, err := s.db.Query(r.Context(),
		`SELECT id, name, description, category, coordinates, created_at
		 FROM geofences WHERE status = 'active' AND ($1 = '' OR category = $1)
		 ORDER BY created_at DESC`, category)
	if err != nil {
		serverError(w, start, err)
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, name, desc, cat string
		var coords []byte
		var created time.Time
		if err := rows.Scan(&id, &name, &desc, &cat, &coords, &created); err != nil {
			serverError(w, start, err)
			return
		}
		out = append(out, map[string]any{
			"id": id, "name": name, "description": desc, "category": cat,
			"coordinates": json.RawMessage(coords), "created_at": created,
		})
	}
	if err := rows.Err(); err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusOK, start, map[string]any{"geofences": out})
}
