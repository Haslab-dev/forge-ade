/// <reference types="vite/client" />

// Wails runtime types for TypeScript
interface Window {
  runtime: {
    EventsOn(eventName: string, callback: (...args: any[]) => void): () => void;
    EventsOff(eventName: string, ...additionalEventNames: string[]): void;
    EventsEmit(eventName: string, ...data: any[]): void;
    WindowReload(): void;
    WindowReloadApp(): void;
    BrowserOpenURL(url: string): void;
    Quit(): void;
    ClipboardGetText(): Promise<string>;
    ClipboardSetText(text: string): Promise<boolean>;
    OpenDirectoryDialog(options?: any): Promise<{ cancelled: boolean; path: string } | null>;
    OpenFileDialog(options?: any): Promise<{ cancelled: boolean; path: string } | null>;
  };
  go: {
    main: {
      App: Record<string, (...args: any[]) => Promise<any>>;
    };
  };
}
