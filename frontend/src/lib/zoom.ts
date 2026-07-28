// Shared zoom level — persists across component mounts
let _zoom = 1;
const listeners = new Set<() => void>();

export function getZoom(): number {
  return _zoom;
}

export function setZoom(level: number) {
  _zoom = Math.max(0.5, Math.min(2, level));
  listeners.forEach((fn) => fn());
}

export function onZoomChange(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
