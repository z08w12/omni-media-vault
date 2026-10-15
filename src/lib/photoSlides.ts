import type { VideoItem } from "@/types";
import type { PhotoSlide } from "@/components/PhotoLightbox";

export type PhotoItemLike =
  | VideoItem
  | { id: string; title: string; thumbnail?: string; previewSrc?: string };

export function toPhotoSlide(p: PhotoItemLike): PhotoSlide {
  const fullSourceUrl = `/api/video/${p.id}/source`;
  const thumbUrl = p.thumbnail || p.previewSrc || fullSourceUrl;
  return {
    src: fullSourceUrl,
    thumbnail: thumbUrl,
    title: p.title,
    downloadUrl: fullSourceUrl,
    id: p.id,
  };
}

export function toPhotoSlides(items: PhotoItemLike[]): PhotoSlide[] {
  return items.map(toPhotoSlide);
}
