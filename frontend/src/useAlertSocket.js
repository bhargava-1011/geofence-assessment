import { useEffect, useRef, useState } from 'react';
import { WS_URL } from './api';

// Connects to /ws/alerts and reconnects with exponential backoff.
export function useAlertSocket(onAlert) {
  const [status, setStatus] = useState('connecting');
  const handler = useRef(onAlert);
  handler.current = onAlert;

  useEffect(() => {
    let ws;
    let timer;
    let attempt = 0;
    let closed = false;

    const connect = () => {
      ws = new WebSocket(WS_URL);
      ws.onopen = () => { attempt = 0; setStatus('connected'); };
      ws.onmessage = (e) => {
        try { handler.current(JSON.parse(e.data)); } catch { /* ignore malformed */ }
      };
      ws.onclose = () => {
        if (closed) return;
        setStatus('reconnecting');
        attempt += 1;
        timer = setTimeout(connect, Math.min(1000 * 2 ** attempt, 15000));
      };
      ws.onerror = () => ws.close();
    };
    connect();

    return () => { closed = true; clearTimeout(timer); if (ws) ws.close(); };
  }, []);

  return status;
}
