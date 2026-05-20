import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import App from './App';
import { Providers } from '@app/providers';
import './index.css';

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error('Root element not found');
}

// Register the service worker in production builds only. The Vite dev
// server doesn't ship /sw.js by default, so registering against the
// dev origin would log a console warning and 404 on every reload.
// autoUpdate (configured in vite.config.ts) means new SW activates
// silently on next navigation; no UI prompt required.
if (import.meta.env.PROD) {
  registerSW({ immediate: true });
}

createRoot(rootElement).render(
  <StrictMode>
    <Providers>
      <App />
    </Providers>
  </StrictMode>,
);
