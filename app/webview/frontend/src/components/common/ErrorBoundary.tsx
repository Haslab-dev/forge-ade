import React, { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle, RefreshCw, Trash2, Copy, Check } from 'lucide-react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
  copied: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  public override state: State = {
    hasError: false,
    error: null,
    errorInfo: null,
    copied: false,
  };

  public static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error };
  }

  public override componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('[ForgeADE ErrorBoundary caught error]', error, errorInfo);
    this.setState({ errorInfo });
  }

  private handleReload = (): void => {
    window.location.reload();
  };

  private handleResetStorage = (): void => {
    try {
      localStorage.clear();
      sessionStorage.clear();
    } catch (err) {
      console.warn('Failed to clear storage:', err);
    }
    window.location.reload();
  };

  private handleCopy = async (): Promise<void> => {
    const { error, errorInfo } = this.state;
    const text = [
      `ForgeADE Render Error: ${error?.name || 'Error'}`,
      `Message: ${error?.message || 'Unknown error'}`,
      error?.stack ? `\nStack:\n${error.stack}` : '',
      errorInfo?.componentStack ? `\nComponent Stack:\n${errorInfo.componentStack}` : '',
    ]
      .filter(Boolean)
      .join('\n');

    try {
      await navigator.clipboard.writeText(text);
      this.setState({ copied: true });
      setTimeout(() => this.setState({ copied: false }), 2000);
    } catch {
      // fallback
    }
  };

  public override render(): ReactNode {
    if (this.state.hasError) {
      const { error, errorInfo, copied } = this.state;
      return (
        <div className="flex h-screen w-screen items-center justify-center bg-app p-6 font-sans select-none">
          <div className="w-full max-w-xl rounded-xl border border-border bg-panel p-6 shadow-2xl">
            <div className="flex items-center gap-3 border-b border-border pb-4">
              <div className="flex size-10 items-center justify-center rounded-lg bg-status-error/10 text-status-error">
                <AlertTriangle className="size-5" />
              </div>
              <div>
                <h1 className="text-ui-lg font-semibold text-fg-primary">
                  Application Encountered an Error
                </h1>
                <p className="text-ui-xs text-fg-tertiary">
                  An unexpected exception occurred during rendering.
                </p>
              </div>
            </div>

            <div className="my-4 max-h-60 overflow-y-auto rounded-lg border border-border bg-surface p-3 font-mono text-ui-xs text-fg-secondary">
              <p className="font-semibold text-status-error">
                {error?.name || 'Error'}: {error?.message || 'Unknown error'}
              </p>
              {error?.stack && (
                <pre className="mt-2 text-ui-xs text-fg-tertiary whitespace-pre-wrap break-all">
                  {error.stack}
                </pre>
              )}
              {errorInfo?.componentStack && (
                <details className="mt-2 text-fg-tertiary cursor-pointer">
                  <summary className="text-ui-xs font-sans hover:text-fg-secondary">
                    Component Stack
                  </summary>
                  <pre className="mt-1 text-ui-xs whitespace-pre-wrap break-all">
                    {errorInfo.componentStack}
                  </pre>
                </details>
              )}
            </div>

            <div className="flex items-center justify-between gap-3 pt-2">
              <button
                type="button"
                onClick={this.handleCopy}
                className="flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-ui-sm font-medium text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors cursor-pointer"
              >
                {copied ? <Check className="size-3.5 text-status-success" /> : <Copy className="size-3.5" />}
                {copied ? 'Copied' : 'Copy Details'}
              </button>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={this.handleResetStorage}
                  className="flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-ui-sm font-medium text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors cursor-pointer"
                  title="Clear localStorage/sessionStorage and reload"
                >
                  <Trash2 className="size-3.5" />
                  Reset State & Reload
                </button>
                <button
                  type="button"
                  onClick={this.handleReload}
                  className="flex items-center gap-1.5 rounded-md bg-foreground px-4 py-1.5 text-ui-sm font-medium text-background hover:opacity-90 transition-opacity cursor-pointer"
                >
                  <RefreshCw className="size-3.5" />
                  Reload
                </button>
              </div>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
