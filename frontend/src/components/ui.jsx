export const CATEGORIES = ['delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area'];
export const CATEGORY_COLORS = {
  delivery_zone: '#2563eb',
  restricted_zone: '#dc2626',
  toll_zone: '#d97706',
  customer_area: '#059669',
};
export const label = (s) => (s || '').replace(/_/g, ' ');
export const fmtTime = (iso) => (iso ? new Date(iso).toLocaleString() : '-');

export function Notice({ notice }) {
  if (!notice) return null;
  return <div className={`notice ${notice.type}`}>{notice.text}</div>;
}

export function Empty({ children }) {
  return <p className="muted">{children}</p>;
}

export function Spinner({ text = 'Loading...' }) {
  return <p className="muted">{text}</p>;
}
