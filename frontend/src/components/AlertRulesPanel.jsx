import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import { Empty, Notice, Spinner, fmtTime } from './ui';

export default function AlertRulesPanel({ geofences, vehicles }) {
  const [form, setForm] = useState({ geofence_id: '', vehicle_id: '', event_type: 'entry' });
  const [rules, setRules] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState(null);
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const load = useCallback(async () => {
    setLoading(true);
    try { setRules((await api.listAlerts()).alerts); }
    catch (e) { setNotice({ type: 'error', text: e.message }); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { load(); }, [load]);

  const submit = async (e) => {
    e.preventDefault();
    setNotice(null);
    if (!form.geofence_id) return setNotice({ type: 'error', text: 'Select a geofence.' });
    setSaving(true);
    try {
      await api.createAlert({
        geofence_id: form.geofence_id,
        event_type: form.event_type,
        ...(form.vehicle_id ? { vehicle_id: form.vehicle_id } : {}),
      });
      setNotice({ type: 'success', text: 'Alert rule created.' });
      load();
    } catch (err) {
      setNotice({ type: 'error', text: err.message });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <h2>Configure alert rule</h2>
      <form onSubmit={submit} className="form">
        <label>Geofence
          <select value={form.geofence_id} onChange={set('geofence_id')}>
            <option value="">Select a geofence...</option>
            {geofences.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
          </select>
        </label>
        <label>Vehicle (optional)
          <select value={form.vehicle_id} onChange={set('vehicle_id')}>
            <option value="">All vehicles</option>
            {vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number}</option>)}
          </select>
        </label>
        <label>Event type
          <select value={form.event_type} onChange={set('event_type')}>
            <option value="entry">Entry</option>
            <option value="exit">Exit</option>
            <option value="both">Both</option>
          </select>
        </label>
        <Notice notice={notice} />
        <button className="btn primary" disabled={saving}>{saving ? 'Saving...' : 'Create alert rule'}</button>
      </form>

      <div className="section-head"><h2>Alert rules ({rules.length})</h2><button className="btn" onClick={load}>Refresh</button></div>
      {loading && <Spinner />}
      {!loading && rules.length === 0 && <Empty>No alert rules configured.</Empty>}
      <ul className="cards">
        {rules.map((r) => (
          <li key={r.alert_id} className="card">
            <div>
              <strong>{r.geofence_name}</strong> <span className="chip">{r.event_type}</span>
              <div className="muted">{r.vehicle_number ? `Vehicle ${r.vehicle_number}` : 'All vehicles'} - {r.status}</div>
              <div className="muted">Created {fmtTime(r.created_at)}</div>
              <code>{r.alert_id}</code>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
