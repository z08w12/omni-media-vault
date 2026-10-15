import { memo } from "react";
import { FolderHeart, Images } from "lucide-react";
import type { TagAlbum } from "@/hooks/useTagAlbums";
import "@/styles/tag-album.css";

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
        <img
          src={coverUrl}
          alt={album.title}
          className="tag-album-cover"
          loading="lazy"
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
