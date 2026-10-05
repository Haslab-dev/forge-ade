// Open-URL hook shared across feature modules (terminal output links etc.).

import { BrowserOpenURL } from "./wails";

export type BrowserOpenHandler = (url: string) => void;

// A mounted browser surface can register itself here to receive URL opens
// in-app; otherwise URLs fall through to the system browser.
let openBrowserHandler: BrowserOpenHandler | null = null;

export function setOpenBrowserHandler(cb: BrowserOpenHandler | null) {
  openBrowserHandler = cb;
}

export function openInBrowser(url: string) {
  if (openBrowserHandler) {
    openBrowserHandler(url);
  } else {
    BrowserOpenURL(url).catch(() => {});
  }
}
