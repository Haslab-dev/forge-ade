import { type ClassValue, clsx } from "clsx";
import { PureComponent } from "react";
import { twMerge } from "tailwind-merge";

// App version — keep in sync with frontend/package.json + wails.json
// (make patch-version / minor-version / major-version + make version).
export const APP_VERSION = "0.5.0";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
