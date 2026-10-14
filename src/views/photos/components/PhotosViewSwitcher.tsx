import { memo } from "react";
import { FolderHeart, LayoutGrid } from "lucide-react";
import type { PhotosDisplayMode } from "@/hooks/useTagAlbums";
import "@/styles/tag-album.css";

type Props = {
  mode: PhotosDisplayMode;
  onChange: (mode: PhotosDisplayMode) => void;
  albumCount?: number;
  photoCount?: number;
};

export const PhotosViewSwitcher = memo(function PhotosViewSwitcher({
  mode,
  onChange,
  albumCount,
  photoCount,
}: Props) {
  return (
    <div className="photos-mode-switcher">
      <button
        type="button"
        className={`photos-mode-btn ${
          mode === "albums" ? "photos-mode-btn--active" : ""
        }`}
        onClick={() => onChange("albums")}
        title="按标签聚合为主题相册展示"
      >
        <FolderHeart size={15} />
        <span>标签相册 {albumCount !== undefined ? `(${albumCount})` : ""}</span>
      </button>

      <button
        type="button"
        className={`photos-mode-btn ${
          mode === "photos" ? "photos-mode-btn--active" : ""
        }`}
        onClick={() => onChange("photos")}
        title="按单张照片流瀑布流展示"
      >
        <LayoutGrid size={15} />
        <span>全部照片 {photoCount !== undefined ? `(${photoCount})` : ""}</span>
      </button>
    </div>
  );
});
