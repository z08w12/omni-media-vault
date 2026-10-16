import { memo } from "react";
import Masonry from "react-masonry-css";
import { Download, Maximize2 } from "lucide-react";
import type { VideoItem } from "@/types";
import "@/styles/masonry.css";

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
      {items.map((item, index) => {
        const imageUrl =
          item.thumbnail || item.previewSrc || `/api/video/${item.id}/source`;
        const downloadUrl = `/api/video/${item.id}/source`;
        const aspectRatio =
          item.width && item.height && item.width > 0 && item.height > 0
            ? `${item.width} / ${item.height}`
            : undefined;

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
              <img
                src={imageUrl}
                alt={item.title}
                className="masonry-img"
                loading="lazy"
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
      })}
    </Masonry>
  );
});
