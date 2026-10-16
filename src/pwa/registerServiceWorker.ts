/**
 * Service Worker registration and lifecycle manager for OmniMediaVault
 */

export function registerServiceWorker(): void {
  if (typeof window === 'undefined' || !('serviceWorker' in navigator)) {
    return;
  }

  // Register Service Worker on window load to avoid blocking critical page rendering
  window.addEventListener('load', () => {
    navigator.serviceWorker
      .register('/sw.js', { scope: '/' })
      .then((registration) => {
        // Check for updates periodically
        registration.addEventListener('updatefound', () => {
          const installingWorker = registration.installing;
          if (!installingWorker) return;

          installingWorker.addEventListener('statechange', () => {
            if (
              installingWorker.state === 'installed' &&
              navigator.serviceWorker.controller
            ) {
              // New version available! Dispatch event for UI notification if desired
              window.dispatchEvent(new CustomEvent('omv:pwa-update-available'));
            }
          });
        });
      })
      .catch((error) => {
        // Service Worker registration failure is non-fatal
        console.warn('[PWA] Service Worker registration failed:', error);
      });
  });
}

/**
 * Trigger immediate activation of the new service worker and refresh the page.
 */
export function applyServiceWorkerUpdate(): void {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.getRegistration().then((reg) => {
      if (reg && reg.waiting) {
        reg.waiting.postMessage({ type: 'SKIP_WAITING' });
        window.location.reload();
      }
    });
  }
}
