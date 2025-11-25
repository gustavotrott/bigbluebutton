declare global {
  interface Window {
    __BUNDLE_URL__?: string;
  }
}

const isSupported = (): boolean => 'serviceWorker' in navigator;

const shouldUseServiceWorker = (): boolean => process.env.NODE_ENV === 'production';

const log = (...args: unknown[]): void => {
  if (process.env.DETAILED_LOGS) {
    // eslint-disable-next-line no-console
    console.log('[pwa]', ...args);
  }
};

export const registerServiceWorker = (): void => {
  if (!isSupported() || !shouldUseServiceWorker()) return;

  window.addEventListener('load', () => {
    const serviceWorkerUrl = new URL('service-worker.js', window.location.href).toString();

    navigator.serviceWorker
      .register(serviceWorkerUrl)
      .then((registration) => {
        log('Service worker registered with scope:', registration.scope);
        const sendPrecacheList = () => {
          if (!navigator.serviceWorker.controller) return;
          const assets = [
            new URL('index.html', window.location.href).toString(),
          ];

          if (window.__BUNDLE_URL__) {
            assets.push(new URL(window.__BUNDLE_URL__, window.location.href).toString());
          }

          navigator.serviceWorker.controller.postMessage({
            type: 'PRECACHE_ASSETS',
            assets,
          });
        };

        if (navigator.serviceWorker.controller) {
          sendPrecacheList();
        } else {
          navigator.serviceWorker.addEventListener('controllerchange', sendPrecacheList, { once: true });
        }
      })
      .catch((error) => {
        // eslint-disable-next-line no-console
        console.error('Service worker registration failed:', error);
      });
  });
};

export const unregisterServiceWorker = (): void => {
  if (!isSupported()) return;

  navigator.serviceWorker.ready
    .then((registration) => registration.unregister())
    .catch(() => undefined);
};
