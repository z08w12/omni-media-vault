import { memo, useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router";
import type { PreviewState, VideoItem } from "@/types";
import {
  prefetchVideoDetail,
  prefetchVideoRecommendations,
} from "@/data/videos";
import { previewController } from "@/lib/previewController";
import {
  shouldInterceptPreviewTap,
  shouldStartInstantPreview,
  usePreviewIntent,
} from "@/lib/previewIntent";
import { useInViewport } from "@/lib/useInViewport";
import { useIsActivePreview, usePreviewEnabled } from "@/lib/useIsActivePreview";
import { preloadVideoDetailPage } from "@/lib/videoDetailRoute";
import { formatCount } from "@/lib/format";
import { isVideoReturnPath, routeToPath } from "@/lib/videoReturnPath";
import { createVideoDetailNavigationState } from "@/lib/videoListingBackground";
import { PreviewVideo } from "./PreviewVideo";
import { VideoThumbnail } from "./VideoThumbnail";

type Props = {
  video: VideoItem;
  eager?: boolean;
  highPriority?: boolean;
};

export const VideoCard = memo(function VideoCard({
  video,
  eager = false,
  highPriority = false,
}: Props) {
  const [previewState, setPreviewState] = useState<PreviewState>("idle");
  const [shouldRenderPreview, setShouldRenderPreview] = useState(false);
  const [progress, setProgress] = useState(0); // 0~1
  const author = video.author.trim();
  const location = useLocation();
  const currentPath = routeToPath(location);
  const linkState = isVideoReturnPath(currentPath)
    ? createVideoDetailNavigationState(currentPath, location)
    : undefined;

  const rootRef = useRef<HTMLElement | null>(null);
  const lastPointerTypeRef = useRef<string>("");
  const canHoverRef = useRef(true);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  const previewIsActive = useIsActivePreview(video.id);
  const previewEnabled = usePreviewEnabled();

  useEffect(() => {
    if (!previewEnabled) cleanup();
  }, [previewEnabled, video.id]);
  const inView = useInViewport(rootRef);

  const isImage = video.duration === "图片";
  const {
    touchPreviewArmedRef,
    clearPreviewIntentTimer,
    startPreviewIntent,
    startTouchPreviewIntent,
  } = usePreviewIntent({
    videoId: video.id,
    previewSrc: video.previewSrc,
    inView,
    isImage,
    setPreviewState,
    onActivatePreview: () => setShouldRenderPreview(true),
  });

  // 当全局活跃卡片不是自己时，立刻停止预览
  useEffect(() => {
    if (
      !previewIsActive &&
      (shouldRenderPreview || touchPreviewArmedRef.current)
    ) {
      cleanup();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [previewIsActive, video.id]);

  // 离开视口时停止预览
  useEffect(() => {
    if (!inView && (shouldRenderPreview || touchPreviewArmedRef.current)) {
      cleanup();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inView]);

  // 卸载时清理
  useEffect(() => {
    return () => {
      cleanup();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const media = window.matchMedia("(hover: hover) and (pointer: fine)");
    const update = () => {
      canHoverRef.current = media.matches;
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  function cleanup() {
    clearPreviewIntentTimer();
    touchPreviewArmedRef.current = false;

    const el = videoRef.current;
    if (el) {
      try {
        el.pause();
        el.removeAttribute("src");
        el.load();
      } catch {
        // noop
      }
    }

    setShouldRenderPreview(false);
    setPreviewState("idle");
    setProgress(0);

    if (previewController.getActiveId() === video.id) {
      previewController.setActiveId(null);
    }
  }

  function stopPreview() {
    cleanup();
  }

  function handlePointerEnter(event: React.PointerEvent<HTMLElement>) {
    lastPointerTypeRef.current = event.pointerType;
    preloadVideoDetailPage();
    if (shouldStartInstantPreview({ pointerType: event.pointerType })) return;
    startPreviewIntent();
  }

  function handlePointerLeave(event: React.PointerEvent<HTMLElement>) {
    if (shouldStartInstantPreview({ pointerType: event.pointerType })) return;
    stopPreview();
  }

  function handlePointerDown(event: React.PointerEvent<HTMLElement>) {
    lastPointerTypeRef.current = event.pointerType;
    prepareDetailNavigation();
  }

  function prepareDetailNavigation() {
    preloadVideoDetailPage();
    void prefetchVideoDetail(video.id);
  }

  function prepareConfirmedDetailNavigation() {
    prepareDetailNavigation();
    void prefetchVideoRecommendations(video.id);
  }

  function handleFocus() {
    preloadVideoDetailPage();
    startPreviewIntent();
  }

  function handleClickCapture(event: React.MouseEvent<HTMLAnchorElement>) {
    if (isImage) {
      prepareConfirmedDetailNavigation();
      return;
    }
    const previewActive =
      previewController.getActiveId() === video.id &&
      (touchPreviewArmedRef.current || shouldRenderPreview);
    if (
      !shouldInterceptPreviewTap({
        previewEnabled: previewController.isEnabled() && Boolean(video.previewSrc),
        pointerType: lastPointerTypeRef.current,
        canHover: canHoverRef.current,
        previewActive,
      })
    ) {
      if (touchPreviewArmedRef.current && !shouldRenderPreview) cleanup();
      prepareConfirmedDetailNavigation();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    startTouchPreviewIntent();
  }

  return (
    <article
      ref={rootRef as React.RefObject<HTMLElement>}
      className="video-card"
      data-preview-enabled={previewEnabled}
      onPointerEnter={handlePointerEnter}
      onPointerLeave={handlePointerLeave}
      onPointerDown={handlePointerDown}
      onFocus={handleFocus}
      onBlur={stopPreview}
    >
      <Link
        to={video.href}
        state={linkState}
        className="video-card__link"
        tabIndex={0}
        onClickCapture={handleClickCapture}
      >
        <div className="thumb-frame">
          <VideoThumbnail
            src={video.thumbnail}
            eager={eager}
            highPriority={highPriority}
          />

          {previewEnabled && shouldRenderPreview && (
            <PreviewVideo
              ref={videoRef}
              src={video.previewSrc}
              state={previewState}
              onCanPlay={() => setPreviewState("playing")}
              onError={() => setPreviewState("error")}
              onTimeUpdate={(p) => setProgress(p)}
            />
          )}

          {previewEnabled && previewState === "loading" && <span className="preview-loader" />}
          {previewEnabled && previewState === "error" && (
            <span className="preview-error">预览加载失败</span>
          )}

          {/* 预览进度条（播放时显示在底部） */}
          {previewEnabled && previewState === "playing" && (
            <div className="preview-progress" aria-hidden="true">
              <div
                className="preview-progress__bar"
                style={{ width: `${Math.min(100, progress * 100)}%` }}
              />
            </div>
          )}

          {/* hover 时右上角 "预览" 角标 */}
          {previewEnabled && previewState === "playing" && (
            <span className="preview-tag" aria-hidden="true">
              预览
            </span>
          )}

          {(video.badges ?? []).length > 0 && (
            <div className="badge-row">
              {video.badges.map((badge) => (
                <span className="video-badge" key={badge}>
                  {badge}
                </span>
              ))}
            </div>
          )}

          {video.sourceLabel && (!previewEnabled || previewState !== "playing") && (
            <span
              className="source-badge"
              data-kind={sourceKindFromLabel(video.sourceLabel)}
              title={`来源：${video.sourceLabel}`}
            >
              {video.sourceLabel}
            </span>
          )}

          <span className="duration">{video.duration}</span>
        </div>

        <h3 className="video-title" title={video.title}>
          {video.title}
        </h3>

        <div className="video-meta">
          {author && (
            <span className="video-meta__author" title={author}>
              {author}
            </span>
          )}
          <span className="video-meta__views">{formatCount(video.views)} 观看</span>
          <span className="video-meta__date">{video.publishedAt}</span>
        </div>
      </Link>
    </article>
  );
});

// 从后端返回的 sourceLabel 推断存储类型（用于颜色标识）。
function sourceKindFromLabel(label: string): string {
  const value = label.toLowerCase();
  if (value.includes("telegram")) return "telegram";
  if (value.includes("本地") || value.includes("localstorage") || value.includes("local storage") || value.includes("upload")) return "localstorage";
  if (value.includes("spider") || value.includes("爬虫") || value.includes("crawler")) return "crawler";
  return "";
}
