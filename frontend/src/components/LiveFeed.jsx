import { useEffect } from 'react';
import { CATEGORY_COLORS, Empty, fmtTime, label } from './ui';

export function AlertItem({ a }) {
  return (
    <li className="card alert-item" style={{ borderLeftColor: CATEGORY_COLORS[a.geofence.category] }}>
      <span className={`tag ${a.event_type}`}>{a.event_type}</span>
      <div>
        <strong>{a.vehicle.vehicle_number}</strong> ({a.vehicle.driver_name})
        <div>{a.event_type === 'entry' ? 'entered' : 'exited'} <strong>{a.geofence.geofence_name}</strong> <span className="muted">({label(a.geofence.category)})</span></div>
        <div className="muted">{fmtTime(a.timestamp)}</div>
        <div className="muted">{a.location.latitude.toFixed(5)}, {a.location.longitude.toFixed(5)}</div>
      </div>
    </li>
  );
}

export function LiveFeed({ alerts }) {
  return (
    <div>
      <h2>Live alerts</h2>
      {alerts.length === 0 && <Empty>Waiting for alerts. Create an alert rule, then update a vehicle location to trigger one.</Empty>}
      <ul className="cards">{alerts.map((a) => <AlertItem key={a.event_id} a={a} />)}</ul>
    </div>
  );
}

function Toast({ a, onDismiss }) {
  useEffect(() => {
    const t = setTimeout(() => onDismiss(a.event_id), 8000);
    return () => clearTimeout(t);
  }, [a.event_id, onDismiss]);
  return (
    <div className={`toast ${a.geofence.category === 'restricted_zone' ? 'toast-danger' : ''}`} onClick={() => onDismiss(a.event_id)}>
      <strong>{a.event_type === 'entry' ? 'Entry alert' : 'Exit alert'}</strong>
      <div>{a.vehicle.vehicle_number} {a.event_type === 'entry' ? 'entered' : 'left'} {a.geofence.geofence_name}</div>
      <div className="muted">{label(a.geofence.category)} - {fmtTime(a.timestamp)}</div>
    </div>
  );
}

export function Toasts({ toasts, onDismiss }) {
  return <div className="toasts">{toasts.map((a) => <Toast key={a.event_id} a={a} onDismiss={onDismiss} />)}</div>;
}
