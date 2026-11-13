package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// writeJSON adds time_ns (handler execution time in nanoseconds) to every response.
func writeJSON(w http.ResponseWriter, status int, start time.Time, body map[string]any) {
	body["time_ns"] = strconv.FormatInt(time.Since(start).Nanoseconds(), 10)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, start time.Time, msg string) {
	writeJSON(w, status, start, map[string]any{"error": msg})
}

func serverError(w http.ResponseWriter, start time.Time, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, start, "internal server error")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// filter builds a parameterised WHERE clause. Use "?" as the placeholder in clauses.
type filter struct {
	clauses []string
	args    []any
}

func (f *filter) add(clause string, v any) {
	f.args = append(f.args, v)
	f.clauses = append(f.clauses, strings.ReplaceAll(clause, "?", "$"+strconv.Itoa(len(f.args))))
}

func (f *filter) where() string {
	if len(f.clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(f.clauses, " AND ")
}
