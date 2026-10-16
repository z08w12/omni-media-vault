import { memo } from "react";
import "@/styles/photos-page.css";
import "@/styles/tag-album.css";
import "@/styles/masonry.css";

type AlbumsSkeletonProps = {
  count?: number;
};

export const AlbumsSkeletonGrid = memo(function AlbumsSkeletonGrid({
  count = 6,
}: AlbumsSkeletonProps) {
  return (
    <div
      className="tag-albums-grid"
      aria-busy="true"
      aria-label="正在加载相册列表..."
    >
      {Array.from({ length: count }).map((_, index) => (
        <div key={index} className="tag-album-card tag-album-card--skeleton">
          <div className="tag-album-cover-wrap">
            <div className="photos-skeleton-shimmer" />
          </div>
          <div className="tag-album-info">
            <div className="photos-skeleton-bar photos-skeleton-bar--title" />
            <div className="photos-skeleton-bar photos-skeleton-bar--meta" />
          </div>
        </div>
      ))}
    </div>
  );
});

type PhotosSkeletonProps = {
  isMobile?: boolean;
};

export const PhotosWaterfallSkeleton = memo(function PhotosWaterfallSkeleton({
  isMobile = false,
}: PhotosSkeletonProps) {
  const columnCount = isMobile ? 2 : 4;
  const cardsPerColumn = isMobile ? 3 : 2;

  // Staggered realistic aspect ratios for authentic waterfall placeholder feel
  const aspectRatios = isMobile
    ? [
        ["3 / 4", "4 / 5", "1 / 1"],
        ["1 / 1", "3 / 4", "4 / 3"],
      ]
    : [
        ["3 / 4", "4 / 3"],
        ["1 / 1", "3 / 4"],
        ["4 / 5", "1 / 1"],
        ["3 / 4", "4 / 5"],
      ];

  const titleWidths = isMobile
    ? [
        ["75%", "60%", "80%"],
        ["65%", "85%", "55%"],
      ]
    : [
        ["70%", "85%"],
        ["80%", "60%"],
        ["65%", "75%"],
        ["75%", "65%"],
      ];

  return (
    <div
      className={`masonry-feed ${isMobile ? "masonry-feed--mobile" : ""}`}
      aria-busy="true"
      aria-label="正在加载照片列表..."
    >
      {Array.from({ length: columnCount }).map((_, colIdx) => (
        <div
          key={colIdx}
          className={`masonry-column ${isMobile ? "masonry-column--mobile" : ""}`}
          style={{ width: `${100 / columnCount}%` }}
        >
          {Array.from({ length: cardsPerColumn }).map((_, itemIdx) => {
            const ar = aspectRatios[colIdx]?.[itemIdx] || "3 / 4";
            const tw = titleWidths[colIdx]?.[itemIdx] || "70%";
            return (
              <div key={itemIdx} className="masonry-item masonry-item--skeleton">
                <div
                  className="masonry-img-wrap"
                  style={{ aspectRatio: ar }}
                >
                  <div className="photos-skeleton-shimmer" />
                </div>
                <div className="masonry-info" style={{ padding: "8px 10px" }}>
                  <div
                    className="photos-skeleton-bar"
                    style={{ width: tw, height: "12px" }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
});
