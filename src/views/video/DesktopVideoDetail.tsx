import { memo, useCallback, useEffect, useState } from "react";
import { AppShell } from "@/components/AppShell";
import { VideoPlayer } from "@/components/VideoPlayer";
import { ImageViewer } from "@/components/ImageViewer";
import { VideoActions } from "@/components/VideoActions";
import { VideoMetaHeader } from "@/components/VideoMetaHeader";
import { VideoInfoPanel } from "@/components/VideoInfoPanel";
import { MobileVideoCollection } from "@/components/MobileVideoCollection";
import { RecommendedRail } from "@/components/RecommendedRail";
import { Tv } from "lucide-react";
import type { useVideoDetailCore } from "@/hooks/useVideoDetailCore";
import "@/styles/desktop-video.css";

type Props = ReturnType<typeof useVideoDetailCore>;

export const DesktopVideoDetail = memo(function DesktopVideoDetail({
  detail,
  visibleRecommendations,
  relatedImages,
  recommendationsLoading,
  recommendationsError,
  retryRecommendations,
  collectionSummary,
  availableTags,
  availableTagsLoading,
  availableTagsError,
  retryAvailableTags,
  tagSaving,
  handleTagsChange,
  deleteSaving,
  handleOpenDelete,
  handleFirstPlay,
  handleReactionCountsChange,
  loadSubtitles,
  isAdmin,
  isPhotoMedia,
  isPikPakMp4,
  navigate,
}: Props) {
  const [isTheater, setIsTheater] = useState<boolean>(() => {
    try {
      return localStorage.getItem("omv_theater_mode") === "true";
    } catch {
      return false;
    }
  });

  const toggleTheater = useCallback(() => {
    setIsTheater((prev) => {
      const next = !prev;
      try {
        localStorage.setItem("omv_theater_mode", String(next));
      } catch {
        // ignore
      }
      return next;
    });
  }, []);

  // Keyboard shortcut listener for desktop (T for Theater Mode)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        (e.target as HTMLElement)?.isContentEditable
      ) {
        return;
      }
      if (e.key === "t" || e.key === "T") {
        e.preventDefault();
        toggleTheater();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [toggleTheater]);

  if (!detail) return null;

  return (
    <AppShell>
      <div className="desktop-video-page">
        {/* Ambient Backlight Glow Canvas / Backdrop */}
        {detail.poster && (
          <div className="desktop-ambient-wrap" aria-hidden="true">
            <div
              className="desktop-ambient-canvas"
              style={{
                backgroundImage: `url(${detail.poster})`,
                backgroundSize: "cover",
                backgroundPosition: "center",
              }}
            />
          </div>
        )}

        <div className="desktop-video-container">
          <div
            className={`desktop-video-layout ${
              isTheater
                ? "desktop-video-layout--theater"
                : "desktop-video-layout--default"
            }`}
          >
            {/* Main Video Section */}
            <div className="desktop-video-main">
              <div className="desktop-player-stage">
                {isPhotoMedia ? (
                  <ImageViewer
                    src={detail.videoSrc}
                    poster={detail.poster}
                    title={detail.title}
                    onView={handleFirstPlay}
                    relatedImages={relatedImages}
                    onSelectImage={(item) =>
                      item.id && navigate(`/video/${item.id}`)
                    }
                  />
                ) : (
                  <VideoPlayer
                    id={detail.id}
                    src={detail.videoSrc}
                    preferTypedMp4SourceOnIOS={isPikPakMp4}
                    poster={detail.poster}
                    previewSrc={detail.previewSrc}
                    loadSubtitles={loadSubtitles}
                    title={detail.title}
                    onFirstPlay={handleFirstPlay}
                    transcodeStatus={detail.transcodeStatus}
                    transcodeError={detail.transcodeError}
                  />
                )}
              </div>

              {/* Theater Mode & Player Tools Switcher */}
              <div className="desktop-player-toolbar">
                <button
                  type="button"
                  className={`desktop-mode-btn ${
                    isTheater ? "desktop-mode-btn--active" : ""
                  }`}
                  onClick={toggleTheater}
                  title="切换宽屏影院模式 (快捷键: T)"
                >
                  <Tv size={15} />
                  <span>{isTheater ? "常规视图" : "宽屏影院"}</span>
                </button>
              </div>

              {/* Meta & Actions */}
              <div className="desktop-video-meta">
                <VideoMetaHeader video={detail} />

                <VideoActions
                  video={detail}
                  onDeleteVideo={handleOpenDelete}
                  deleteSaving={deleteSaving}
                  canDelete={isAdmin}
                  onReactionCountsChange={handleReactionCountsChange}
                />
              </div>

              {/* In Theater Mode, show collection & info below player */}
              {collectionSummary && (
                <div style={{ marginTop: "20px" }}>
                  <MobileVideoCollection
                    videoId={detail.id}
                    collection={collectionSummary}
                  />
                </div>
              )}

              <div style={{ marginTop: "20px" }}>
                <VideoInfoPanel
                  video={detail}
                  availableTags={availableTags}
                  availableTagsLoading={availableTagsLoading}
                  availableTagsError={availableTagsError}
                  onRetryAvailableTags={retryAvailableTags}
                  tagSaving={tagSaving}
                  onTagsChange={isAdmin ? handleTagsChange : undefined}
                />
              </div>

              {/* If Theater Mode is active, recommendations appear below the info panel */}
              {isTheater && (
                <div style={{ marginTop: "32px" }}>
                  <h3 style={{ fontSize: "18px", fontWeight: "600", marginBottom: "16px", color: "#f8fafc" }}>
                    相关推荐与选集
                  </h3>
                  <RecommendedRail
                    videos={visibleRecommendations}
                    videoId={detail.id}
                    collection={collectionSummary ?? undefined}
                    recommendationsLoading={recommendationsLoading}
                    recommendationsError={recommendationsError}
                    onRetryRecommendations={retryRecommendations}
                  />
                </div>
              )}
            </div>

            {/* If Default 2-column mode, rail appears on the right */}
            {!isTheater && (
              <aside className="desktop-video-aside">
                <RecommendedRail
                  videos={visibleRecommendations}
                  videoId={detail.id}
                  collection={collectionSummary ?? undefined}
                  recommendationsLoading={recommendationsLoading}
                  recommendationsError={recommendationsError}
                  onRetryRecommendations={retryRecommendations}
                />
              </aside>
            )}
          </div>
        </div>
      </div>
    </AppShell>
  );
});
