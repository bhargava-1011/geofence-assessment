import { useState } from 'react';
import { api } from '../api';
import { Empty, Notice, Spinner, fmtTime } from './ui';

export default function VehiclePanel({ vehicles, locations, loading, onCreated, onRefresh, selectedVehicle, onSelect }) {
  const [form, setForm] = useState({ vehicle_number: '', driver_name: '', vehicle_type: 'truck', phone: '' });
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState(null);
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e) => {
    e.preventDefault();
    setNotice(null);
    setSaving(true);
    try {
      await api.createVehicle(form);
      setNotice({ type: 'success', text: `Vehicle ${form.vehicle_number} registered.` });
      setForm({ ...form, vehicle_number: '', driver_name: '', phone: '' });
      onCreated();
    } catch (err) {
      setNotice({ type: 'error', text: err.message });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <h2>Register vehicle</h2>
      <form onSubmit={submit} className="form">
        <label>Vehicle number<input required value={form.vehicle_number} onChange={set('vehicle_number')} placeholder="KA-01-AB-1234" /></label>
        <label>Driver name<input required value={form.driver_name} onChange={set('driver_name')} placeholder="John Doe" /></label>
        <label>Vehicle type
          <select value={form.vehicle_type} onChange={set('vehicle_type')}>
            {['truck', 'car', 'van', 'motorcycle'].map((t) => <option key={t}>{t}</option>)}
          </select>
        </label>
        <label>Phone<input required value={form.phone} onChange={set('phone')} placeholder="+1234567890" /></label>
        <Notice notice={notice} />
        <button className="btn primary" disabled={saving}>{saving ? 'Registering...' : 'Register vehicle'}</button>
      </form>

      <div className="section-head">
        <h2>Vehicles ({vehicles.length})</h2>
        <button className="btn" onClick={onRefresh}>Refresh</button>
      </div>
      {loading && <Spinner />}
      {!loading && vehicles.length === 0 && <Empty>No vehicles registered yet.</Empty>}
      <ul className="cards">
        {vehicles.map((v) => {
          const info = locations[v.id];
          const loc = info?.current_location;
          return (
            <li key={v.id} className={`card clickable ${v.id === selectedVehicle ? 'selected' : ''}`} onClick={() => onSelect(v.id)}>
              <div>
                <strong>{v.vehicle_number}</strong> <span className="muted">{v.vehicle_type} - {v.driver_name}</span>
                <div className="muted">
                  {loc ? `${loc.latitude.toFixed(5)}, ${loc.longitude.toFixed(5)} at ${fmtTime(loc.timestamp)}` : 'No location reported yet'}
                </div>
                {loc && (
                  <div className="chips">
                    {info.current_geofences.length === 0 && <span className="chip">outside all geofences</span>}
                    {info.current_geofences.map((g) => (
                      <span key={g.geofence_id} className={`chip ${g.category === 'restricted_zone' ? 'danger' : ''}`}>{g.geofence_name}</span>
                    ))}
                  </div>
                )}
                <code>{v.id}</code>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
