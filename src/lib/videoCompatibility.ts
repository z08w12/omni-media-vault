import { useEffect, useState } from "react";

const BROWSER_PLAYABLE_EXTENSIONS = new Set(["mp4", "m4v", "webm"]);

const STORAGE_KEY = "omni_unsupported_videos";

type Listener = () => void;
const listeners = new Set<Listener>();

function notify() {
  for (const listener of listeners) {
    try {
      listener();
    } catch {
      // ignore listener errors
    }
  }
}

function getStoredUnsupportedIds(): Set<string> {
  if (typeof window === "undefined" || !window.localStorage) {
    return new Set();
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return new Set();
    const list = JSON.parse(raw);
    return new Set(Array.isArray(list) ? list : []);
  } catch {
    return new Set();
  }
}

function saveStoredUnsupportedIds(ids: Set<string>) {
  if (typeof window === "undefined" || !window.localStorage) return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(Array.from(ids)));
  } catch {
    // ignore storage quota errors
  }
}

/**
 * 判断视频文件容器/格式在标准浏览器下是否原生可播放（如 mp4, m4v, webm）。
 * mkv, avi, wmv, flv, mov, rmvb, ts 等非标准格式返回 false。
 */
export function isFormatBrowserPlayable(ext?: string): boolean {
  const clean = (ext || "").toLowerCase().replace(/^\./, "").trim();
  return BROWSER_PLAYABLE_EXTENSIONS.has(clean);
}

/**
 * 检查指定视频是否曾触发浏览器播放器解码失败（如虽然是 mp4 但使用了 HEVC/DTS 或文件损坏）。
 */
export function isPlaybackErrorMarked(videoId?: string): boolean {
  if (!videoId) return false;
  return getStoredUnsupportedIds().has(videoId);
}

/**
 * 综合判定视频是否支持浏览器端播放：
 * 容器格式必须为浏览器支持格式，且没有被探测到解码异常。
 */
export function isBrowserPlayable(video?: { id: string; ext: string }): boolean {
  if (!video) return false;
  if (!isFormatBrowserPlayable(video.ext)) {
    return false;
  }
  return !isPlaybackErrorMarked(video.id);
}

/**
 * 当浏览器播放器解码报错时调用，将该视频标记为浏览器不支持播放。
 */
export function markVideoPlaybackUnsupported(videoId: string) {
  if (!videoId) return;
  const set = getStoredUnsupportedIds();
  if (!set.has(videoId)) {
    set.add(videoId);
    saveStoredUnsupportedIds(set);
    notify();
  }
}

/**
 * 转码完成或重新恢复时调用，清除不支持标记。
 */
export function clearVideoPlaybackUnsupported(videoId: string) {
  if (!videoId) return;
  const set = getStoredUnsupportedIds();
  if (set.delete(videoId)) {
    saveStoredUnsupportedIds(set);
    notify();
  }
}

export function subscribeCompatibilityChanges(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useVideoCompatibility() {
  const [, setVersion] = useState(0);
  useEffect(() => {
    return subscribeCompatibilityChanges(() => setVersion((v) => v + 1));
  }, []);
  return {
    isBrowserPlayable,
    isFormatBrowserPlayable,
    isPlaybackErrorMarked,
    markVideoPlaybackUnsupported,
    clearVideoPlaybackUnsupported,
  };
}
