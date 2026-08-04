import { type ClassValue, clsx } from "clsx";
import { PureComponent } from "react";
import { twMerge } from "tailwind-merge";

// App version — injected at build time from frontend/package.json (vite.config.ts).
// Bump via `make patch-version` / `minor-version` / `major-version`.
export const APP_VERSION = __APP_VERSION__;

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
