/**
 * Dev-only benchmark harness (window.__forgeBench).
 *
 * Measures what the WKWebView cannot report on its own: JS heap is not
 * exposed outside Safari's inspector, so we track (a) DOM node counts,
 * (b) wall-clock timings of hot operations, and (c) the Go process RSS via
 * GetProcessRSS — the doc's "measure per process" rule: WebView heap and Go
 * RSS move independently.
 *
 * Enable: run the app in dev (bun run dev / wails dev), open the webview
 * console (right-click → Inspect), then:
 *
 *   __forgeBench.report()                  // DOM nodes + Go RSS snapshot
 *   await __forgeBench.openCloseLoop('/path/to/file.ts', 20)
 *   await __forgeBench.refreshLoop(10)     // file-tree reload under load
 *   __forgeBench.trace(true)               // long-task observer on/off
 *
 * Full methodology: docs/PERFORMANCE.md.
 */

import { ApiBridge } from '../services/apiBridge';

interface BenchWindow {
  __forgeBench?: ForgeBench;
}

interface ForgeBench {
  domNodes: () => number;
  rss: () => Promise<number>;
  time: <T>(label: string, fn: () => T | Promise<T>) => Promise<T>;
  report: () => Promise<void>;
  openCloseLoop: (path: string, rounds?: number) => Promise<void>;
  refreshLoop: (rounds?: number) => Promise<void>;
  trace: (on: boolean) => void;
  /** Registered by workspaceStore in dev builds — the lifecycle hooks. */
  __store?: {
    openFile: (path: string) => Promise<void>;
    closeActiveTab: () => void;
    activeTabPath: () => string | null;
    tabCount: () => number;
    refreshFiles: () => Promise<void>;
  };
}

const samples: Array<{ t: number; label: string; ms?: number; dom?: number; rssKb?: number }> = [];
// Long benchmark sessions must not accumulate entries forever.
const MAX_SAMPLES = 200;
let longTaskObserver: PerformanceObserver | null = null;
let t0 = performance.now();

function stamp(label: string, extra: { ms?: number; rssKb?: number } = {}) {
  const entry = {
    t: Math.round(performance.now() - t0),
    label,
    dom: document.getElementsByTagName('*').length,
    ...extra
  };
  samples.push(entry);
  if (samples.length > MAX_SAMPLES) samples.splice(0, samples.length - MAX_SAMPLES);
  const rss = entry.rssKb !== undefined ? ` rss=${(entry.rssKb / 1024).toFixed(1)}MB` : '';
  const ms = entry.ms !== undefined ? ` in ${entry.ms.toFixed(1)}ms` : '';
  console.log(`[bench] +${entry.t}ms ${label}${ms} dom=${entry.dom}${rss}`);
}

function installHarness(): ForgeBench {
  const bench: ForgeBench = {
    domNodes: () => document.getElementsByTagName('*').length,
    rss: () => ApiBridge.processRSS(),
    time: async (label, fn) => {
      const start = performance.now();
      const result = await fn();
      stamp(label, { ms: performance.now() - start });
      return result;
    },
    report: async () => {
      const rssKb = await bench.rss();
      stamp('report', { rssKb });
      console.table(samples.slice(-15));
    },
    openCloseLoop: async (path, rounds = 20) => {
      const store = bench.__store;
      if (!store) {
        console.warn('[bench] store hooks not registered yet — is the workspace open?');
        return;
      }
      const before = await bench.rss();
      const start = performance.now();
      for (let i = 0; i < rounds; i++) {
        await store.openFile(path);
        store.closeActiveTab();
        // Let React commit between rounds so the measurement includes renders.
        await new Promise(r => requestAnimationFrame(() => r(null)));
      }
      const ms = (performance.now() - start) / rounds;
      // Two idle GC-ish frames later, memory should return near baseline —
      // a persistent climb means a retained buffer (leak).
      await new Promise(r => setTimeout(r, 600));
      const after = await bench.rss();
      stamp(`openCloseLoop×${rounds} avg`, { ms, rssKb: after });
      console.log(`[bench] RSS before=${(before / 1024).toFixed(1)}MB after=${(after / 1024).toFixed(1)}MB delta=${((after - before) / 1024).toFixed(2)}MB — should hover near 0`);
    },
    refreshLoop: async (rounds = 10) => {
      const store = bench.__store;
      if (!store) {
        console.warn('[bench] store hooks not registered yet');
        return;
      }
      const start = performance.now();
      for (let i = 0; i < rounds; i++) {
        await bench.time(`refreshFiles#${i}`, () => store.refreshFiles());
        await new Promise(r => requestAnimationFrame(() => r(null)));
      }
      console.log(`[bench] refreshLoop avg ${((performance.now() - start) / rounds).toFixed(1)}ms`);
    },
    trace: (on: boolean) => {
      if (on && !longTaskObserver) {
        longTaskObserver = new PerformanceObserver(list => {
          for (const e of list.getEntries()) {
            console.warn(`[bench] long task ${Math.round(e.duration)}ms`);
          }
        });
        longTaskObserver.observe({ entryTypes: ['longtask'] });
        console.log('[bench] long-task tracing on');
      } else if (!on && longTaskObserver) {
        longTaskObserver.disconnect();
        longTaskObserver = null;
        console.log('[bench] long-task tracing off');
      }
    }
  };
  return bench;
}

/** Installs the harness once; no-op in production builds. */
export function initDevBench(): void {
  if (!import.meta.env.DEV) return;
  const w = window as unknown as BenchWindow;
  if (!w.__forgeBench) {
    w.__forgeBench = installHarness();
    console.info('[bench] harness ready — try __forgeBench.report()');
  }
}

/** workspaceStore calls this in dev to expose lifecycle hooks. */
export function registerBenchStore(api: NonNullable<ForgeBench['__store']>): void {
  if (!import.meta.env.DEV) return;
  const w = window as unknown as BenchWindow;
  if (w.__forgeBench) w.__forgeBench.__store = api;
}
