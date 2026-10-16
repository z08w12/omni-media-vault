/**
 * OmniMediaVault High-Performance Service Worker
 * Version: 1.6.1
 */

const CACHE_NAME = 'omv-shell-v1.6.1';

const PRECACHE_URLS = [
  '/',
  '/index.html',
  '/manifest.webmanifest',
  '/icon.png',
  '/favicon.ico',
  '/apple-touch-icon.png'
];

// URLs or patterns that MUST bypass Service Worker (video streams, telegram downloads, range requests)
function shouldBypass(request, url) {
  // Only handle GET/HEAD
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    return true;
  }

  // Bypass HTTP Range requests (video seeks, partial content streaming)
  if (request.headers.has('range')) {
    return true;
  }

  const pathname = url.pathname;

  // Bypass media streaming endpoints
  if (
    pathname.includes('/source') ||
    pathname.startsWith('/api/telegram/download') ||
    pathname.startsWith('/api/backup/download') ||
    pathname.endsWith('.mp4') ||
    pathname.endsWith('.m3u8') ||
    pathname.endsWith('.ts')
  ) {
    return true;
  }

  // Bypass non-http(s) schemes like chrome-extension://
  if (!url.protocol.startsWith('http')) {
    return true;
  }

  return false;
}

// Install Event: Precache core application shell
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(PRECACHE_URLS).catch((err) => {
        console.warn('[SW] Precache partial error:', err);
      });
    }).then(() => self.skipWaiting())
  );
});

// Activate Event: Clear outdated caches and take control
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) {
            return caches.delete(key);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// Fetch Event: Cache strategies
self.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new URL(request.url);

  // Critical bypass for media streams and non-GET requests
  if (shouldBypass(request, url)) {
    return;
  }

  // Navigation requests: NetworkFirst with /index.html offline fallback
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          }
          return response;
        })
        .catch(async () => {
          const cached = await caches.match(request);
          if (cached) return cached;
          return caches.match('/index.html');
        })
    );
    return;
  }

  // Hashed static assets (JS, CSS, fonts, SVG) -> Stale-While-Revalidate
  const isStaticAsset =
    url.pathname.startsWith('/assets/') ||
    url.pathname.endsWith('.js') ||
    url.pathname.endsWith('.css') ||
    url.pathname.endsWith('.woff2') ||
    url.pathname.endsWith('.png') ||
    url.pathname.endsWith('.svg') ||
    url.pathname.endsWith('.ico');

  if (isStaticAsset) {
    event.respondWith(
      caches.match(request).then((cachedResponse) => {
        const fetchPromise = fetch(request)
          .then((networkResponse) => {
            if (networkResponse && networkResponse.status === 200) {
              const copy = networkResponse.clone();
              caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
            }
            return networkResponse;
          })
          .catch(() => null);

        return cachedResponse || fetchPromise;
      })
    );
    return;
  }

  // Dynamic API requests -> NetworkFirst with offline cache fallback
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          }
          return response;
        })
        .catch(() => caches.match(request))
    );
    return;
  }
});

// Listener for skipWaiting message from client
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
