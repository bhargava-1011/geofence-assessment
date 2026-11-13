import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import { useAlertSocket } from './useAlertSocket';
import MapView from './components/MapView';
import GeofencePanel from './components/GeofencePanel';
import VehiclePanel from './components/VehiclePanel';
import LocationPanel from './components/LocationPanel';
import AlertRulesPanel from './components/AlertRulesPanel';
import ViolationHistory from './components/ViolationHistory';
import { LiveFeed, Toasts } from './components/LiveFeed';
import { Notice } from './components/ui';

const TABS = [
  ['geofences', 'Geofences'],
  ['vehicles', 'Vehicles'],
  ['location', 'Location'],
  ['alerts', 'Alert rules'],
  ['history', 'History'],
  ['live', 'Live feed'],
];

export default function App() {
  const [tab, setTab] = useState('geofences');
  const [geofences, setGeofences] = useState([]);
  const [vehicles, setVehicles] = useState([]);
  const [locations, setLocations] = useState({});
  const [loadingVehicles, setLoadingVehicles] = useState(false);
  const [error, setError] = useState(null);

  const [mode, setMode] = useState('none'); // 'none' | 'draw' | 'pick'
  const [draft, setDraft] = useState([]);
  const [picked, setPicked] = useState(null);
  const [selectedVehicle, setSelectedVehicle] = useState('');

  const [alerts, setAlerts] = useState([]);
  const [toasts, setToasts] = useState([]);
  const [historyTick, setHistoryTick] = useState(0);

  const loadGeofences = useCallback(async () => {
    try { setGeofences((await api.listGeofences()).geofences); setError(null); }
    catch (e) { setError({ type: 'error', text: e.message }); }
  }, []);

  const refreshLocation = useCallback(async (id) => {
    try {
      const d = await api.getLocation(id);
      setLocations((prev) => ({ ...prev, [id]: d }));
    } catch { /* vehicle may have been removed */ }
  }, []);

  const loadVehicles = useCallback(async () => {
    setLoadingVehicles(true);
    try {
      const { vehicles: list } = await api.listVehicles();
      setVehicles(list);
      const entries = await Promise.all(list.map((v) => api.getLocation(v.id).then((l) => [v.id, l]).catch(() => null)));
      setLocations(Object.fromEntries(entries.filter(Boolean)));
      setError(null);
    } catch (e) {
      setError({ type: 'error', text: e.message });
    } finally {
      setLoadingVehicles(false);
    }
  }, []);

  useEffect(() => { loadGeofences(); loadVehicles(); }, [loadGeofences, loadVehicles]);

  const wsStatus = useAlertSocket((evt) => {
    setAlerts((prev) => [evt, ...prev].slice(0, 50));
    setToasts((prev) => [...prev, evt]);
    refreshLocation(evt.vehicle.vehicle_id);
    setHistoryTick((t) => t + 1);
  });

  const dismissToast = useCallback((id) => setToasts((prev) => prev.filter((t) => t.event_id !== id)), []);

  const onMapPoint = (p) => {
    if (mode === 'draw') setDraft((d) => [...d, p]);
    if (mode === 'pick') setPicked(p);
  };

  const switchTab = (t) => {
    setTab(t);
    if (t !== 'geofences') setMode((m) => (m === 'draw' ? 'none' : m));
    if (t !== 'location') setMode((m) => (m === 'pick' ? 'none' : m));
  };

  return (
    <div className="app">
      <header>
        <h1>Geofence Tracker</h1>
        <span className={`ws ws-${wsStatus}`}>
          <span className="ws-dot" /> Live alerts: {wsStatus}
        </span>
      </header>

      <div className="layout">
        <aside>
          <nav className="tabs">
            {TABS.map(([key, text]) => (
              <button key={key} className={tab === key ? 'tab active' : 'tab'} onClick={() => switchTab(key)}>
                {text}{key === 'live' && alerts.length > 0 ? ` (${alerts.length})` : ''}
              </button>
            ))}
          </nav>
          <div className="panel">
            <Notice notice={error} />
            {tab === 'geofences' && (
              <GeofencePanel geofences={geofences} onCreated={loadGeofences} mode={mode} setMode={setMode} draft={draft} setDraft={setDraft} />
            )}
            {tab === 'vehicles' && (
              <VehiclePanel vehicles={vehicles} locations={locations} loading={loadingVehicles}
                onCreated={loadVehicles} onRefresh={loadVehicles} selectedVehicle={selectedVehicle} onSelect={setSelectedVehicle} />
            )}
            {tab === 'location' && (
              <LocationPanel vehicles={vehicles} selectedVehicle={selectedVehicle} onSelect={setSelectedVehicle}
                picked={picked} setPicked={setPicked} mode={mode} setMode={setMode} onUpdated={(id) => { refreshLocation(id); setHistoryTick((t) => t + 1); }} />
            )}
            {tab === 'alerts' && <AlertRulesPanel geofences={geofences} vehicles={vehicles} />}
            {tab === 'history' && <ViolationHistory geofences={geofences} vehicles={vehicles} tick={historyTick} />}
            {tab === 'live' && <LiveFeed alerts={alerts} />}
          </div>
        </aside>

        <main>
          <MapView geofences={geofences} vehicles={vehicles} locations={locations} selectedVehicle={selectedVehicle}
            mode={mode} draft={draft} picked={picked} onPoint={onMapPoint} />
          {mode !== 'none' && (
            <div className="map-banner">{mode === 'draw' ? 'Drawing mode: click to add polygon corners' : 'Click the map to set the vehicle location'}</div>
          )}
        </main>
      </div>

      <Toasts toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}
