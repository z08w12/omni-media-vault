import { useParams } from "react-router";
import { AppShell } from "@/components/AppShell";
import { VideoDetailLoading } from "@/components/VideoDetailLoading";
import { AdaptiveView } from "@/platform";
import { useVideoDetailCore } from "@/hooks/useVideoDetailCore";
import { DesktopVideoDetail } from "@/views/video/DesktopVideoDetail";
import { MobileVideoDetail } from "@/views/video/MobileVideoDetail";

export default function VideoDetailPage() {
  const { id } = useParams<{ id: string }>();

  // Key by video ID to cleanly unmount player and refresh snapshot caches upon route change
  return <VideoDetailContent key={id ?? "missing"} id={id} />;
}

function VideoDetailContent({ id }: { id?: string }) {
  const core = useVideoDetailCore(id);

  if (core.loading) {
    return <VideoDetailLoading isAdmin={core.isAdmin} />;
  }

  if (!core.detail) {
    return (
      <AppShell mobileAutoHideNav>
        <div className="vd-page">
          <div className="container vd-page__inner">
            <div className="vd-empty">
              <div>{core.detailError || "视频不存在或已被移除"}</div>
              {core.detailError && (
                <button
                  className="vd-empty__retry"
                  type="button"
                  onClick={core.retryDetail}
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
    <AdaptiveView
      desktop={<DesktopVideoDetail {...core} />}
      mobile={<MobileVideoDetail {...core} />}
    />
  );
}
