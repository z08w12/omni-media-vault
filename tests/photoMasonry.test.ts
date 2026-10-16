import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const masonryGridSource = readFileSync(
  new URL("../src/views/photos/components/MasonryGrid.tsx", import.meta.url),
  "utf8"
);
const masonryCss = readFileSync(
  new URL("../src/styles/masonry.css", import.meta.url),
  "utf8"
);

test("MasonryGrid uses third-party react-masonry-css for true sequential waterfall", () => {
  assert.match(masonryGridSource, /import Masonry from "react-masonry-css";/);
  assert.match(masonryGridSource, /<Masonry\b/);
  assert.doesNotMatch(masonryCss, /column-count/);
});

test("MasonryGrid breakpoints define responsive column counts", () => {
  assert.match(masonryGridSource, /default:\s*5/);
  assert.match(masonryGridSource, /1440:\s*4/);
  assert.match(masonryGridSource, /1024:\s*3/);
  assert.match(masonryGridSource, /768:\s*2/);
  assert.match(masonryGridSource, /MOBILE_MASONRY_BREAKPOINTS = \{\s*default:\s*2/);
});

test("MasonryGrid preserves item aspect ratio and photo click index", () => {
  assert.match(masonryGridSource, /item\.width && item\.height/);
  assert.match(masonryGridSource, /style=\{aspectRatio \? \{ aspectRatio \} : undefined\}/);
  assert.match(masonryGridSource, /onClick=\{\(\) => onOpenPhoto\(item, index\)\}/);
});

test("masonry.css provides flex column waterfall layout with gutter offsets", () => {
  assert.match(masonryCss, /\.masonry-feed\s*\{[^}]*display:\s*flex;/);
  assert.match(masonryCss, /\.masonry-column\s*\{[^}]*padding-left:/);
  assert.match(masonryCss, /\.masonry-img-wrap\[style\*="aspect-ratio"\]\s*\.masonry-img\s*\{[^}]*height:\s*100%;/);
});

test("MasonryGrid guarantees non-zero card height and per-card loading shimmer", () => {
  // Fallback aspect ratio is computed for items missing dimension metadata in db
  assert.match(masonryGridSource, /getPhotoFallbackAspectRatio/);
  assert.match(masonryGridSource, /photos-skeleton-shimmer/);
  assert.match(masonryGridSource, /is-loaded/);

  // CSS ensures min-height and aspect-ratio fallback so card never collapses to 2px line
  assert.match(masonryCss, /\.masonry-img-wrap\s*\{[^}]*min-height:\s*160px;/);
  assert.match(masonryCss, /\.masonry-img-wrap\s*\{[^}]*aspect-ratio:\s*3\s*\/\s*4;/);
});
