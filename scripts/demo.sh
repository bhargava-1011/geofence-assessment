#!/usr/bin/env bash
# Walks through the full flow. Keep the frontend open on the Live feed tab to see alerts arrive.
set -euo pipefail
API="https://geofence-api-i2tp.onrender.com"
id() { sed -E 's/.*"id":"([^"]+)".*/\1/'; }
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }

echo "== Create restricted geofence"
GEO=$(curl -sS -X POST "$API/geofences" -H 'Content-Type: application/json' -d '{
  "name":"Downtown Restricted Zone","description":"No-entry area","category":"restricted_zone",
  "coordinates":[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]}')
echo "$GEO"; GEO_ID=$(echo "$GEO" | id)

echo "== Register vehicle"
VEH=$(curl -sS -X POST "$API/vehicles" -H 'Content-Type: application/json' \
  -d "{\"vehicle_number\":\"KA-01-AB-$RANDOM\",\"driver_name\":\"John Doe\",\"vehicle_type\":\"truck\",\"phone\":\"+1234567890\"}")
echo "$VEH"; VEH_ID=$(echo "$VEH" | id)

echo "== Alert rule: both entry and exit, all vehicles"
curl -sS -X POST "$API/alerts/configure" -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO_ID\",\"event_type\":\"both\"}"; echo

move() {
  echo "== Move to $1, $2"
  curl -sS -X POST "$API/vehicles/location" -H 'Content-Type: application/json' \
    -d "{\"vehicle_id\":\"$VEH_ID\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"$(now)\"}"; echo
  sleep 1
}
move 37.7000 -122.4000   # outside
move 37.7800 -122.4150   # inside -> entry alert
move 37.7900 -122.4000   # outside -> exit alert

echo "== Current location"; curl -sS "$API/vehicles/location/$VEH_ID"; echo
echo "== History";          curl -sS "$API/violations/history?vehicle_id=$VEH_ID&limit=10"; echo
