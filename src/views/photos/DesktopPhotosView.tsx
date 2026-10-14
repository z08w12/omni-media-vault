import { memo, useMemo } from "react";
import { AppShell } from "@/components/AppShell";
import { SearchPanel } from "@/components/SearchPanel";
import { TagCloud } from "@/components/TagCloud";
import { PromoStrip } from "@/components/PromoStrip";
import { InfiniteFeedStatus } from "@/components/InfiniteFeedStatus";
import { PhotoLightbox, type PhotoSlide } from "@/components/PhotoLightbox";
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
  const allPhotoSlides: PhotoSlide[] = useMemo(() => {
    return items.map((p) => {
      const src = p.thumbnail || p.previewSrc || `/api/video/${p.id}/source`;
      const downloadUrl = `/api/video/${p.id}/source`;
      return {
        src,
        title: p.title,
        downloadUrl,
        id: p.id,
      };
    });
  }, [items]);

  const handleOpenAlbum = (album: TagAlbum) => {
    const albumSlides: PhotoSlide[] = album.photos.map((p) => {
      const src = p.thumbnail || p.previewSrc || `/api/video/${p.id}/source`;
      const downloadUrl = `/api/video/${p.id}/source`;
      return {
        src,
        title: p.title,
        downloadUrl,
        id: p.id,
      };
    });
    onOpenLightbox(albumSlides, 0);
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
                {tag ? `#${tag} 图集` : "高清相册与图集专区"}
              </h1>
              <span className="photos-header__count">
                {displayMode === "albums" && !tag
                  ? `共 ${albums.length} 个标签相册 · ${items.length} 张照片`
                  : `共 ${items.length} 张照片`}
              </span>
            </div>

            <p className="photos-header__desc">
              私有高清照片相册归档 · 基于标签主题聚合 · 支持无损原图缩放与下载
            </p>
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
            {items.length === 0 ? (
              <div className="photos-empty" style={{ textAlign: "center", padding: "60px 0" }}>
                <p style={{ color: "rgba(255, 255, 255, 0.5)", fontSize: "15px" }}>
                  暂无匹配的照片图集
                </p>
              </div>
            ) : displayMode === "albums" && !tag ? (
              /* Tag Albums Grid */
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
      />
    </AppShell>
  );
});
