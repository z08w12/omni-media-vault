import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

test("manifest.webmanifest strictly adheres to PWA installability criteria", () => {
  const manifestPath = path.resolve(process.cwd(), "public/manifest.webmanifest");
  assert.ok(fs.existsSync(manifestPath), "manifest.webmanifest must exist");

  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf-8"));
  assert.equal(manifest.name, "OmniMediaVault");
  assert.equal(manifest.short_name, "OmniVault");
  assert.equal(manifest.display, "standalone");
  assert.ok(
    Array.isArray(manifest.display_override) &&
      manifest.display_override.includes("fullscreen") &&
      manifest.display_override.includes("standalone"),
    "display_override must include fullscreen and standalone"
  );
  assert.equal(manifest.theme_color, "#0a0d14");
  assert.equal(manifest.background_color, "#0a0d14");

  // Verify icons
  assert.ok(Array.isArray(manifest.icons) && manifest.icons.length >= 3);
  const has192 = manifest.icons.some((i: any) => i.sizes === "192x192");
  const has512 = manifest.icons.some((i: any) => i.sizes === "512x512");
  const hasMaskable = manifest.icons.some((i: any) => i.purpose === "maskable");
  assert.ok(has192, "Must contain 192x192 icon");
  assert.ok(has512, "Must contain 512x512 icon");
  assert.ok(hasMaskable, "Must contain maskable icon");

  // Verify shortcuts
  assert.ok(Array.isArray(manifest.shortcuts) && manifest.shortcuts.length >= 3);
  const shortcutUrls = manifest.shortcuts.map((s: any) => s.url);
  assert.ok(shortcutUrls.includes("/"), "Must have shortcut to /");
  assert.ok(shortcutUrls.includes("/shorts"), "Must have shortcut to /shorts");
  assert.ok(shortcutUrls.includes("/photos"), "Must have shortcut to /photos");
});

test("index.html specifies complete PWA meta tags and Apple fullscreen properties", () => {
  const htmlPath = path.resolve(process.cwd(), "index.html");
  assert.ok(fs.existsSync(htmlPath), "index.html must exist");

  const html = fs.readFileSync(htmlPath, "utf-8");
  assert.ok(html.includes('rel="manifest" href="/manifest.webmanifest"'), "Must link manifest");
  assert.ok(html.includes('rel="apple-touch-icon"'), "Must link apple-touch-icon");
  assert.ok(html.includes('name="mobile-web-app-capable" content="yes"'), "Must set mobile-web-app-capable");
  assert.ok(html.includes('name="apple-mobile-web-app-capable" content="yes"'), "Must set apple-mobile-web-app-capable");
  assert.ok(html.includes('name="apple-mobile-web-app-status-bar-style" content="black-translucent"'), "Must set status-bar black-translucent");
  assert.ok(html.includes("viewport-fit=cover"), "Must specify viewport-fit=cover for notch display");
  assert.ok(html.includes('name="theme-color" content="#0a0d14"'), "Must match dark theme color");
});

test("public/sw.js implements stream bypass and offline App Shell fallback", () => {
  const swPath = path.resolve(process.cwd(), "public/sw.js");
  assert.ok(fs.existsSync(swPath), "public/sw.js must exist");

  const swContent = fs.readFileSync(swPath, "utf-8");
  assert.ok(swContent.includes("CACHE_NAME"), "Must define CACHE_NAME");
  assert.ok(swContent.includes("shouldBypass"), "Must define bypass validation logic");
  assert.ok(swContent.includes(".includes('/source')"), "Must bypass media /source endpoints");
  assert.ok(swContent.includes("request.headers.has('range')"), "Must bypass HTTP range requests");
  assert.ok(swContent.includes("caches.match('/index.html')"), "Must fallback navigate requests to cached /index.html");
  assert.ok(swContent.includes("SKIP_WAITING"), "Must handle SKIP_WAITING update message");
});

test("src/pwa/registerServiceWorker.ts integrates lifecycle and update dispatcher", () => {
  const regPath = path.resolve(process.cwd(), "src/pwa/registerServiceWorker.ts");
  assert.ok(fs.existsSync(regPath), "registerServiceWorker.ts must exist");

  const regContent = fs.readFileSync(regPath, "utf-8");
  assert.ok(regContent.includes(".register('/sw.js'"), "Must register /sw.js with scope /");
  assert.ok(regContent.includes("omv:pwa-update-available"), "Must dispatch update event");

  // Check registration in main.tsx
  const mainPath = path.resolve(process.cwd(), "src/main.tsx");
  const mainContent = fs.readFileSync(mainPath, "utf-8");
  assert.ok(mainContent.includes("registerServiceWorker"), "Must invoke registerServiceWorker in main.tsx");
});

test("InstallPrompt component implements dual-platform A2HS and dismissal persistence", () => {
  const compPath = path.resolve(process.cwd(), "src/components/pwa/InstallPrompt.tsx");
  assert.ok(fs.existsSync(compPath), "InstallPrompt.tsx must exist");

  const compContent = fs.readFileSync(compPath, "utf-8");
  assert.ok(compContent.includes("beforeinstallprompt"), "Must handle beforeinstallprompt");
  assert.ok(compContent.includes("isStandalone"), "Must check display-mode standalone");
  assert.ok(compContent.includes("isIOS"), "Must have iOS specific guidance");
  assert.ok(compContent.includes("DISMISSED_STORAGE_KEY"), "Must persist dismissal preference");

  // Check App.tsx integration
  const appPath = path.resolve(process.cwd(), "src/App.tsx");
  const appContent = fs.readFileSync(appPath, "utf-8");
  assert.ok(appContent.includes("<InstallPrompt />"), "App.tsx must render <InstallPrompt />");
});
