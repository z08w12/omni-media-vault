import { useMemo, useState } from "react";
import type { VideoItem } from "@/types";

export type TagAlbum = {
  tag: string;
  title: string;
  count: number;
  coverPhoto: VideoItem;
  samplePhotos: VideoItem[];
  photos: VideoItem[];
  latestTimestamp: number;
};

// System tags that shouldn't form their own independent content albums
const SYSTEM_TAGS = new Set([
  "图片",
  "tg",
  "telegram",
  "x",
  "twitter",
  "youtube",
  "douyin",
  "tiktok",
  "bilibili",
  "web",
]);

export type PhotosDisplayMode = "albums" | "photos";

const STORAGE_KEY_PHOTOS_VIEW = "omv_photos_view_mode";

export function useTagAlbums(items: VideoItem[]) {
  const [displayMode, setDisplayModeState] = useState<PhotosDisplayMode>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY_PHOTOS_VIEW);
      if (saved === "albums" || saved === "photos") return saved;
    } catch {
      // ignore
    }
    return "albums";
  });

  const setDisplayMode = (mode: PhotosDisplayMode) => {
    setDisplayModeState(mode);
    try {
      localStorage.setItem(STORAGE_KEY_PHOTOS_VIEW, mode);
    } catch {
      // ignore
    }
  };

  const albums = useMemo<TagAlbum[]>(() => {
    const albumMap = new Map<string, VideoItem[]>();
    const uncategorized: VideoItem[] = [];

    for (const item of items) {
      const tags = (item.tags ?? []).filter(
        (t) => !SYSTEM_TAGS.has(t.trim().toLowerCase())
      );

      if (tags.length === 0) {
        uncategorized.push(item);
      } else {
        for (const t of tags) {
          const cleanTag = t.trim();
          if (!cleanTag) continue;
          let list = albumMap.get(cleanTag);
          if (!list) {
            list = [];
            albumMap.set(cleanTag, list);
          }
          list.push(item);
        }
      }
    }

    const result: TagAlbum[] = [];

    for (const [tag, photos] of albumMap.entries()) {
      if (photos.length === 0) continue;
      // Photos sorted newest first
      const sortedPhotos = [...photos].sort((a, b) => {
        const timeA = new Date(a.publishedAt || 0).getTime();
        const timeB = new Date(b.publishedAt || 0).getTime();
        return timeB - timeA;
      });

      result.push({
        tag,
        title: tag.startsWith("#") ? tag : `#${tag}`,
        count: sortedPhotos.length,
        coverPhoto: sortedPhotos[0],
        samplePhotos: sortedPhotos.slice(0, 4),
        photos: sortedPhotos,
        latestTimestamp: new Date(sortedPhotos[0].publishedAt || 0).getTime(),
      });
    }

    // Sort albums by: count desc, then latest update
    result.sort((a, b) => {
      if (b.count !== a.count) return b.count - a.count;
      return b.latestTimestamp - a.latestTimestamp;
    });

    if (uncategorized.length > 0) {
      const sortedUncat = [...uncategorized].sort((a, b) => {
        return (
          new Date(b.publishedAt || 0).getTime() -
          new Date(a.publishedAt || 0).getTime()
        );
      });
      result.push({
        tag: "未分类",
        title: "未分类图集",
        count: sortedUncat.length,
        coverPhoto: sortedUncat[0],
        samplePhotos: sortedUncat.slice(0, 4),
        photos: sortedUncat,
        latestTimestamp: new Date(sortedUncat[0].publishedAt || 0).getTime(),
      });
    }

    return result;
  }, [items]);

  return {
    albums,
    displayMode,
    setDisplayMode,
  };
}
