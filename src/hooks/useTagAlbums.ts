import { useCallback, useEffect, useState } from "react";
import type { VideoItem } from "@/types";
import { fetchPhotoAlbums } from "@/data/videos";

export type TagAlbum = {
  tag: string;
  title: string;
  count: number;
  coverPhoto: VideoItem;
  samplePhotos: VideoItem[];
  photos: VideoItem[];
  latestTimestamp: number;
};

export type PhotosDisplayMode = "albums" | "photos";

const STORAGE_KEY_PHOTOS_VIEW = "omv_photos_view_mode";

export function useTagAlbums(_items?: VideoItem[]) {
  const [displayMode, setDisplayModeState] = useState<PhotosDisplayMode>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_PHOTOS_VIEW);
      if (saved === "albums" || saved === "photos") return saved;
    } catch {
      // ignore
    }
    return "albums";
  });

  const [albums, setAlbums] = useState<TagAlbum[]>([]);
  const [loadingAlbums, setLoadingAlbums] = useState(false);

  const setDisplayMode = (mode: PhotosDisplayMode) => {
    setDisplayModeState(mode);
    try {
      localStorage.setItem(STORAGE_KEY_PHOTOS_VIEW, mode);
    } catch {
      // ignore
    }
  };

  const loadServerAlbums = useCallback(async () => {
    setLoadingAlbums(true);
    try {
      const list = await fetchPhotoAlbums();
      const mapped: TagAlbum[] = list.map((a) => {
        const cover =
          a.coverPhoto ||
          a.samplePhotos?.[0] ||
          ({
            id: a.tag,
            href: "",
            title: a.title,
            thumbnail: "",
            previewSrc: "",
            previewDuration: 0,
            previewStrategy: "teaser-file",
            duration: "0",
            badges: [],
            author: "",
            views: 0,
            publishedAt: "",
          } as VideoItem);

        return {
          tag: a.tag,
          title: a.title,
          count: a.count,
          coverPhoto: cover,
          samplePhotos: a.samplePhotos ?? [],
          photos: a.samplePhotos ?? [],
          latestTimestamp: a.latestTimestamp,
        };
      });
      setAlbums(mapped);
    } catch (err) {
      console.warn("Failed to fetch photo albums from server:", err);
    } finally {
      setLoadingAlbums(false);
    }
  }, []);

  useEffect(() => {
    void loadServerAlbums();
  }, [loadServerAlbums]);

  return {
    albums,
    loadingAlbums,
    reloadAlbums: loadServerAlbums,
    displayMode,
    setDisplayMode,
  };
}
