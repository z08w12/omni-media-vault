import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const photosPageSource = readFileSync(
  new URL("../src/pages/PhotosPage.tsx", import.meta.url),
  "utf8"
);
const mobileViewSource = readFileSync(
  new URL("../src/views/photos/MobilePhotosView.tsx", import.meta.url),
  "utf8"
);
const desktopViewSource = readFileSync(
  new URL("../src/views/photos/DesktopPhotosView.tsx", import.meta.url),
  "utf8"
);

test("PhotosPage pauses listing pagination and guards bottom triggers during albums mode", () => {
  // Verifies isAlbumsMode calculation
  assert.match(photosPageSource, /const isAlbumsMode = displayMode === "albums" && !tag;/);
  // Verifies pausePagination includes isAlbumsMode
  assert.match(photosPageSource, /pausePagination:\s*!routeActive\s*\|\|\s*isAlbumsMode/);
  // Verifies listing scroll restore is bypassed in albums mode
  assert.match(photosPageSource, /active:\s*routeActive\s*&&\s*!isAlbumsMode/);
  // Verifies IntersectionObserver guards against albums mode
  assert.match(photosPageSource, /if\s*\([^)]*isAlbumsMode\)\s*return;/);
  // Verifies scroll event listener guards against albums mode
  assert.match(photosPageSource, /checkAndLoadMore\s*=\s*\(\)\s*=>\s*\{[^}]*isAlbumsMode[^}]*return;/);
});

test("MobilePhotosView separates album view from photo feed and prevents sentinel spinner in album mode", () => {
  // displayMode check is prioritized before single photo checks
  const albumsModeBranchIndex = mobileViewSource.indexOf('displayMode === "albums" && !tag');
  const itemsCheckIndex = mobileViewSource.indexOf('initialLoading && items.length === 0');
  assert.ok(
    albumsModeBranchIndex > 0 && albumsModeBranchIndex < itemsCheckIndex,
    "Albums mode branch must precede items photo listing checks"
  );

  // Sentinel is isolated within the photo masonry branch, NOT rendered in albums branch
  const albumsBlock = mobileViewSource.slice(albumsModeBranchIndex, itemsCheckIndex);
  assert.doesNotMatch(albumsBlock, /onSetSentinel/);
  assert.doesNotMatch(albumsBlock, /loadingMore && <InfiniteFeedStatus/);

  // Albums block shows clean end status
  assert.match(albumsBlock, /<InfiniteFeedStatus state="end" \/>/);
});

test("DesktopPhotosView mirrors album view isolation and sentinel safeguards", () => {
  const albumsModeBranchIndex = desktopViewSource.indexOf('displayMode === "albums" && !tag');
  const itemsCheckIndex = desktopViewSource.indexOf('initialLoading && items.length === 0');
  assert.ok(
    albumsModeBranchIndex > 0 && albumsModeBranchIndex < itemsCheckIndex,
    "Albums mode branch must precede items photo listing checks on desktop"
  );

  const albumsBlock = desktopViewSource.slice(albumsModeBranchIndex, itemsCheckIndex);
  assert.doesNotMatch(albumsBlock, /onSetSentinel/);
  assert.doesNotMatch(albumsBlock, /loadingMore && <InfiniteFeedStatus/);
  assert.match(albumsBlock, /<InfiniteFeedStatus state="end" \/>/);
});
