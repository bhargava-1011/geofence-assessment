import { useEffect, useState } from 'react';
import { api } from '../api';
import { CATEGORIES, CATEGORY_COLORS, Empty, Notice, Spinner, label } from './ui';

function parseCoords(text) {
  const t = text.trim();
  if (!t) return [];
  if (t.startsWith('[')) {
    const arr = JSON.parse(t);
    if (!Array.isArray(arr)) throw new Error('Coordinates JSON must be an array of [lat, lng] pairs.');
    return arr;
  }
  return t.split('\n').map((l) => l.trim()).filter(Boolean).map((line) => {
    const [a, b] = line.split(/[\s,]+/).map(Number);
    if (Number.isNaN(a) || Number.isNaN(b)) throw new Error(`Could not read coordinate line: "${line}"`);
    return [a, b];
  });
}

export default function GeofencePanel({ geofences, onCreated, mode, setMode, draft, setDraft }) {
  const [form, setForm] = useState({ name: '', description: '', category: 'delivery_zone', text: '' });
  const [filter, setFilter] = useState('');
  const [list, setList] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api.listGeofences(filter)
      .then((d) => alive && setList(d.geofences))
      .catch((e) => alive && setNotice({ type: 'error', text: e.message }))
      .finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [filter, geofences]);

  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e) => {
    e.preventDefault();
    setNotice(null);
    try {
      let coords = draft.length ? draft.map((p) => [...p]) : parseCoords(form.text);
      if (coords.length < 3) throw new Error('Add at least 3 points: click the map in draw mode, or type coordinates.');
      const f = coords[0];
      const l = coords[coords.length - 1];
      if (f[0] !== l[0] || f[1] !== l[1]) coords = [...coords, [...f]];
      setSaving(true);
      await api.createGeofence({ name: form.name, description: form.description, category: form.category, coordinates: coords });
      setNotice({ type: 'success', text: `Geofence "${form.name}" created.` });
      setForm({ ...form, name: '', description: '', text: '' });
      setDraft([]);
      setMode('none');
      onCreated();
    } catch (err) {
      setNotice({ type: 'error', text: err.message });
    } finally {
      setSaving(false);
    }
  };

  const drawing = mode === 'draw';

  return (
    <div>
      <h2>Create geofence</h2>
      <form onSubmit={submit} className="form">
        <label>Name<input required value={form.name} onChange={set('name')} placeholder="Downtown Delivery Zone" /></label>
        <label>Description<input value={form.description} onChange={set('description')} placeholder="Optional" /></label>
        <label>Category
          <select value={form.category} onChange={set('category')}>
            {CATEGORIES.map((c) => <option key={c} value={c}>{label(c)}</option>)}
          </select>
        </label>

        <div className="row">
          <button type="button" className={drawing ? 'btn active' : 'btn'} onClick={() => setMode(drawing ? 'none' : 'draw')}>
            {drawing ? 'Stop drawing' : 'Draw on map'}
          </button>
          <button type="button" className="btn" disabled={!draft.length} onClick={() => setDraft(draft.slice(0, -1))}>Undo point</button>
          <button type="button" className="btn" disabled={!draft.length} onClick={() => setDraft([])}>Clear</button>
        </div>
        <p className="hint">
          {drawing ? 'Click the map to add corners. The polygon closes automatically.' : 'Or paste coordinates below.'}
          {draft.length > 0 && ` ${draft.length} point(s) drawn (used instead of the text box).`}
        </p>

        <label>Coordinates (one "lat, lng" per line, or a JSON array)
          <textarea rows={4} value={form.text} onChange={set('text')} disabled={draft.length > 0}
            placeholder={'37.7749, -122.4194\n37.7849, -122.4194\n37.7849, -122.4094\n37.7749, -122.4094'} />
        </label>
        <Notice notice={notice} />
        <button className="btn primary" disabled={saving}>{saving ? 'Creating...' : 'Create geofence'}</button>
      </form>

      <div className="section-head">
        <h2>Geofences ({list.length})</h2>
        <select value={filter} onChange={(e) => setFilter(e.target.value)} aria-label="Filter by category">
          <option value="">All categories</option>
          {CATEGORIES.map((c) => <option key={c} value={c}>{label(c)}</option>)}
        </select>
      </div>
      {loading && <Spinner />}
      {!loading && list.length === 0 && <Empty>No geofences yet.</Empty>}
      <ul className="cards">
        {list.map((g) => (
          <li key={g.id} className="card">
            <span className="dot" style={{ background: CATEGORY_COLORS[g.category] }} />
            <div>
              <strong>{g.name}</strong>
              <div className="muted">{label(g.category)} - {g.coordinates.length - 1} vertices</div>
              {g.description && <div className="muted">{g.description}</div>}
              <code>{g.id}</code>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
