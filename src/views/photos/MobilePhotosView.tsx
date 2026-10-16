import { memo, useMemo } from "react";
import { AppShell } from "@/components/AppShell";
import { SearchPanel } from "@/components/SearchPanel";
import { TagCloud } from "@/components/TagCloud";
import { InfiniteFeedStatus } from "@/components/InfiniteFeedStatus";
import { PhotoLightbox, type PhotoSlide } from "@/components/PhotoLightbox";
import { toPhotoSlides } from "@/lib/photoSlides";
import { TagAlbumCard } from "./components/TagAlbumCard";
import { PhotosViewSwitcher } from "./components/PhotosViewSwitcher";
import { MasonryGrid } from "./components/MasonryGrid";
import { AlbumsSkeletonGrid, PhotosWaterfallSkeleton } from "./components/PhotosSkeleton";
import type { VideoItem } from "@/types";
import type { TagAlbum, PhotosDisplayMode } from "@/hooks/useTagAlbums";
import "@/styles/photos-page.css";
import "@/styles/tag-album.css";

type Props = {
  items: VideoItem[];
  total?: number;
  albums: TagAlbum[];
  displayMode: PhotosDisplayMode;
  setDisplayMode: (mode: PhotosDisplayMode) => void;
  hasMore: boolean;
  loadingMore: boolean;
  initialLoading?: boolean;
  failed?: boolean;
  error?: string | null;
  onReload?: () => void;
  onRetry?: () => void;
  loadingAlbums?: boolean;
  albumsError?: string | null;
  onReloadAlbums?: () => void;
  onSetSentinel: (el: HTMLDivElement | null) => void;
  keyword: string;
  tag: string;
  onSelectTag: (tag: string) => void;
  lightboxOpen: boolean;
  lightboxIndex: number;
  slides: PhotoSlide[];
  onOpenLightbox: (slides: PhotoSlide[], index: number) => void;
  onCloseLightbox: () => void;
  onLightboxIndexChange: (index: number) => void;
};

