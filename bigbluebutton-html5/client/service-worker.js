/**
 * BigBlueButton HTML5 Client - Service Worker
 *
 * This service worker handles asset caching separately from application fetch requests.
 * It provides offline support and faster load times by caching static assets.
 */

const CACHE_VERSION = 'v1';
const ASSET_CACHE_NAME = `bbb-assets-${CACHE_VERSION}`;
const RUNTIME_CACHE_NAME = `bbb-runtime-${CACHE_VERSION}`;

// Assets to precache on service worker installation
const PRECACHE_URLS = [
  './',
  './index.html',
  './stylesheets/normalize.css',
  './stylesheets/bbb-icons.css',
  './stylesheets/fonts.css',
  './stylesheets/fontSizing.css',
  './stylesheets/modals.css',
  './stylesheets/toastify.css',
  './stylesheets/toggleSwitch.css',
];

// Asset file extensions that should be cached
const CACHEABLE_EXTENSIONS = [
  '.js',
  '.css',
  '.woff',
  '.woff2',
  '.ttf',
  '.otf',
  '.eot',
  '.svg',
  '.png',
  '.jpg',
  '.jpeg',
  '.gif',
  '.webp',
  '.ico',
];

/**
 * Install event - Precache static assets
 */
self.addEventListener('install', (event) => {
  console.log('[Service Worker] Installing and precaching assets');

  event.waitUntil(
    caches.open(ASSET_CACHE_NAME)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting())
      .catch((error) => {
        console.error('[Service Worker] Precaching failed:', error);
      })
  );
});

/**
 * Activate event - Clean up old caches and claim clients
 */
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((cacheNames) => {
        return Promise.all(
          cacheNames
            .filter((cacheName) => {
              // Delete caches that don't match current version
              return cacheName.startsWith('bbb-') &&
                     cacheName !== ASSET_CACHE_NAME &&
                     cacheName !== RUNTIME_CACHE_NAME;
            })
            .map((cacheName) => {
              console.log('[Service Worker] Deleting old cache:', cacheName);
              return caches.delete(cacheName);
            })
        );
      })
      .then(() => self.clients.claim())
  );
});

/**
 * Get the service worker scope path (cached)
 */
let SCOPE_PATH = null;
const getScopePath = () => {
  if (SCOPE_PATH === null) {
    const url = new URL(self.location.href);
    SCOPE_PATH = url.pathname.replace('service-worker.js', '');
  }
  return SCOPE_PATH;
};

/**
 * Check if request is within service worker scope
 */
const inScope = (url) => {
  const scopePath = getScopePath();
  return url.origin === self.location.origin &&
         url.pathname.startsWith(scopePath);
};

/**
 * Check if the request is for a cacheable asset
 */
const isCacheableAsset = (url) => {
  return CACHEABLE_EXTENSIONS.some(ext => url.pathname.endsWith(ext));
};

/**
 * Check if the request is for an API call (GraphQL, REST, WebSocket)
 */
const isApiRequest = (url) => {
  return url.pathname.includes('/graphql') ||
         url.pathname.includes('/api/') ||
         url.pathname.includes('/bigbluebutton/api');
};

/**
 * Message handler - Allow client to precache additional assets dynamically
 */
self.addEventListener('message', (event) => {
  const { data } = event;

  if (!data) return;

  // Handle precache asset requests from client
  if (data.type === 'PRECACHE_ASSETS' && Array.isArray(data.assets)) {
    console.log('[Service Worker] Precaching additional assets:', data.assets);
    event.waitUntil(
      caches.open(ASSET_CACHE_NAME)
        .then((cache) => cache.addAll(data.assets))
        .catch((error) => {
          console.error('[Service Worker] Dynamic precaching failed:', error);
        })
    );
  }

  // Handle cache clearing requests
  if (data.type === 'CLEAR_CACHE') {
    console.log('[Service Worker] Clearing all caches');
    event.waitUntil(
      caches.keys().then((cacheNames) => {
        return Promise.all(
          cacheNames.map((cacheName) => caches.delete(cacheName))
        );
      })
    );
  }

  // Handle skip waiting requests
  if (data.type === 'SKIP_WAITING') {
    console.log('[Service Worker] Skipping waiting period');
    self.skipWaiting();
  }
});

/**
 * Fetch event - Intercept network requests and serve from cache when appropriate
 *
 * Strategy:
 * - Navigation requests: Network-first with cache fallback (stale-while-revalidate)
 * - Static assets: Cache-first with network fallback
 * - API requests: Network-only (let application handle with fetch())
 */
self.addEventListener('fetch', (event) => {
  const { request } = event;

  // Only handle GET requests
  if (request.method !== 'GET') {
    return;
  }

  const requestUrl = new URL(request.url);

  // Only handle requests within our scope
  if (!inScope(requestUrl)) {
    return;
  }

  // Strategy 1: API requests - Network only, no caching
  // This allows the application's fetch() initializer to handle these requests
  if (isApiRequest(requestUrl)) {
    // Don't intercept - let it pass through to the application's fetch handler
    return;
  }

  // Strategy 2: Navigation requests - Stale-while-revalidate
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          // Update cache with fresh response
          const responseClone = response.clone();
          caches.open(RUNTIME_CACHE_NAME)
            .then((cache) => cache.put(request, responseClone));
          return response;
        })
        .catch(async () => {
          // Network failed, try cache
          const cachedResponse = await caches.match(request);
          if (cachedResponse) {
            return cachedResponse;
          }
          // Fallback to index.html for SPA routing
          return caches.match('./index.html');
        })
    );
    return;
  }

  // Strategy 3: Static assets - Cache-first with network fallback
  if (isCacheableAsset(requestUrl)) {
    event.respondWith(
      caches.match(request)
        .then((cachedResponse) => {
          if (cachedResponse) {
            // Served from cache - silent success
            return cachedResponse;
          }

          // Not in cache, fetch from network
          return fetch(request)
            .then((response) => {
              // Cache successful responses
              if (response && response.status === 200) {
                const responseClone = response.clone();
                caches.open(ASSET_CACHE_NAME)
                  .then((cache) => cache.put(request, responseClone));
              }
              return response;
            })
            .catch((error) => {
              console.error('[Service Worker] Failed to fetch asset:', requestUrl.pathname, error);
              // Return cached response even if it was rejected earlier
              return cachedResponse;
            });
        })
    );
    return;
  }

  // Strategy 4: Everything else - Network-first
  event.respondWith(
    fetch(request)
      .catch(() => caches.match(request))
  );
});
