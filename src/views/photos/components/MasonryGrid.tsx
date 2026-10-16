import { memo, useState } from "react";
import Masonry from "react-masonry-css";
import { Download, Maximize2 } from "lucide-react";
import type { VideoItem } from "@/types";
import "@/styles/masonry.css";
import "@/styles/photos-page.css";

type MasonryGridProps = {
  items: VideoItem[];
  isMobile?: boolean;
  onOpenPhoto: (photo: VideoItem, index: number) => void;
};

export const DESKTOP_MASONRY_BREAKPOINTS = {
  default: 5,
  1440: 4,
  1024: 3,
  768: 2,
};

export const MOBILE_MASONRY_BREAKPOINTS = {
  default: 2,
};

function getPhotoFallbackAspectRatio(id: string): string {
  const RATIOS = ["3 / 4", "4 / 5", "1 / 1", "3 / 4", "2 / 3", "4 / 3"];
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash << 5) - hash + id.charCodeAt(i);
    hash |= 0;
  }
  const index = Math.abs(hash) % RATIOS.length;
  return RATIOS[index];
}

const MasonryPhotoCard = memo(function MasonryPhotoCard({
  item,
  index,
  onOpenPhoto,
}: {
  item: VideoItem;
  index: number;
  onOpenPhoto: (photo: VideoItem, index: number) => void;
}) {
  const [loaded, setLoaded] = useState(false);
  const imageUrl =
    item.thumbnail || item.previewSrc || `/api/video/${item.id}/source`;
  const downloadUrl = `/api/video/${item.id}/source`;
  const aspectRatio =
    item.width && item.height && item.width > 0 && item.height > 0
      ? `${item.width} / ${item.height}`
      : getPhotoFallbackAspectRatio(item.id);

  return (
    <div
      key={item.id}
      data-photo-id={item.id}
      className="masonry-item"
      onClick={() => onOpenPhoto(item, index)}
    >
      <div
        className="masonry-img-wrap"
        style={aspectRatio ? { aspectRatio } : undefined}
      >
        {!loaded && (
          <div
            className="photos-skeleton-shimmer"
            style={{ position: "absolute", inset: 0, zIndex: 1 }}
            aria-hidden="true"
          />
        )}
        <img
          src={imageUrl}
          alt={item.title}
          className={`masonry-img ${loaded ? "is-loaded" : ""}`}
          loading="lazy"
          onLoad={() => setLoaded(true)}
          ref={(el) => {
            if (el?.complete && el.naturalWidth > 0 && !loaded) {
              setLoaded(true);
            }
          }}
        />

        <div className="masonry-overlay">
          <div className="masonry-actions">
            <a
              href={downloadUrl}
              download={`${item.title || "photo"}.jpg`}
              className="masonry-action-btn"
              onClick={(e) => e.stopPropagation()}
              title="下载原图"
            >
              <Download size={14} />
            </a>
            <button
              type="button"
              className="masonry-action-btn"
              onClick={(e) => {
                e.stopPropagation();
                onOpenPhoto(item, index);
              }}
              title="全屏大图预览"
            >
              <Maximize2 size={14} />
            </button>
          </div>

          {item.title && (
            <p className="masonry-title" title={item.title}>
              {item.title}
            </p>
          )}
        </div>
      </div>
    </div>
  );
});

export const MasonryGrid = memo(function MasonryGrid({
  items,
  isMobile = false,
  onOpenPhoto,
}: MasonryGridProps) {
  const breakpointCols = isMobile
    ? MOBILE_MASONRY_BREAKPOINTS
    : DESKTOP_MASONRY_BREAKPOINTS;

  return (
    <Masonry
      breakpointCols={breakpointCols}
      className={`masonry-feed ${
        isMobile ? "masonry-feed--mobile" : "masonry-feed--desktop"
      }`}
      columnClassName={`masonry-column ${
        isMobile ? "masonry-column--mobile" : "masonry-column--desktop"
      }`}
    >
      {items.map((item, index) => (
        <MasonryPhotoCard
          key={item.id}
          item={item}
          index={index}
          onOpenPhoto={onOpenPhoto}
        />
      ))}
    </Masonry>
  );
});
