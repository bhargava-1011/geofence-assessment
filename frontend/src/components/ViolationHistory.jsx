import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import { Empty, Notice, Spinner, fmtTime } from './ui';

const PAGE_SIZE = 15;

export default function ViolationHistory({ geofences, vehicles, tick }) {
  const [filters, setFilters] = useState({ vehicle_id: '', geofence_id: '', start: '', end: '' });
  const [page, setPage] = useState(0);
  const [data, setData] = useState({ violations: [], total_count: 0 });
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setNotice(null);
    try {
      const d = await api.history({
        vehicle_id: filters.vehicle_id,
        geofence_id: filters.geofence_id,
        start_date: filters.start ? new Date(filters.start).toISOString() : '',
        end_date: filters.end ? new Date(filters.end).toISOString() : '',
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      });
      setData(d);
    } catch (e) {
      setNotice({ type: 'error', text: e.message });
    } finally {
      setLoading(false);
    }
  }, [filters, page]);

  useEffect(() => { load(); }, [load, tick]);

  const change = (k) => (e) => { setPage(0); setFilters({ ...filters, [k]: e.target.value }); };
  const pages = Math.max(1, Math.ceil(data.total_count / PAGE_SIZE));

  return (
    <div>
      <h2>Violation history</h2>
      <div className="form">
        <div className="row">
          <label>Vehicle
            <select value={filters.vehicle_id} onChange={change('vehicle_id')}>
              <option value="">All</option>
              {vehicles.map((v) => <option key={v.id} value={v.id}>{v.vehicle_number}</option>)}
            </select>
          </label>
          <label>Geofence
            <select value={filters.geofence_id} onChange={change('geofence_id')}>
              <option value="">All</option>
              {geofences.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
            </select>
          </label>
        </div>
        <div className="row">
          <label>From<input type="datetime-local" value={filters.start} onChange={change('start')} /></label>
          <label>To<input type="datetime-local" value={filters.end} onChange={change('end')} /></label>
        </div>
      </div>
      <Notice notice={notice} />
      {loading && <Spinner />}
      {!loading && data.violations.length === 0 && <Empty>No events match these filters.</Empty>}
      <ul className="cards">
        {data.violations.map((v) => (
          <li key={v.id} className="card">
            <span className={`tag ${v.event_type}`}>{v.event_type}</span>
            <div>
              <strong>{v.vehicle_number}</strong> - {v.geofence_name}
              <div className="muted">{fmtTime(v.timestamp)}</div>
              <div className="muted">{v.latitude.toFixed(5)}, {v.longitude.toFixed(5)}</div>
            </div>
          </li>
        ))}
      </ul>
      <div className="pager">
        <button className="btn" disabled={page === 0} onClick={() => setPage(page - 1)}>Prev</button>
        <span>Page {page + 1} of {pages} ({data.total_count} events)</span>
        <button className="btn" disabled={page + 1 >= pages} onClick={() => setPage(page + 1)}>Next</button>
      </div>
    </div>
  );
}
