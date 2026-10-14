import { memo } from "react";
import { Download, Maximize2 } from "lucide-react";
import type { VideoItem } from "@/types";
import "@/styles/masonry.css";

type MasonryGridProps = {
  items: VideoItem[];
  isMobile?: boolean;
  onOpenPhoto: (photo: VideoItem, index: number) => void;
};

export const MasonryGrid = memo(function MasonryGrid({
  items,
  isMobile = false,
  onOpenPhoto,
}: MasonryGridProps) {
  return (
    <div
      className={`masonry-feed ${
        isMobile ? "masonry-feed--mobile" : "masonry-feed--desktop"
      }`}
    >
      {items.map((item, index) => {
        const imageUrl =
          item.thumbnail || item.previewSrc || `/api/video/${item.id}/source`;
        const downloadUrl = `/api/video/${item.id}/source`;

        return (
          <div
            key={item.id}
            className="masonry-item"
            onClick={() => onOpenPhoto(item, index)}
          >
            <div className="masonry-img-wrap">
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
    </div>
  );
});
