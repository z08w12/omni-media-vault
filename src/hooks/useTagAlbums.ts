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
  // 进入图集始终默认展示 "albums"（标签相册）
  const [displayMode, setDisplayModeState] = useState<PhotosDisplayMode>("albums");

  const [albums, setAlbums] = useState<TagAlbum[]>([]);
  const [loadingAlbums, setLoadingAlbums] = useState(false);
  const [albumsError, setAlbumsError] = useState<string | null>(null);

  // 清除旧版本本地存储中可能持久化的 "photos" 模式，避免用户设备被历史缓存劫持
  useEffect(() => {
    try {
      localStorage.removeItem(STORAGE_KEY_PHOTOS_VIEW);
    } catch {
      // ignore
    }
  }, []);

  const setDisplayMode = useCallback((mode: PhotosDisplayMode) => {
    setDisplayModeState(mode);
  }, []);

  const loadServerAlbums = useCallback(async () => {
    setLoadingAlbums(true);
    setAlbumsError(null);
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
      setAlbumsError(err instanceof Error ? err.message : "获取相册失败，请稍后重试");
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
    albumsError,
    reloadAlbums: loadServerAlbums,
    displayMode,
    setDisplayMode,
  };
}
