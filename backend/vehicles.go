package main

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var in struct {
		VehicleNumber string `json:"vehicle_number"`
		DriverName    string `json:"driver_name"`
		VehicleType   string `json:"vehicle_type"`
		Phone         string `json:"phone"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, start, "request body must be valid JSON")
		return
	}
	in.VehicleNumber = strings.TrimSpace(in.VehicleNumber)
	in.DriverName = strings.TrimSpace(in.DriverName)
	in.VehicleType = strings.TrimSpace(in.VehicleType)
	in.Phone = strings.TrimSpace(in.Phone)
	switch {
	case in.VehicleNumber == "":
		writeError(w, http.StatusBadRequest, start, "vehicle_number is required")
		return
	case in.DriverName == "":
		writeError(w, http.StatusBadRequest, start, "driver_name is required")
		return
	case in.VehicleType == "":
		writeError(w, http.StatusBadRequest, start, "vehicle_type is required")
		return
	case in.Phone == "":
		writeError(w, http.StatusBadRequest, start, "phone is required")
		return
	}

	id := newID("veh")
	_, err := s.db.Exec(r.Context(),
		`INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone) VALUES ($1,$2,$3,$4,$5)`,
		id, in.VehicleNumber, in.DriverName, in.VehicleType, in.Phone)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, start, "a vehicle with this vehicle_number already exists")
			return
		}
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, start, map[string]any{
		"id": id, "vehicle_number": in.VehicleNumber, "status": "active",
	})
}

func (s *Server) handleListVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rows, err := s.db.Query(r.Context(),
		`SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		 FROM vehicles ORDER BY created_at DESC`)
	if err != nil {
		serverError(w, start, err)
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, num, driver, vtype, phone, status string
		var created time.Time
		if err := rows.Scan(&id, &num, &driver, &vtype, &phone, &status, &created); err != nil {
			serverError(w, start, err)
			return
		}
		out = append(out, map[string]any{
			"id": id, "vehicle_number": num, "driver_name": driver, "vehicle_type": vtype,
			"phone": phone, "status": status, "created_at": created,
		})
	}
	if err := rows.Err(); err != nil {
		serverError(w, start, err)
		return
	}
	writeJSON(w, http.StatusOK, start, map[string]any{"vehicles": out})
}
