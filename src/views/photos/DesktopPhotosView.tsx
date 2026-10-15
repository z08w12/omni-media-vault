import { memo, useMemo } from "react";
import { AppShell } from "@/components/AppShell";
import { SearchPanel } from "@/components/SearchPanel";
import { TagCloud } from "@/components/TagCloud";
import { PromoStrip } from "@/components/PromoStrip";
import { InfiniteFeedStatus } from "@/components/InfiniteFeedStatus";
import { PhotoLightbox, type PhotoSlide } from "@/components/PhotoLightbox";
import { toPhotoSlides } from "@/lib/photoSlides";
import { TagAlbumCard } from "./components/TagAlbumCard";
import { PhotosViewSwitcher } from "./components/PhotosViewSwitcher";
import { MasonryGrid } from "./components/MasonryGrid";
import type { VideoItem } from "@/types";
import type { TagAlbum, PhotosDisplayMode } from "@/hooks/useTagAlbums";
import "@/styles/photos-page.css";
import "@/styles/tag-album.css";

type Props = {
  items: VideoItem[];
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

export const DesktopPhotosView = memo(function DesktopPhotosView({
  items,
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
  // Convert items into slides for Lightbox
  const allPhotoSlides: PhotoSlide[] = useMemo(() => toPhotoSlides(items), [items]);

  const handleOpenAlbum = (album: TagAlbum) => {
    onSelectTag(album.tag);
  };

  const handleOpenSinglePhoto = (_photo: VideoItem, index: number) => {
    onOpenLightbox(allPhotoSlides, index);
  };

  return (
    <AppShell>
      <div className="photos-page">
        <div className="container photos-page__inner">
          <PromoStrip />

          <div className="photos-header">
            <div className="photos-header__title-row">
              <h1 style={{ fontSize: "24px", fontWeight: "700", color: "#f8fafc", margin: 0 }}>
                {tag ? `#${tag} 相册` : "高清相册与图集专区"}
              </h1>
              <span className="photos-header__count">
                {displayMode === "albums" && !tag
                  ? `共 ${albums.length} 个主题相册`
                  : `共 ${items.length} 张照片`}
              </span>
            </div>

            <p className="photos-header__desc">
              私有高清照片相册归档 · 基于标签主题聚合 · 支持无损原图缩放与下载
            </p>

            {tag && (
              <div style={{ marginTop: "12px", display: "flex", alignItems: "center", gap: "10px" }}>
                <button
                  type="button"
                  onClick={() => onSelectTag("")}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "6px",
                    background: "rgba(56, 189, 248, 0.15)",
                    border: "1px solid rgba(56, 189, 248, 0.3)",
                    borderRadius: "8px",
                    padding: "6px 14px",
                    color: "#38bdf8",
                    fontSize: "13px",
                    fontWeight: 600,
                    cursor: "pointer",
                  }}
                >
                  ← 返回全部相册
                </button>
                <span style={{ fontSize: "13px", color: "rgba(255, 255, 255, 0.6)" }}>
                  当前相册：<strong style={{ color: "#fff" }}>#{tag}</strong>
                </span>
              </div>
            )}
          </div>

          <SearchPanel navigationPath="/photos" />

          <TagCloud linkBasePath="/photos" />

          {/* View Toolbar & Switcher */}
          {!tag && (
            <div className="photos-view-toolbar">
              <PhotosViewSwitcher
                mode={displayMode}
                onChange={setDisplayMode}
                albumCount={albums.length}
                photoCount={items.length}
              />
            </div>
          )}

          <main id="main-content" className="photos-main" tabIndex={-1}>
            {initialLoading ? (
              <div className="photos-loading-skeleton" style={{ textAlign: "center", padding: "80px 0" }}>
                <span className="preview-loader" style={{ margin: "0 auto 16px auto" }} />
                <p style={{ color: "rgba(255, 255, 255, 0.6)", fontSize: "14px" }}>
                  正在加载照片图集...
                </p>
              </div>
            ) : failed && items.length === 0 ? (
              <div className="photos-error-state" style={{ textAlign: "center", padding: "60px 0" }}>
                <p style={{ color: "#f87171", fontSize: "15px", marginBottom: "16px" }}>
                  {error || "照片图集加载失败，请检查网络连接"}
                </p>
                {onReload && (
                  <button
                    type="button"
                    className="admin-btn admin-btn--primary"
                    onClick={onReload}
                    style={{ padding: "8px 20px" }}
                  >
                    重新加载
                  </button>
                )}
              </div>
            ) : items.length === 0 ? (
              <div className="photos-empty" style={{ textAlign: "center", padding: "60px 0" }}>
                <p style={{ color: "rgba(255, 255, 255, 0.5)", fontSize: "15px" }}>
                  暂无匹配的照片图集
                </p>
              </div>
            ) : displayMode === "albums" && !tag ? (
              /* Tag Albums Grid */
              loadingAlbums && albums.length === 0 ? (
                <div className="photos-loading-skeleton" style={{ textAlign: "center", padding: "80px 0" }}>
                  <span className="preview-loader" style={{ margin: "0 auto 16px auto" }} />
                  <p style={{ color: "rgba(255, 255, 255, 0.6)", fontSize: "14px" }}>
                    正在加载相册列表...
                  </p>
                </div>
              ) : albumsError && albums.length === 0 ? (
                <div className="photos-error-state" style={{ textAlign: "center", padding: "60px 0" }}>
                  <p style={{ color: "#f87171", fontSize: "15px", marginBottom: "16px" }}>
                    {albumsError}
                  </p>
                  {onReloadAlbums && (
                    <button
                      type="button"
                      className="admin-btn admin-btn--primary"
                      onClick={onReloadAlbums}
                      style={{ padding: "8px 20px" }}
                    >
                      重试相册
                    </button>
                  )}
                </div>
              ) : (
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
              )
            ) : (
              /* Staggered Masonry Waterfall Grid */
              <MasonryGrid
                items={items}
                isMobile={false}
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
              <div style={{ textAlign: "center", padding: "16px 0" }}>
                <button
                  type="button"
                  className="admin-btn"
                  onClick={onRetry}
                  style={{
                    background: "rgba(255, 255, 255, 0.08)",
                    border: "1px solid rgba(255, 255, 255, 0.2)",
                    color: "#fff",
                    padding: "6px 16px",
                    borderRadius: "6px",
                    fontSize: "13px",
                    cursor: "pointer",
                  }}
                >
                  下一页加载失败，点击重试
                </button>
              </div>
            )}
            {!hasMore && items.length > 0 && <InfiniteFeedStatus state="end" />}
          </main>
        </div>
      </div>

      <PhotoLightbox
        open={lightboxOpen}
        slides={slides}
        index={lightboxIndex}
        onClose={onCloseLightbox}
        onIndexChange={onLightboxIndexChange}
        showThumbnails={slides.length > 1}
        hasMore={hasMore}
        loadingMore={loadingMore}
      />
    </AppShell>
  );
});
