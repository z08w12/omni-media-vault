import { memo, useState } from "react";
import { Download, ExternalLink, Maximize2, Minimize2 } from "lucide-react";

type Props = {
  src: string;
  poster?: string;
  title: string;
  onView?: () => void;
};

export const ImageViewer = memo(function ImageViewer({
  src,
  poster,
  title,
  onView,
}: Props) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState(false);

  const displaySrc = src || poster || "";

  function handleImageClick() {
    setIsFullscreen((prev) => !prev);
    if (onView) onView();
  }

  return (
    <div
      className={`image-viewer-container ${isFullscreen ? "image-viewer--fullscreen" : ""}`}
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Escape" && isFullscreen) {
          setIsFullscreen(false);
        }
      }}
    >
      <div className="image-viewer-stage">
        {!loaded && !error && (
          <div className="image-viewer-loading">
            <span className="preview-loader" />
          </div>
        )}

        {error ? (
          <div className="image-viewer-error">
            <span>图片加载失败</span>
          </div>
        ) : (
          <img
            src={displaySrc}
            alt={title}
            className={`image-viewer-img ${loaded ? "is-loaded" : ""}`}
            onLoad={() => {
              setLoaded(true);
              if (onView) onView();
            }}
            onError={() => {
              setError(true);
              setLoaded(true);
            }}
            onClick={handleImageClick}
            title="点击切换全屏浏览"
          />
        )}
      </div>

      <div className="image-viewer-toolbar">
        <button
          type="button"
          className="image-viewer-btn"
          onClick={() => setIsFullscreen((prev) => !prev)}
          title={isFullscreen ? "退出全屏 (Esc)" : "全屏查看"}
        >
          {isFullscreen ? <Minimize2 size={18} /> : <Maximize2 size={18} />}
          <span>{isFullscreen ? "缩小" : "全屏"}</span>
        </button>

        <a
          href={displaySrc}
          download={title}
          target="_blank"
          rel="noopener noreferrer"
          className="image-viewer-btn"
          title="下载图片原图"
        >
          <Download size={18} />
          <span>下载原图</span>
        </a>

        <a
          href={displaySrc}
          target="_blank"
          rel="noopener noreferrer"
          className="image-viewer-btn"
          title="新标签页打开原图"
        >
          <ExternalLink size={18} />
          <span>原图</span>
        </a>
      </div>
    </div>
  );
});
