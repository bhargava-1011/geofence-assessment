export const API_URL = (import.meta.env.VITE_API_URL || 'http://localhost:8080').replace(/\/$/, '');
export const WS_URL = API_URL.replace(/^http/, 'ws') + '/ws/alerts';

async function request(path, options = {}) {
  let res;
  try {
    res = await fetch(API_URL + path, { headers: { 'Content-Type': 'application/json' }, ...options });
  } catch {
    throw new Error('Cannot reach the server. Is the backend running?');
  }
  let data = null;
  try { data = await res.json(); } catch { /* empty body */ }
  if (!res.ok) throw new Error(data?.error || `Request failed (${res.status})`);
  return data;
}

const qs = (params = {}) => {
  const p = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => { if (v !== '' && v != null) p.set(k, v); });
  const s = p.toString();
  return s ? `?${s}` : '';
};
const post = (path, body) => request(path, { method: 'POST', body: JSON.stringify(body) });

export const api = {
  listGeofences: (category) => request('/geofences' + qs({ category })),
  createGeofence: (body) => post('/geofences', body),
  listVehicles: () => request('/vehicles'),
  createVehicle: (body) => post('/vehicles', body),
  updateLocation: (body) => post('/vehicles/location', body),
  getLocation: (id) => request(`/vehicles/location/${encodeURIComponent(id)}`),
  listAlerts: () => request('/alerts'),
  createAlert: (body) => post('/alerts/configure', body),
  history: (params) => request('/violations/history' + qs(params)),
};