export const MobilePhotosView = memo(function MobilePhotosView({
  items,
  total,
  albums,
  displayMode,
  setDisplayMode,
  hasMore,
  loadingMore,
  initialLoading,
  failed,
  error,
  onReload,
  onRetry,
  loadingAlbums,
  albumsError,
  onReloadAlbums,
  onSetSentinel,
  tag,
  onSelectTag,
  lightboxOpen,
  lightboxIndex,
  slides,
  onOpenLightbox,
  onCloseLightbox,
  onLightboxIndexChange,
}: Props) {
  const allPhotoSlides: PhotoSlide[] = useMemo(() => toPhotoSlides(items), [items]);

  // 计算展示给用户的照片总数：优先使用 API 返回的全量 total，加载中时不抢显错误的小数字
  const effectivePhotoCount =
    total !== undefined && total > 0
      ? total
      : initialLoading
      ? undefined
      : items.length > 0
      ? items.length
      : total;

  const handleOpenAlbum = (album: TagAlbum) => {
    onSelectTag(album.tag);
  };

  const handleOpenSinglePhoto = (_photo: VideoItem, index: number) => {
    onOpenLightbox(allPhotoSlides, index);
  };

  return (
    <AppShell mobileAutoHideNav={false}>
      <div className="photos-page" style={{ paddingBottom: "70px" }}>
        <div className="container photos-page__inner" style={{ padding: "0 12px" }}>
          {/* Mobile compact header */}
          <div style={{ padding: "16px 0 10px" }}>
            <h1 style={{ fontSize: "20px", fontWeight: "700", color: "#f8fafc", margin: "0 0 4px 0" }}>
              {tag ? `#${tag} 相册` : "图集专区"}
            </h1>
            <p style={{ fontSize: "12px", color: "rgba(255, 255, 255, 0.45)", margin: 0 }}>
              {displayMode === "albums" && !tag
                ? `${albums.length} 个主题相册`
                : `共 ${effectivePhotoCount !== undefined ? effectivePhotoCount : items.length} 张图片`}
            </p>

            {tag && (
              <div style={{ marginTop: "10px", display: "flex", alignItems: "center", gap: "8px" }}>
                <button
                  type="button"
                  onClick={() => onSelectTag("")}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "4px",
                    background: "rgba(56, 189, 248, 0.15)",
                    border: "1px solid rgba(56, 189, 248, 0.3)",
                    borderRadius: "6px",
                    padding: "4px 10px",
                    color: "#38bdf8",
                    fontSize: "12px",
                    fontWeight: 600,
                    cursor: "pointer",
                  }}
                >
                  ← 返回相册
                </button>
                <span style={{ fontSize: "12px", color: "rgba(255, 255, 255, 0.6)" }}>
                  #{tag}
                </span>
              </div>
            )}
          </div>

          <SearchPanel navigationPath="/photos" />

          <TagCloud linkBasePath="/photos" />

          {/* Mode Switcher */}
          {!tag && (
            <div style={{ margin: "14px 0 16px" }}>
              <PhotosViewSwitcher
                mode={displayMode}
                onChange={setDisplayMode}
                albumCount={albums.length}
                photoCount={effectivePhotoCount}
              />
            </div>
          )}

          <main id="main-content" className="photos-main" tabIndex={-1}>
            {displayMode === "albums" && !tag ? (
              /* 1. 标签相册列表模式（一次性全量加载完成，独立于单图瀑布流分页） */
              loadingAlbums && albums.length === 0 ? (
                <AlbumsSkeletonGrid count={6} />
              ) : albumsError && albums.length === 0 ? (
                <div className="photos-error-state" style={{ textAlign: "center", padding: "40px 0" }}>
                  <p style={{ color: "#f87171", fontSize: "14px", marginBottom: "12px" }}>
                    {albumsError}
                  </p>
                  {onReloadAlbums && (
                    <button
                      type="button"
                      className="admin-btn admin-btn--primary"
                      onClick={onReloadAlbums}
                      style={{ padding: "6px 16px", fontSize: "13px" }}
                    >
                      重试相册
                    </button>
                  )}
                </div>
              ) : albums.length === 0 ? (
                <div style={{ textAlign: "center", padding: "40px 0", color: "rgba(255, 255, 255, 0.45)" }}>
                  暂无主题相册
                </div>
              ) : (
                <>
                  <div className="tag-albums-grid">
                    {albums.map((album) => (
                      <TagAlbumCard
                        key={album.tag}
                        album={album}
                        onOpenAlbum={handleOpenAlbum}
                        onSelectTag={onSelectTag}
                      />
                    ))}
                  </div>
                  <InfiniteFeedStatus state="end" />
                </>
              )
            ) : (
              /* 2. 单张照片瀑布流模式（支持无限滚动哨兵与分页状态） */
              <>
                {initialLoading && items.length === 0 ? (
                  <PhotosWaterfallSkeleton isMobile={true} />
                ) : failed && items.length === 0 ? (
                  <div className="photos-error-state" style={{ textAlign: "center", padding: "40px 0" }}>
                    <p style={{ color: "#f87171", fontSize: "14px", marginBottom: "12px" }}>
                      {error || "照片图集加载失败，请检查网络"}
                    </p>
                    {onReload && (
                      <button
                        type="button"
                        className="admin-btn admin-btn--primary"
                        onClick={onReload}
                        style={{ padding: "6px 16px", fontSize: "13px" }}
                      >
                        重新加载
                      </button>
                    )}
                  </div>
                ) : items.length === 0 ? (
                  <div style={{ textAlign: "center", padding: "40px 0", color: "rgba(255, 255, 255, 0.45)" }}>
                    暂无匹配的照片
                  </div>
                ) : (
                  <MasonryGrid
                    items={items}
                    isMobile={true}
                    onOpenPhoto={handleOpenSinglePhoto}
                  />
                )}

                <div
                  ref={onSetSentinel}
                  className="photos-sentinel"
                  style={{ height: "20px" }}
                  aria-hidden="true"
                />

                {loadingMore && <InfiniteFeedStatus state="loading" />}
                {failed && items.length > 0 && (
                  <div style={{ textAlign: "center", padding: "12px 0" }}>
                    <button
                      type="button"
                      className="admin-btn"
                      onClick={onRetry}
                      style={{
                        background: "rgba(255, 255, 255, 0.08)",
                        border: "1px solid rgba(255, 255, 255, 0.2)",
                        color: "#fff",
                        padding: "5px 14px",
                        borderRadius: "6px",
                        fontSize: "12px",
                        cursor: "pointer",
                      }}
                    >
                      下一页加载失败，点击重试
                    </button>
                  </div>
                )}
                {!hasMore && items.length > 0 && <InfiniteFeedStatus state="end" />}
              </>
            )}
          </main>
        </div>
      </div>

      <PhotoLightbox
        open={lightboxOpen}
        slides={slides}
        index={lightboxIndex}
        onClose={onCloseLightbox}
        onIndexChange={onLightboxIndexChange}
        showThumbnails={false}
        hasMore={hasMore}
        loadingMore={loadingMore}
      />
    </AppShell>
  );
});
