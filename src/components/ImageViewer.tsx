import { memo, useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, Download, ExternalLink, Maximize2 } from "lucide-react";
import { PhotoLightbox, type PhotoSlide } from "./PhotoLightbox";

export type ImageItemInfo = {
  src: string;
  title: string;
  poster?: string;
  id?: string;
};

type Props = {
  src: string;
  poster?: string;
  title: string;
  onView?: () => void;
  relatedImages?: ImageItemInfo[];
  onSelectImage?: (item: ImageItemInfo) => void;
};

type ViewerSlide = PhotoSlide & {
  originalItem?: ImageItemInfo;
};

export const ImageViewer = memo(function ImageViewer({
  src,
  poster,
  title,
  onView,
  relatedImages = [],
  onSelectImage,
}: Props) {
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [activeSlideIndex, setActiveSlideIndex] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState(false);

  const displaySrc = src || poster || "";

  // 构建画廊幻灯片列表：当前图片优先放在第一个，其余相关图片去重跟随
  const slides: ViewerSlide[] = useMemo(() => {
    const list: ViewerSlide[] = [];
    const seen = new Set<string>();

    if (displaySrc) {
      list.push({
        src: displaySrc,
        title: title || "图片",
        downloadUrl: displaySrc,
        originalItem: {
          src: displaySrc,
          title: title || "图片",
          poster,
        },
      });
      seen.add(displaySrc);
    }

    for (const img of relatedImages) {
      const s = img.src || img.poster;
      if (s && !seen.has(s)) {
        seen.add(s);
        list.push({
          src: s,
          title: img.title || "图片",
          downloadUrl: s,
          id: img.id,
          originalItem: img,
        });
      }
    }
    return list;
  }, [displaySrc, title, poster, relatedImages]);

  const currentSlide = slides[activeSlideIndex] ?? slides[0];
  const currentDisplaySrc = currentSlide?.src || displaySrc;
  const currentTitle = currentSlide?.title || title || "图片";

  useEffect(() => {
    setLoaded(false);
    setError(false);
  }, [currentDisplaySrc]);

  function handleOpenLightbox(index = activeSlideIndex) {
    setActiveSlideIndex(index);
    setLightboxOpen(true);
    if (onView) onView();
  }

  function handlePrevImage(e: React.MouseEvent) {
    e.stopPropagation();
    if (slides.length <= 1) return;
    const nextIdx = (activeSlideIndex - 1 + slides.length) % slides.length;
    setActiveSlideIndex(nextIdx);
    const targetItem = slides[nextIdx]?.originalItem;
    if (onSelectImage && targetItem) {
      onSelectImage(targetItem);
    }
  }

  function handleNextImage(e: React.MouseEvent) {
    e.stopPropagation();
    if (slides.length <= 1) return;
    const nextIdx = (activeSlideIndex + 1) % slides.length;
    setActiveSlideIndex(nextIdx);
    const targetItem = slides[nextIdx]?.originalItem;
    if (onSelectImage && targetItem) {
      onSelectImage(targetItem);
    }
  }

  return (
    <div className="image-viewer-container" tabIndex={0}>
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
            src={currentDisplaySrc}
            alt={currentTitle}
            className={`image-viewer-img ${loaded ? "is-loaded" : ""}`}
            onLoad={() => {
              setLoaded(true);
              if (onView) onView();
            }}
            onError={() => {
              setError(true);
              setLoaded(true);
            }}
            onClick={() => handleOpenLightbox(activeSlideIndex)}
            title="点击进入全屏查看器（支持手势缩放与左右切换）"
          />
        )}

        {/* 幻灯片左右悬浮切换按钮（当存在多张图片时展示） */}
        {slides.length > 1 && (
          <>
            <button
              type="button"
              className="image-viewer-nav-btn is-prev"
              onClick={handlePrevImage}
              aria-label="查看上一张图片"
              title="上一张图片"
            >
              <ChevronLeft size={22} />
            </button>
            <button
              type="button"
              className="image-viewer-nav-btn is-next"
              onClick={handleNextImage}
              aria-label="查看下一张图片"
              title="下一张图片"
            >
              <ChevronRight size={22} />
            </button>
          </>
        )}
      </div>

      <div className="image-viewer-toolbar">
        {slides.length > 1 && (
          <span className="image-viewer-counter" aria-label="图片序号">
            {activeSlideIndex + 1} / {slides.length}
          </span>
        )}

        <button
          type="button"
          className="image-viewer-btn"
          onClick={() => handleOpenLightbox(activeSlideIndex)}
          title="全屏多功能查看（支持放大缩小、手势缩放与左右翻页）"
        >
          <Maximize2 size={16} />
          <span>全屏预览</span>
        </button>

        <a
          href={currentDisplaySrc}
          download={currentTitle}
          target="_blank"
          rel="noopener noreferrer"
          className="image-viewer-btn"
          title="下载图片原图"
        >
          <Download size={16} />
          <span>下载原图</span>
        </a>

        <a
          href={currentDisplaySrc}
          target="_blank"
          rel="noopener noreferrer"
          className="image-viewer-btn"
          title="新标签页打开原图"
        >
          <ExternalLink size={16} />
          <span>新标签打开</span>
        </a>
      </div>

      {/* 现代专业全屏预览插件（支持双指缩放、滚轮放大、左右滑动翻页） */}
      <PhotoLightbox
        open={lightboxOpen}
        slides={slides}
        index={activeSlideIndex}
        onClose={() => setLightboxOpen(false)}
        onIndexChange={(idx) => setActiveSlideIndex(idx)}
        showThumbnails={slides.length > 1}
      />
    </div>
  );
});
