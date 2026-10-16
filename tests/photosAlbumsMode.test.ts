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
const skeletonSource = readFileSync(
  new URL("../src/views/photos/components/PhotosSkeleton.tsx", import.meta.url),
  "utf8"
);
const photosCss = readFileSync(
  new URL("../src/styles/photos-page.css", import.meta.url),
  "utf8"
);
const useTagAlbumsSource = readFileSync(
  new URL("../src/hooks/useTagAlbums.ts", import.meta.url),
  "utf8"
);

test("PhotosPage pauses listing pagination and guards bottom triggers during albums mode", () => {
  assert.match(photosPageSource, /const isAlbumsMode = displayMode === "albums" && !tag;/);
  assert.match(photosPageSource, /pausePagination:\s*!routeActive\s*\|\|\s*isAlbumsMode/);
  assert.match(photosPageSource, /active:\s*routeActive\s*&&\s*!isAlbumsMode/);
  assert.match(photosPageSource, /if\s*\([^)]*isAlbumsMode\)\s*return;/);
  assert.match(photosPageSource, /checkAndLoadMore\s*=\s*\(\)\s*=>\s*\{[^}]*isAlbumsMode[^}]*return;/);
});

test("PhotosPage extracts and passes total to views", () => {
  assert.match(photosPageSource, /const total = listing\.total;/);
  assert.match(photosPageSource, /viewProps = \{[^}]*total,/s);
});

test("MobilePhotosView separates album view from photo feed and prevents sentinel spinner in album mode", () => {
  const albumsModeBranchIndex = mobileViewSource.indexOf('displayMode === "albums" && !tag');
  const itemsCheckIndex = mobileViewSource.indexOf('initialLoading && items.length === 0');
  assert.ok(
    albumsModeBranchIndex > 0 && albumsModeBranchIndex < itemsCheckIndex,
    "Albums mode branch must precede items photo listing checks"
  );

  const albumsBlock = mobileViewSource.slice(albumsModeBranchIndex, itemsCheckIndex);
  assert.doesNotMatch(albumsBlock, /onSetSentinel/);
  assert.doesNotMatch(albumsBlock, /loadingMore && <InfiniteFeedStatus/);
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

test("Views use total for accurate photoCount instead of single-page items.length", () => {
  // Mobile uses total for switcher and header count
  assert.match(mobileViewSource, /effectivePhotoCount\s*=/);
  assert.match(mobileViewSource, /photoCount=\{effectivePhotoCount\}/);
  assert.doesNotMatch(mobileViewSource, /photoCount=\{items\.length\}/);

  // Desktop uses total for switcher and header count
  assert.match(desktopViewSource, /effectivePhotoCount\s*=/);
  assert.match(desktopViewSource, /photoCount=\{effectivePhotoCount\}/);
  assert.doesNotMatch(desktopViewSource, /photoCount=\{items\.length\}/);
});

test("Photo loading replaces 3px preview-loader with realistic skeleton card grids", () => {
  // Neither view should use the 3px line preview-loader for photo page skeletons
  assert.doesNotMatch(mobileViewSource, /<span className="preview-loader"/);
  assert.doesNotMatch(desktopViewSource, /<span className="preview-loader"/);

  // Both views must use AlbumsSkeletonGrid and PhotosWaterfallSkeleton
  assert.match(mobileViewSource, /<AlbumsSkeletonGrid/);
  assert.match(mobileViewSource, /<PhotosWaterfallSkeleton\s+isMobile=\{true\}/);
  assert.match(desktopViewSource, /<AlbumsSkeletonGrid/);
  assert.match(desktopViewSource, /<PhotosWaterfallSkeleton\s+isMobile=\{false\}/);

  // Skeleton component renders aspect-ratio card shimmers
  assert.match(skeletonSource, /export const AlbumsSkeletonGrid/);
  assert.match(skeletonSource, /export const PhotosWaterfallSkeleton/);
  assert.match(photosCss, /\.photos-skeleton-shimmer/);
  assert.match(photosCss, /\.tag-album-card--skeleton/);
  assert.match(photosCss, /\.masonry-item--skeleton/);
});

test("Gallery entry defaults to albums mode and clears legacy localStorage persistence", () => {
  // useTagAlbums must initialize displayMode unconditionally to "albums"
  assert.match(
    useTagAlbumsSource,
    /const \[displayMode, setDisplayModeState\] = useState<PhotosDisplayMode>\("albums"\);/
  );
  // Must purge legacy localStorage key to unstick users previously trapped in "photos" mode
  assert.match(useTagAlbumsSource, /localStorage\.removeItem\(STORAGE_KEY_PHOTOS_VIEW\)/);
  // setDisplayMode must not persist "photos" back to localStorage
  assert.doesNotMatch(useTagAlbumsSource, /localStorage\.setItem\(STORAGE_KEY_PHOTOS_VIEW/);
});

test("PhotosPage coordinates urlMode with displayMode and resets view params on tag selection", () => {
  // PhotosPage defaults urlMode to "albums" unless explicitly specified as "photos"
  assert.match(
    photosPageSource,
    /const urlMode:\s*PhotosDisplayMode\s*=\s*\(\s*params\.get\("view"\)\s*\|\|\s*params\.get\("mode"\)\s*\)\s*===\s*"photos"\s*\?\s*"photos"\s*:\s*"albums";/
  );
  // Syncs URL mode with displayMode
  assert.match(photosPageSource, /if\s*\(urlMode\s*!==\s*displayMode\)\s*\{\s*setDisplayMode\(urlMode\);/);
  // handleSelectTag removes view and mode params to cleanly restore album view
  assert.match(photosPageSource, /nextParams\.delete\("view"\);\s*nextParams\.delete\("mode"\);/);
  // viewProps uses handleSetDisplayMode
  assert.match(photosPageSource, /setDisplayMode:\s*handleSetDisplayMode/);
});
