import { useEffect, useState } from 'react';
import { api } from '../api';
import { Empty, Notice } from './ui';

const toLocalInput = (d) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);

export default function LocationPanel({ vehicles, selectedVehicle, onSelect, picked, setPicked, mode, setMode, onUpdated }) {
  const [lat, setLat] = useState('');
  const [lng, setLng] = useState('');
  const [when, setWhen] = useState(toLocalInput(new Date()));
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState(null);
  const [result, setResult] = useState(null);

  // Clicking the map fills the inputs.
  useEffect(() => {
    if (picked) { setLat(String(picked[0])); setLng(String(picked[1])); }
  }, [picked]);

  const onInput = (setter, which) => (e) => {
    setter(e.target.value);
    const a = which === 'lat' ? Number(e.target.value) : Number(lat);
    const b = which === 'lng' ? Number(e.target.value) : Number(lng);
    if (!Number.isNaN(a) && !Number.isNaN(b) && lat !== '' && lng !== '') setPicked([a, b]);
  };

  const submit = async (e) => {
    e.preventDefault();
    setNotice(null);
    setResult(null);
    const la = Number(lat);
    const lo = Number(lng);
    if (!selectedVehicle) return setNotice({ type: 'error', text: 'Select a vehicle first.' });
    if (lat === '' || lng === '' || Number.isNaN(la) || Number.isNaN(lo)) return setNotice({ type: 'error', text: 'Enter a valid latitude and longitude, or click the map.' });
    setSaving(true);
    try {
      const d = await api.updateLocation({
        vehicle_id: selectedVehicle, latitude: la, longitude: lo, timestamp: new Date(when).toISOString(),
      });
      setResult(d);
      setNotice({ type: 'success', text: 'Location updated.' });
      onUpdated(selectedVehicle);
    } catch (err) {
      setNotice({ type: 'error', text: err.message });
    } finally {
      setSaving(false);
    }
  };

  if (vehicles.length === 0) return <Empty>Register a vehicle first.</Empty>;
  const picking = mode === 'pick';

  return (
    <div>
      <h2>Update vehicle location</h2>
      <form onSubmit={submit} className="form">
        <label>Vehicle
          <select value={selectedVehicle} onChange={(e) => onSelect(e.target.value)}>
            <option value="">Select a vehicle...</option>
            {vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number} ({v.driver_name})</option>)}
          </select>
        </label>
        <button type="button" className={picking ? 'btn active' : 'btn'} onClick={() => setMode(picking ? 'none' : 'pick')}>
          {picking ? 'Stop picking' : 'Pick location on map'}
        </button>
        <div className="row">
          <label>Latitude<input value={lat} onChange={onInput(setLat, 'lat')} placeholder="37.7849" /></label>
          <label>Longitude<input value={lng} onChange={onInput(setLng, 'lng')} placeholder="-122.4194" /></label>
        </div>
        <label>Timestamp<input type="datetime-local" value={when} onChange={(e) => setWhen(e.target.value)} /></label>
        <button type="button" className="btn" onClick={() => setWhen(toLocalInput(new Date()))}>Use current time</button>
        <Notice notice={notice} />
        <button className="btn primary" disabled={saving}>{saving ? 'Sending...' : 'Send location update'}</button>
      </form>

      {result && (
        <div className="card result">
          <strong>Currently inside:</strong>
          {result.current_geofences.length === 0
            ? <div className="muted">No geofences</div>
            : <ul>{result.current_geofences.map((g) => <li key={g.geofence_id}>{g.geofence_name}</li>)}</ul>}
        </div>
      )}
    </div>
  );
}
