import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { timeAgo } from './gitDiffUtils';

describe('timeAgo', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-22T16:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns empty string for falsy/invalid inputs', () => {
    expect(timeAgo()).toBe('');
    expect(timeAgo('')).toBe('');
    expect(timeAgo('invalid-date')).toBe('');
    expect(timeAgo(0)).toBe('');
  });

  it('returns "just now" for dates within 1 minute or in the future', () => {
    expect(timeAgo(new Date('2026-10-22T16:00:10Z').toISOString())).toBe('just now');
    expect(timeAgo(new Date('2026-10-22T15:59:30Z').toISOString())).toBe('just now');
  });

  it('returns minutes ago', () => {
    expect(timeAgo(new Date('2026-10-22T15:45:00Z').toISOString())).toBe('15m ago');
    expect(timeAgo(new Date('2026-10-22T15:01:00Z').toISOString())).toBe('59m ago');
  });

  it('returns hours ago', () => {
    expect(timeAgo(new Date('2026-10-22T14:00:00Z').toISOString())).toBe('2h ago');
    expect(timeAgo(new Date('2026-10-21T17:00:00Z').toISOString())).toBe('23h ago');
  });

  it('returns days ago (up to 30 days)', () => {
    // 6 days ago (the user case: 16/10/2026 commit from screenshot)
    expect(timeAgo('2026-10-16T01:31:26Z')).toBe('6d ago');
    expect(timeAgo(new Date('2026-10-02T16:00:00Z').toISOString())).toBe('20d ago');
  });

  it('returns months ago', () => {
    expect(timeAgo(new Date('2026-08-22T16:00:00Z').toISOString())).toBe('2mo ago');
  });

  it('returns years ago', () => {
    expect(timeAgo(new Date('2024-10-22T16:00:00Z').toISOString())).toBe('2y ago');
  });

  it('supports unix timestamp in seconds', () => {
    const tsSec = Math.floor(new Date('2026-10-16T01:31:26Z').getTime() / 1000);
    expect(timeAgo(tsSec)).toBe('6d ago');
  });

  it('supports unix timestamp in milliseconds', () => {
    const tsMs = new Date('2026-10-16T01:31:26Z').getTime();
    expect(timeAgo(tsMs)).toBe('6d ago');
  });
});
