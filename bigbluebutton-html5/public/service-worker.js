const CACHE_VERSION = 'v1';
const CACHE_NAME = `bbb-html5-${CACHE_VERSION}`;
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

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key)),
      ))
      .then(() => self.clients.claim()),
  );
});

const getScopePath = () => new URL(self.location.href).pathname.replace('service-worker.js', '');

const inScope = (url) => {
  const scopePath = getScopePath();
  return url.origin === self.location.origin && url.pathname.startsWith(scopePath);
};

self.addEventListener('message', (event) => {
  const { data } = event;
  if (!data || data.type !== 'PRECACHE_ASSETS' || !Array.isArray(data.assets)) return;

  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(data.assets)),
  );
});

self.addEventListener('fetch', (event) => {
  const { request } = event;

  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (!inScope(url)) return;

  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          const copy = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          return response;
        })
        .catch(async () => (await caches.match(request)) || caches.match('./index.html')),
    );
    return;
  }

  const cacheableDestinations = ['script', 'style', 'font', 'image'];
  const shouldCache = cacheableDestinations.includes(request.destination);

  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;

      return fetch(request)
        .then((response) => {
          if (shouldCache && response && response.status === 200) {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          }
          return response;
        })
        .catch(() => cached);
    }),
  );
});
