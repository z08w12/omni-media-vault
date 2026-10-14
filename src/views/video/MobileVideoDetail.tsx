import { memo, useState } from "react";
import { AppShell } from "@/components/AppShell";
import { VideoPlayer } from "@/components/VideoPlayer";
import { ImageViewer } from "@/components/ImageViewer";
import { VideoActions } from "@/components/VideoActions";
import { VideoMetaHeader } from "@/components/VideoMetaHeader";
import { VideoInfoPanel } from "@/components/VideoInfoPanel";
import { MobileVideoCollection } from "@/components/MobileVideoCollection";
import { RecommendedRail } from "@/components/RecommendedRail";
import type { useVideoDetailCore } from "@/hooks/useVideoDetailCore";
import "@/styles/mobile-video.css";

type Props = ReturnType<typeof useVideoDetailCore>;

export const MobileVideoDetail = memo(function MobileVideoDetail({
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
  deleteOpen,
  deleteSaving,
  deleteSource,
  setDeleteSource,
  deleteError,
  handleOpenDelete,
  handleCloseDelete,
  handleConfirmDelete,
  handleFirstPlay,
  handleReactionCountsChange,
  loadSubtitles,
  isAdmin,
  isPhotoMedia,
  isPikPakMp4,
  navigate,
}: Props) {
  const [activeTab, setActiveTab] = useState<"intro" | "episodes" | "related">("intro");

  if (!detail) return null;

  return (
    <AppShell mobileAutoHideNav={false}>
      <div className="mobile-video-page">
        {/* Sticky Player Section */}
        <div className="mobile-player-container mobile-player-container--stuck">
          <div className="mobile-player-wrap">
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
        </div>

        {/* Mobile Segmented Navigation Tabs */}
        <nav className="mobile-video-tabs" aria-label="移动端视频详情分页导航">
          <button
            type="button"
            className={`mobile-video-tab ${
              activeTab === "intro" ? "mobile-video-tab--active" : ""
            }`}
            onClick={() => setActiveTab("intro")}
          >
            简介
          </button>

          {collectionSummary && (
            <button
              type="button"
              className={`mobile-video-tab ${
                activeTab === "episodes" ? "mobile-video-tab--active" : ""
              }`}
              onClick={() => setActiveTab("episodes")}
            >
              选集 ({collectionSummary.total})
            </button>
          )}

          <button
            type="button"
            className={`mobile-video-tab ${
              activeTab === "related" ? "mobile-video-tab--active" : ""
            }`}
            onClick={() => setActiveTab("related")}
          >
            推荐 ({visibleRecommendations.length})
          </button>
        </nav>

        {/* Tab Content Panels */}
        <div className="mobile-tab-content">
          {activeTab === "intro" && (
            <section className="mobile-tab-panel">
              <div className="mobile-video-meta">
                <VideoMetaHeader video={detail} />

                <VideoActions
                  video={detail}
                  onDeleteVideo={handleOpenDelete}
                  deleteSaving={deleteSaving}
                  canDelete={isAdmin}
                  onReactionCountsChange={handleReactionCountsChange}
                />
              </div>

              <VideoInfoPanel
                video={detail}
                availableTags={availableTags}
                availableTagsLoading={availableTagsLoading}
                availableTagsError={availableTagsError}
                onRetryAvailableTags={retryAvailableTags}
                tagSaving={tagSaving}
                onTagsChange={isAdmin ? handleTagsChange : undefined}
              />

              {/* Quick teaser of recommendations below intro */}
              <div style={{ marginTop: "24px" }}>
                <h4 style={{ fontSize: "14px", fontWeight: "600", color: "rgba(255, 255, 255, 0.7)", marginBottom: "12px" }}>
                  相关推荐
                </h4>
                <RecommendedRail
                  videos={visibleRecommendations.slice(0, 8)}
                  videoId={detail.id}
                  collection={collectionSummary ?? undefined}
                  recommendationsLoading={recommendationsLoading}
                  recommendationsError={recommendationsError}
                  onRetryRecommendations={retryRecommendations}
                />
              </div>
            </section>
          )}

          {activeTab === "episodes" && collectionSummary && (
            <section className="mobile-tab-panel">
              <MobileVideoCollection
                videoId={detail.id}
                collection={collectionSummary}
              />
            </section>
          )}

          {activeTab === "related" && (
            <section className="mobile-tab-panel">
              <RecommendedRail
                videos={visibleRecommendations}
                videoId={detail.id}
                collection={collectionSummary ?? undefined}
                recommendationsLoading={recommendationsLoading}
                recommendationsError={recommendationsError}
                onRetryRecommendations={retryRecommendations}
              />
            </section>
          )}
        </div>
      </div>

      {/* Admin Delete Confirmation Modal */}
      {deleteOpen && isAdmin && (
        <div className="vd-delete-modal" role="presentation">
          <div
            className="vd-delete-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="vd-delete-title"
          >
            <div className="vd-delete-head">
              <h2 id="vd-delete-title" className="vd-delete-title">
                删除视频
              </h2>
              <p className="vd-delete-text">
                确定删除「{detail.title}」吗？
              </p>
            </div>

            <label className="vd-delete-option">
              <input
                type="checkbox"
                checked={deleteSource}
                disabled={deleteSaving}
                onChange={(e) => setDeleteSource(e.target.checked)}
              />
              <span>
                <strong>同时删除视频源文件</strong>
              </span>
            </label>

            {deleteError && <div className="vd-delete-error">{deleteError}</div>}

            <div className="vd-delete-actions">
              <button
                type="button"
                className="vd-delete-action vd-delete-cancel"
                onClick={handleCloseDelete}
                disabled={deleteSaving}
              >
                取消
              </button>
              <button
                type="button"
                className="vd-delete-action vd-delete-confirm"
                onClick={handleConfirmDelete}
                disabled={deleteSaving}
              >
                {deleteSaving ? "删除中..." : "删除"}
              </button>
            </div>
          </div>
        </div>
      )}
    </AppShell>
  );
});
