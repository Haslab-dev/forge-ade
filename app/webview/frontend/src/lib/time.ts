/**
 * Relative time formatting for the forge chrome: "now", "2m", "1h",
 * "9d", then absolute dates for older items.
 */
export function formatRelativeTime(input: string | number | Date | undefined | null): string {
  if (!input) return 'just now';
  let then: number;
  if (input instanceof Date) then = input.getTime();
  else if (typeof input === 'number') then = input;
  else {
    const raw = String(input).trim();
    if (/^\d+[smhdwMy]$/.test(raw)) return raw;
    const parsed = Date.parse(raw);
    if (Number.isNaN(parsed)) {
      const match = raw.match(/(\d{12,13})/);
      if (match) {
        then = parseInt(match[1], 10);
      } else {
        return !/now|just/i.test(raw) ? raw : 'just now';
      }
    } else {
      then = parsed;
    }
  }
  const diff = Date.now() - then;
  if (diff < 0) return 'just now';
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return 'just now';
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h`;
  const day = Math.floor(hr / 24);
  return `${day}d`;
}

/**
 * Resolves the numeric millisecond epoch for any session, recovering
 * timestamps from ISO strings, numbers, or session IDs containing epochs.
 */
export function getSessionEpoch(sess: { id?: string; updatedAt?: string; createdAt?: string }): number {
  const raw = sess.updatedAt || sess.createdAt;
  if (raw && !/now|just/i.test(raw)) {
    const parsed = Date.parse(raw);
    if (!Number.isNaN(parsed)) return parsed;
  }
  if (sess.id) {
    const match = sess.id.match(/(\d{12,13})/);
    if (match) {
      const epoch = parseInt(match[1], 10);
      if (epoch > 1000000000000 && epoch <= Date.now() + 86400000) {
        return epoch;
      }
    }
  }
  return 0;
}

/**
 * Computes the formatted time-ago for a session.
 */
export function formatSessionTime(sess: { id?: string; updatedAt?: string; createdAt?: string; status?: string }): string {
  if (sess.status === 'running') return 'running';
  
  const raw = sess.updatedAt || sess.createdAt;
  if (raw && /^\d+[smhdwMy]$/.test(raw.trim())) {
    return raw.trim();
  }

  const epoch = getSessionEpoch(sess);
  if (epoch > 0) {
    return formatRelativeTime(epoch);
  }

  return 'just now';
}
