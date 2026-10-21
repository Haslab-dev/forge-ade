import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'
import { ToastProvider } from './lib/toast.tsx'
import { initDevBench } from './lib/devBench'
import { initThemeEngine } from './lib/themeEngine'
import { ErrorBoundary } from './components/common/ErrorBoundary'

initDevBench()
initThemeEngine()

// Global handlers to capture uncaught errors and log them clearly
if (typeof window !== 'undefined') {
  window.addEventListener('error', (event) => {
    console.error('[ForgeADE Uncaught Error]', event.error || event.message);
  });
  window.addEventListener('unhandledrejection', (event) => {
    console.error('[ForgeADE Unhandled Rejection]', event.reason);
  });
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <ToastProvider>
        <App />
      </ToastProvider>
    </ErrorBoundary>
  </StrictMode>,
)
