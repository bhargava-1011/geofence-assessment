# Geofencing and Real-time Vehicle Alerts

Go API + PostgreSQL/PostGIS + WebSocket alerts + React/Leaflet dashboard.

```
backend/    Go API (net/http, pgx, gorilla/websocket)
frontend/   React + Vite + Leaflet
scripts/    demo.sh (end-to-end curl walkthrough)
docker-compose.yml   db + backend + frontend
```

## Prerequisites
- Docker and Docker Compose (easiest path), or
- Go 1.22+, Node 20+, and a PostgreSQL 14+ instance with PostGIS

## Run everything with Docker Compose
```bash
docker compose up --build
```
- Frontend: http://localhost:5173
- API: http://localhost:8080 (health check: `GET /health`)
- WebSocket: `ws://localhost:8080/ws/alerts`

The schema (including the PostGIS extension and indexes) is applied automatically on backend start.

First-build note: the backend Dockerfile runs `go mod tidy` to generate `go.sum`. After the first successful
build, run `cd backend && go mod tidy` locally and commit `go.sum` for reproducible builds.

## Run without Docker
```bash
# database (any PostGIS-enabled Postgres)
docker run -d --name geo-db -p 5432:5432 -e POSTGRES_USER=geo -e POSTGRES_PASSWORD=geo -e POSTGRES_DB=geo postgis/postgis:16-3.4

# backend
cd backend
go mod tidy
export DATABASE_URL='postgres://geo:geo@localhost:5432/geo?sslmode=disable'
go run .

# frontend (new terminal)
cd frontend
cp .env.example .env
npm install
npm run dev        # http://localhost:5173
```

Backend environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | local postgres | Postgres connection string |
| `PORT` | `8080` | HTTP port |
| `CORS_ORIGIN` | `*` | Allowed browser origin (set to your frontend URL in production) |
| `RATE_LIMIT_RPS` | `50` | Per-IP rate limit on `POST /vehicles/location` |

Unit tests: `cd backend && go test ./...`

## API testing guide
Quick path: `./scripts/demo.sh` runs the whole flow. Manual examples:

```bash
API=http://localhost:8080

# 1. Create a geofence ([lat, lng] pairs, closed polygon)
curl -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name":"Downtown Delivery Zone","description":"Main delivery area","category":"delivery_zone",
  "coordinates":[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]}'

# 2. List geofences (optionally filter)
curl "$API/geofences?category=delivery_zone"

# 3. Register a vehicle
curl -X POST $API/vehicles -H 'Content-Type: application/json' \
  -d '{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'

# 4. List vehicles
curl $API/vehicles

# 5. Configure an alert (vehicle_id optional; event_type: entry | exit | both)
curl -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d '{"geofence_id":"geo_xxx","vehicle_id":"veh_xxx","event_type":"entry"}'

# 6. Update location (triggers geofence check and alerts)
curl -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d '{"vehicle_id":"veh_xxx","latitude":37.7800,"longitude":-122.4150,"timestamp":"2025-01-15T10:35:00Z"}'

# 7. Current location + geofences
curl $API/vehicles/location/veh_xxx

# 8. List alert rules
curl "$API/alerts?geofence_id=geo_xxx"

# 9. History (limit default 50, max 500; offset is an extra for pagination)
curl "$API/violations/history?vehicle_id=veh_xxx&start_date=2025-01-01T00:00:00Z&limit=100"
```

Watch live alerts from the terminal (needs `websocat`): `websocat ws://localhost:8080/ws/alerts`

Every response includes `time_ns` (handler execution time in nanoseconds, as a string). Errors return
`{"error": "...", "time_ns": "..."}` with 400 / 404 / 409 / 429 / 500 status codes.

## Frontend usage guide
1. **Geofences tab**: click "Draw on map", click corners on the map, name it, pick a category, create. Or paste `lat, lng` lines. Filter the list by category.
2. **Vehicles tab**: register vehicles; the list shows current location and which geofences each is inside. Click a vehicle to select it.
3. **Alert rules tab**: pick a geofence, optionally a vehicle, and entry / exit / both.
4. **Location tab**: pick a vehicle, click "Pick location on map" (or type coordinates), set the timestamp, send. Crossing a boundary with a matching rule fires an alert.
5. **Live feed tab + toasts**: alerts arrive over the WebSocket. The header shows connection state and the client reconnects automatically with backoff.
6. **History tab**: filter by vehicle, geofence, and date range; paginated.

