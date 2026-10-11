// Open-URL hook shared across feature modules (terminal output links etc.).

import { BrowserOpenURL, BrowserUseNavigate } from "./wails";

export type BrowserOpenHandler = (url: string) => void;

let openBrowserHandler: BrowserOpenHandler | null = null;
let lastOpenedUrl: string | null = null;

export function setOpenBrowserHandler(cb: BrowserOpenHandler | null) {
  openBrowserHandler = cb;
}

export function getLastOpenedUrl(): string | null {
  return lastOpenedUrl;
}

export function clearLastOpenedUrl() {
  lastOpenedUrl = null;
}

export function isLocalhostUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    const h = parsed.hostname.toLowerCase();
    return (
      h === "localhost" ||
      h === "127.0.0.1" ||
      h === "0.0.0.0" ||
      h === "[::1]" ||
      h === "::1" ||
      h.endsWith(".localhost") ||
      /^127\.\d+\.\d+\.\d+$/.test(h)
    );
  } catch {
    return (
      /^https?:\/\/(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\])/i.test(url) ||
      /^(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\])/i.test(url)
    );
  }
}

export function normalizeBrowserUrl(raw: string): string {
  let target = raw.trim();
  if (!target) return "";
  if (/^0\.0\.0\.0(:\d+)?/i.test(target)) {
    target = target.replace(/^0\.0\.0\.0/i, "localhost");
  }
  if (/^https?:\/\/0\.0\.0\.0(:\d+)?/i.test(target)) {
    target = target.replace(/^(https?:\/\/)0\.0\.0\.0/i, "$1localhost");
  }
  if (
    !/^https?:\/\//i.test(target) &&
    !target.startsWith("about:") &&
    !target.startsWith("data:")
  ) {
    target = "http://" + target;
  }
  return target;
}

export function openInSidePanelBrowser(rawUrl: string) {
  const url = normalizeBrowserUrl(rawUrl);
  lastOpenedUrl = url;
  if (openBrowserHandler) {
    openBrowserHandler(url);
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(
      new CustomEvent("forge:open-sidepane-browser", { detail: { url } })
    );
  }
  BrowserUseNavigate(url).catch(() => {});
}

export function openInBrowser(url: string) {
  if (isLocalhostUrl(url)) {
    openInSidePanelBrowser(url);
    return;
  }
  if (openBrowserHandler) {
    openBrowserHandler(url);
  } else {
    BrowserOpenURL(url).catch(() => {});
  }
}

