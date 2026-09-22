/**
 * Relative time formatting for the forge chrome: "now", "2m", "1h",
 * "9d", then absolute dates for older items.
 */
export function formatRelativeTime(input: string | number | Date | undefined | null): string {
  if (!input) return '';
  let then: number;
  if (input instanceof Date) then = input.getTime();
  else if (typeof input === 'number') then = input;
  else {
    const parsed = Date.parse(input);
    if (Number.isNaN(parsed)) return typeof input === 'string' && !/now|just/i.test(input) ? input : '';
    then = parsed;
  }
  const diff = Date.now() - then;
  if (diff < 0) return 'now';
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return 'now';
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h`;
  const day = Math.floor(hr / 24);
  if (day < 30) return `${day}d`;
  const mo = Math.floor(day / 30);
  if (mo < 12) return `${mo}mo`;
  return `${Math.floor(mo / 12)}y`;
}