## Architecture
- **Point-in-polygon**: done in PostGIS (`ST_Covers` on a `geometry(Polygon, 4326)` column with a GiST index). Boundary points count as inside. Input is `[lat, lng]`, PostGIS wants `(lng, lat)`, so coordinates are swapped once in `polygonWKT`.
- **Entry/exit detection**: `vehicle_current` stores the set of geofence IDs the vehicle was in at its last update. Each update, the new set is diffed against the old one. The row is locked with `SELECT ... FOR UPDATE` inside the transaction, so concurrent updates for one vehicle cannot double-fire or miss events. Out-of-order (older) timestamps are stored in history but do not change state or fire events.
- **Alerts**: after the transaction commits, a goroutine checks `alert_rules` and broadcasts matches through an in-memory WebSocket hub, so the HTTP response is never blocked. Every entry/exit is stored in `violations` (`alerted = true` when a rule fired).
- **Hub**: one read pump and one write pump per client, ping/pong keepalive, bounded send buffer; slow clients are dropped instead of blocking the broadcast.
- **Tables**: `geofences`, `vehicles`, `vehicle_locations` (history), `vehicle_current`, `alert_rules`, `violations`. See `backend/schema.sql`.

## Deployment

### 1. Push the backend image to Docker Hub
```bash
docker login
docker build -t YOUR_DOCKERHUB_USER/geofence-api:latest ./backend
docker push YOUR_DOCKERHUB_USER/geofence-api:latest
```

### 2. Database
Use a managed Postgres with PostGIS, for example Neon, Supabase, or Cloud SQL. Copy the connection string (use `sslmode=require`). The backend runs `CREATE EXTENSION IF NOT EXISTS postgis` itself; on some providers you may need to enable the extension once from their dashboard.

### 3. Backend host (pick one)
**Render** (simplest): New > Web Service > "Existing image from a registry" > `docker.io/YOUR_DOCKERHUB_USER/geofence-api:latest`. Set env `DATABASE_URL`, `CORS_ORIGIN` (your frontend URL). Health check path: `/health`. Render proxies WebSockets, so `wss://` works out of the box.

**Google Cloud Run**:
```bash
gcloud run deploy geofence-api --source ./backend --region asia-south1 \
  --allow-unauthenticated --max-instances 1 --timeout 3600 \
  --set-env-vars DATABASE_URL='postgres://...sslmode=require',CORS_ORIGIN='https://your-frontend.vercel.app'
```
Keep `--max-instances 1` (see limitations): the WebSocket hub is in memory.

### 4. Frontend (Vercel / Netlify / Cloudflare Pages)
- Root directory: `frontend`, build command `npm run build`, output `dist`
- Environment variable: `VITE_API_URL=https://your-backend-url` (no trailing slash). The WebSocket URL is derived from it (`https` becomes `wss`).
- Redeploy after changing the variable, since Vite bakes it in at build time.

## Known limitations and roadmap (useful for the video)
- WebSocket hub is in-process, so multiple backend instances would not share alerts. Fix: Redis pub/sub or Postgres `LISTEN/NOTIFY` (about half a day).
- No auth. Add API keys or JWT middleware (a few hours); the WebSocket would need a token query param.
- Rate limiter keeps per-IP buckets in memory forever and trusts `X-Forwarded-For`. Needs eviction and a trusted-proxy setting.
- Polygons are planar lat/lng: fine at city scale, not for geofences crossing the antimeridian or at very large scale (use `geography` type).
- `vehicle_locations` grows unbounded. Partition by time (or use TimescaleDB) and add retention.
- The frontend fetches each vehicle's location individually; a batch endpoint would scale better.
- No edit or delete endpoints for geofences, vehicles, or rules (not in the spec).
- Scaling to thousands of vehicles: batch location ingestion, queue (Kafka/NATS) in front of the geofence worker, cache active geofences in memory with an R-tree, shard by vehicle ID.
