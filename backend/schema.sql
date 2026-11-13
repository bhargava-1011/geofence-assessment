CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS geofences (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL CHECK (category IN ('delivery_zone','restricted_zone','toll_zone','customer_area')),
    coordinates JSONB NOT NULL,
    geom        geometry(Polygon, 4326) NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_geofences_geom ON geofences USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_geofences_category ON geofences (category);

CREATE TABLE IF NOT EXISTS vehicles (
    id             TEXT PRIMARY KEY,
    vehicle_number TEXT NOT NULL UNIQUE,
    driver_name    TEXT NOT NULL,
    vehicle_type   TEXT NOT NULL,
    phone          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Full location history
CREATE TABLE IF NOT EXISTS vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_locations_vehicle_time ON vehicle_locations (vehicle_id, recorded_at DESC);

-- Latest known state per vehicle (used to detect entry/exit transitions)
CREATE TABLE IF NOT EXISTS vehicle_current (
    vehicle_id   TEXT PRIMARY KEY REFERENCES vehicles(id),
    latitude     DOUBLE PRECISION,
    longitude    DOUBLE PRECISION,
    recorded_at  TIMESTAMPTZ,
    geofence_ids TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS alert_rules (
    id          TEXT PRIMARY KEY,
    geofence_id TEXT NOT NULL REFERENCES geofences(id),
    vehicle_id  TEXT REFERENCES vehicles(id),
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit','both')),
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rules_geofence ON alert_rules (geofence_id);

-- Every entry/exit event. alerted = true when a matching alert rule fired a real-time alert.
CREATE TABLE IF NOT EXISTS violations (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL UNIQUE,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id),
    geofence_id TEXT NOT NULL REFERENCES geofences(id),
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit')),
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    alerted     BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_violations_time ON violations (recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_violations_vehicle ON violations (vehicle_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_violations_geofence ON violations (geofence_id, recorded_at DESC);
