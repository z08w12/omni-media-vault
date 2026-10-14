import { memo, useMemo } from "react";
import { AppShell } from "@/components/AppShell";
import { SearchPanel } from "@/components/SearchPanel";
import { TagCloud } from "@/components/TagCloud";
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

export const MobilePhotosView = memo(function MobilePhotosView({
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
                ? `${albums.length} 个标签相册 · ${items.length} 张图片`
                : `${items.length} 张图片`}
            </p>
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
                photoCount={items.length}
              />
            </div>
          )}

          <main id="main-content" className="photos-main" tabIndex={-1}>
            {items.length === 0 ? (
              <div style={{ textAlign: "center", padding: "40px 0", color: "rgba(255, 255, 255, 0.45)" }}>
                暂无匹配的照片
              </div>
            ) : displayMode === "albums" && !tag ? (
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
        showThumbnails={false}
      />
    </AppShell>
  );
});
