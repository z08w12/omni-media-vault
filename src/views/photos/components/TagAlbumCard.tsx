import { memo, useState } from "react";
import { FolderHeart, Images } from "lucide-react";
import type { TagAlbum } from "@/hooks/useTagAlbums";
import "@/styles/tag-album.css";
import "@/styles/photos-page.css";

type TagAlbumCardProps = {
  album: TagAlbum;
  onOpenAlbum: (album: TagAlbum) => void;
  onSelectTag?: (tag: string) => void;
};

export const TagAlbumCard = memo(function TagAlbumCard({
  album,
  onOpenAlbum,
  onSelectTag,
}: TagAlbumCardProps) {
  const [coverLoaded, setCoverLoaded] = useState(false);
  const coverUrl =
    album.coverPhoto.thumbnail ||
    album.coverPhoto.previewSrc ||
    `/api/video/${album.coverPhoto.id}/source`;

  return (
    <div
      className="tag-album-card"
      onClick={() => onOpenAlbum(album)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpenAlbum(album);
        }
      }}
    >
      <div className="tag-album-cover-wrap">
        {!coverLoaded && (
          <div
            className="photos-skeleton-shimmer"
            style={{ position: "absolute", inset: 0, zIndex: 1 }}
            aria-hidden="true"
          />
        )}
        <img
          src={coverUrl}
          alt={album.title}
          className={`tag-album-cover ${coverLoaded ? "is-loaded" : ""}`}
          loading="lazy"
          onLoad={() => setCoverLoaded(true)}
          ref={(el) => {
            if (el?.complete && el.naturalWidth > 0 && !coverLoaded) {
              setCoverLoaded(true);
            }
          }}
        />

        {/* Photo count badge */}
        <div className="tag-album-badge">
          <Images size={13} />
          <span>{album.count}</span>
        </div>

        {/* Collage mini preview on hover (desktop) */}
        {album.samplePhotos.length > 1 && (
          <div className="tag-album-collage">
            {album.samplePhotos.slice(1, 4).map((p) => {
              const u = p.thumbnail || `/api/video/${p.id}/source`;
              return (
                <img
                  key={p.id}
                  src={u}
                  alt=""
                  className="tag-album-collage-thumb"
                  loading="lazy"
                />
              );
            })}
          </div>
        )}
      </div>

      <div className="tag-album-info">
        <div className="tag-album-title" title={album.title}>
          <FolderHeart size={15} className="tag-album-title-hash" />
          <span>{album.title}</span>
        </div>

        <div className="tag-album-meta">
          <span>共 {album.count} 张照片</span>
          {onSelectTag && (
            <span
              onClick={(e) => {
                e.stopPropagation();
                onSelectTag(album.tag);
              }}
              style={{
                color: "#38bdf8",
                cursor: "pointer",
                textDecoration: "underline",
              }}
            >
              进入相册 →
            </span>
          )}
        </div>
      </div>
    </div>
  );
});
