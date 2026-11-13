import { useEffect, useRef } from 'react';
import { MapContainer, TileLayer, Polygon, Polyline, CircleMarker, Tooltip, useMap, useMapEvents } from 'react-leaflet';
import { CATEGORY_COLORS, label } from './ui';

function ClickHandler({ mode, onPoint }) {
  useMapEvents({
    click(e) {
      if (mode === 'none') return;
      onPoint([+e.latlng.lat.toFixed(6), +e.latlng.lng.toFixed(6)]);
    },
  });
  return null;
}

function FitOnce({ geofences }) {
  const map = useMap();
  const done = useRef(false);
  useEffect(() => {
    if (done.current || geofences.length === 0) return;
    done.current = true;
    map.fitBounds(geofences.flatMap((g) => g.coordinates), { padding: [40, 40] });
  }, [geofences, map]);
  return null;
}

export default function MapView({ geofences, vehicles, locations, selectedVehicle, mode, draft, picked, onPoint }) {
  return (
    <MapContainer center={[37.7799, -122.4144]} zoom={13} className={`map ${mode !== 'none' ? 'map-active' : ''}`}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <FitOnce geofences={geofences} />
      <ClickHandler mode={mode} onPoint={onPoint} />

      {geofences.map((g) => (
        <Polygon
          key={g.id}
          positions={g.coordinates}
          pathOptions={{ color: CATEGORY_COLORS[g.category], weight: 2, fillOpacity: 0.18 }}
        >
          <Tooltip sticky>{g.name} ({label(g.category)})</Tooltip>
        </Polygon>
      ))}

      {vehicles.map((v) => {
        const loc = locations[v.id]?.current_location;
        if (!loc) return null;
        const inside = locations[v.id].current_geofences || [];
        const restricted = inside.some((g) => g.category === 'restricted_zone');
        const selected = v.id === selectedVehicle;
        return (
          <CircleMarker
            key={v.id}
            center={[loc.latitude, loc.longitude]}
            radius={selected ? 11 : 8}
            pathOptions={{ color: '#fff', weight: 2, fillColor: restricted ? '#dc2626' : '#1e3a5f', fillOpacity: 1 }}
          >
            <Tooltip direction="top" offset={[0, -8]}>
              <strong>{v.vehicle_number}</strong> ({v.driver_name})
              <br />
              {inside.length ? `Inside: ${inside.map((g) => g.geofence_name).join(', ')}` : 'Outside all geofences'}
            </Tooltip>
          </CircleMarker>
        );
      })}

      {draft.length > 0 && (
        <>
          {draft.length >= 3 ? (
            <Polygon positions={draft} pathOptions={{ color: '#7c3aed', dashArray: '6', fillOpacity: 0.1 }} />
          ) : (
            <Polyline positions={draft} pathOptions={{ color: '#7c3aed', dashArray: '6' }} />
          )}
          {draft.map((p, i) => (
            <CircleMarker key={i} center={p} radius={5} pathOptions={{ color: '#7c3aed', fillColor: '#fff', fillOpacity: 1 }} />
          ))}
        </>
      )}

      {picked && (
        <CircleMarker center={picked} radius={9} pathOptions={{ color: '#f59e0b', fillColor: '#fde68a', fillOpacity: 0.9, weight: 3 }}>
          <Tooltip permanent direction="top" offset={[0, -8]}>New location</Tooltip>
        </CircleMarker>
      )}
    </MapContainer>
  );
}
