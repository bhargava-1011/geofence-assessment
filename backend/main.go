package main

import (
	"context"
	_ "embed"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

//go:embed schema.sql
var schemaSQL string

type Server struct {
	db  *pgxpool.Pool
	hub *Hub
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 1; i <= 30; i++ {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		log.Printf("waiting for database (attempt %d/30): %v", i, err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func main() {
	ctx := context.Background()
	pool, err := connectDB(ctx, env("DATABASE_URL", "postgres://geo:geo@localhost:5432/geo?sslmode=disable"))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		log.Fatalf("schema: %v", err)
	}

	s := &Server{db: pool, hub: NewHub()}
	rps, _ := strconv.ParseFloat(env("RATE_LIMIT_RPS", "50"), 64)
	limiter := newIPLimiter(rate.Limit(rps), int(rps*2))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, time.Now(), map[string]any{"status": "ok"})
	})
	mux.HandleFunc("POST /geofences", s.handleCreateGeofence)
	mux.HandleFunc("GET /geofences", s.handleListGeofences)
	mux.HandleFunc("POST /vehicles", s.handleCreateVehicle)
	mux.HandleFunc("GET /vehicles", s.handleListVehicles)
	mux.Handle("POST /vehicles/location", limiter.middleware(http.HandlerFunc(s.handleUpdateLocation)))
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", s.handleGetLocation)
	mux.HandleFunc("POST /alerts/configure", s.handleConfigureAlert)
	mux.HandleFunc("GET /alerts", s.handleListAlerts)
	mux.HandleFunc("GET /violations/history", s.handleViolationHistory)
	mux.HandleFunc("GET /ws/alerts", s.hub.ServeWS)

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           cors(env("CORS_ORIGIN", "*"), logging(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start))
	})
}

func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ipLimiter is a per-client token bucket (bonus: rate limiting on location updates).
type ipLimiter struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
	r  rate.Limit
	b  int
}

func newIPLimiter(r rate.Limit, b int) *ipLimiter {
	return &ipLimiter{m: map[string]*rate.Limiter{}, r: r, b: b}
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.m[ip]
	if !ok {
		lim = rate.NewLimiter(l.r, l.b)
		l.m[ip] = lim
	}
	return lim
}

func (l *ipLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if !l.get(ip).Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, time.Now(), "rate limit exceeded, slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
