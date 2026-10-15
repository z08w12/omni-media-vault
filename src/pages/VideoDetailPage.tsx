import { useParams } from "react-router";
import { AppShell } from "@/components/AppShell";
import { VideoDetailLoading } from "@/components/VideoDetailLoading";
import { AdaptiveView } from "@/platform/AdaptiveView";
import { DesktopVideoDetail } from "@/views/video/DesktopVideoDetail";
import { MobileVideoDetail } from "@/views/video/MobileVideoDetail";
import { useVideoDetailCore } from "@/hooks/useVideoDetailCore";

export default function VideoDetailPage() {
  const { id } = useParams<{ id: string }>();

  return <VideoDetailContent key={id ?? "missing"} id={id} />;
}

function VideoDetailContent({ id }: { id?: string }) {
  const core = useVideoDetailCore(id);
  const {
    detail,
    loading,
    detailError,
    retryDetail,
    deleteOpen,
    deleteSaving,
    deleteSource,
    setDeleteSource,
    deleteError,
    handleCloseDelete,
    handleConfirmDelete,
    isAdmin,
  } = core;

  if (loading) {
    return <VideoDetailLoading isAdmin={isAdmin} />;
  }

  if (!detail) {
    return (
      <AppShell mobileAutoHideNav>
        <div className="vd-page">
          <div className="container vd-page__inner">
            <div className="vd-empty">
              <div>{detailError || "视频不存在或已被移除"}</div>
              {detailError && (
                <button
                  className="vd-empty__retry"
                  type="button"
                  onClick={retryDetail}
                >
                  重试
                </button>
              )}
            </div>
          </div>
        </div>
      </AppShell>
    );
  }

  return (
    <>
      <AdaptiveView
        desktop={<DesktopVideoDetail {...core} />}
        mobile={<MobileVideoDetail {...core} />}
      />

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
    </>
  );
}
