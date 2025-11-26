/**
 * BigBlueButton HTML5 Client - Service Worker Registration
 *
 * This module handles service worker registration and provides utilities
 * for interacting with the service worker.
 */

interface ServiceWorkerConfig {
  onSuccess?: (registration: ServiceWorkerRegistration) => void;
  onUpdate?: (registration: ServiceWorkerRegistration) => void;
  onError?: (error: Error) => void;
}

/**
 * Check if service workers are supported in the current browser
 */
export const isServiceWorkerSupported = (): boolean => {
  return 'serviceWorker' in navigator;
};

/**
 * Check if the app is being served over HTTPS or localhost
 * Service workers require secure context (HTTPS) except for localhost
 */
const isSecureContext = (): boolean => {
  const isLocalhost = Boolean(
    window.location.hostname === 'localhost' ||
    window.location.hostname === '[::1]' ||
    window.location.hostname.match(/^127(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}$/)
  );

  return window.isSecureContext || isLocalhost;
};

/**
 * Get the base path for the application dynamically
 * This extracts the directory path from the current URL
 * For example: https://example.com/html5client/ -> /html5client/
 */
const getBasePath = (): string => {
  const pathname = window.location.pathname;

  // If we're at the root or a file, return empty string
  if (pathname === '/' || !pathname.includes('/')) {
    return '';
  }

  // Extract the directory path (everything before the last segment)
  // For /html5client/index.html -> /html5client
  // For /html5client/ -> /html5client
  const pathParts = pathname.split('/').filter(part => part.length > 0);

  // If the last part looks like a file (has an extension), remove it
  const lastPart = pathParts[pathParts.length - 1];
  if (lastPart && lastPart.includes('.')) {
    pathParts.pop();
  }

  // Return the base path with leading and trailing slashes
  return pathParts.length > 0 ? `/${pathParts.join('/')}/` : '/';
};

/**
 * Register the service worker
 */
export const registerServiceWorker = (config?: ServiceWorkerConfig): void => {
  // Check if service workers are supported
  if (!isServiceWorkerSupported()) {
    console.warn('[SW Registration] Service workers are not supported in this browser');
    return;
  }

  // Check if we're in a secure context
  if (!isSecureContext()) {
    console.warn('[SW Registration] Service workers require HTTPS or localhost');
    return;
  }

  // Wait for the window to load before registering
  window.addEventListener('load', () => {
    // Dynamically detect the base path from the current URL
    const basePath = getBasePath();
    const swUrl = `${basePath}service-worker.js`;

    console.log('[SW Registration] Registering at:', swUrl);

    navigator.serviceWorker
      .register(swUrl)
      .then((registration) => {
        console.log('[SW Registration] Service worker registered successfully');

        // Handle updates
        registration.onupdatefound = () => {
          const installingWorker = registration.installing;
          if (!installingWorker) {
            return;
          }

          installingWorker.onstatechange = () => {
            if (installingWorker.state === 'installed') {
              if (navigator.serviceWorker.controller) {
                // New service worker available, but old one is still controlling the page
                console.log('[SW Registration] New content is available; please refresh.');

                if (config?.onUpdate) {
                  config.onUpdate(registration);
                }
              } else {
                // Service worker cached content for the first time
                console.log('[SW Registration] Content is cached for offline use.');

                if (config?.onSuccess) {
                  config.onSuccess(registration);
                }
              }
            }
          };
        };

        // Check for updates periodically (every hour)
        setInterval(() => {
          registration.update();
        }, 1000 * 60 * 60);
      })
      .catch((error) => {
        console.error('[SW Registration] Service worker registration failed:', error);

        if (config?.onError) {
          config.onError(error);
        }
      });
  });
};

/**
 * Unregister the service worker
 */
export const unregisterServiceWorker = (): Promise<boolean> => {
  if (!isServiceWorkerSupported()) {
    return Promise.resolve(false);
  }

  return navigator.serviceWorker.ready
    .then((registration) => {
      return registration.unregister();
    })
    .catch((error) => {
      console.error('[SW Registration] Error unregistering service worker:', error);
      return false;
    });
};

/**
 * Send a message to the active service worker
 */
export const sendMessageToServiceWorker = (message: any): void => {
  if (!isServiceWorkerSupported()) {
    console.warn('[SW Registration] Service workers not supported');
    return;
  }

  if (!navigator.serviceWorker.controller) {
    console.warn('[SW Registration] No active service worker to send message to');
    return;
  }

  navigator.serviceWorker.controller.postMessage(message);
};

/**
 * Request the service worker to precache additional assets
 */
export const precacheAssets = (assets: string[]): void => {
  sendMessageToServiceWorker({
    type: 'PRECACHE_ASSETS',
    assets,
  });
};

/**
 * Request the service worker to clear all caches
 */
export const clearServiceWorkerCache = (): void => {
  sendMessageToServiceWorker({
    type: 'CLEAR_CACHE',
  });
};

/**
 * Request the service worker to skip waiting and activate immediately
 */
export const skipWaiting = (): void => {
  sendMessageToServiceWorker({
    type: 'SKIP_WAITING',
  });
};

/**
 * Listen for messages from the service worker
 */
export const onServiceWorkerMessage = (callback: (event: MessageEvent) => void): void => {
  if (!isServiceWorkerSupported()) {
    return;
  }

  navigator.serviceWorker.addEventListener('message', callback);
};

/**
 * Get the current service worker registration
 */
export const getServiceWorkerRegistration = (): Promise<ServiceWorkerRegistration | undefined> => {
  if (!isServiceWorkerSupported()) {
    return Promise.resolve(undefined);
  }

  return navigator.serviceWorker.getRegistration();
};

/**
 * Check if there's a waiting service worker ready to be activated
 */
export const hasWaitingServiceWorker = async (): Promise<boolean> => {
  const registration = await getServiceWorkerRegistration();
  return Boolean(registration?.waiting);
};

/**
 * Activate a waiting service worker
 */
export const activateWaitingServiceWorker = async (): Promise<void> => {
  const registration = await getServiceWorkerRegistration();

  if (registration?.waiting) {
    registration.waiting.postMessage({ type: 'SKIP_WAITING' });

    // Reload the page once the new service worker takes control
    let refreshing = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (!refreshing) {
        refreshing = true;
        window.location.reload();
      }
    });
  }
};

export default {
  register: registerServiceWorker,
  unregister: unregisterServiceWorker,
  isSupported: isServiceWorkerSupported,
  sendMessage: sendMessageToServiceWorker,
  precacheAssets,
  clearCache: clearServiceWorkerCache,
  skipWaiting,
  onMessage: onServiceWorkerMessage,
  getRegistration: getServiceWorkerRegistration,
  hasWaiting: hasWaitingServiceWorker,
  activateWaiting: activateWaitingServiceWorker,
};
