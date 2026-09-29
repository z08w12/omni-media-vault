import { memo, useCallback } from "react";
import Lightbox from "yet-another-react-lightbox";
import Zoom from "yet-another-react-lightbox/plugins/zoom";
import Fullscreen from "yet-another-react-lightbox/plugins/fullscreen";
import Download from "yet-another-react-lightbox/plugins/download";
import Thumbnails from "yet-another-react-lightbox/plugins/thumbnails";
import Counter from "yet-another-react-lightbox/plugins/counter";
import Captions from "yet-another-react-lightbox/plugins/captions";

import "yet-another-react-lightbox/styles.css";
import "yet-another-react-lightbox/plugins/thumbnails.css";
import "yet-another-react-lightbox/plugins/captions.css";
import "yet-another-react-lightbox/plugins/counter.css";
import "@/styles/lightbox.css";

export type PhotoSlide = {
  src: string;
  title?: string;
  description?: string;
  downloadUrl?: string;
  id?: string;
};

export type PhotoLightboxProps = {
  open: boolean;
  slides: PhotoSlide[];
  index: number;
  onClose: () => void;
  onIndexChange?: (index: number) => void;
  showThumbnails?: boolean;
};

export const PhotoLightbox = memo(function PhotoLightbox({
  open,
  slides,
  index,
  onClose,
  onIndexChange,
  showThumbnails = true,
}: PhotoLightboxProps) {
  const handleView = useCallback(
    ({ index: currentIndex }: { index: number }) => {
      onIndexChange?.(currentIndex);
    },
    [onIndexChange]
  );

  const formattedSlides = slides.map((slide) => {
    const download = slide.title
      ? { url: slide.downloadUrl || slide.src, filename: `${slide.title}.jpg` }
      : slide.downloadUrl || slide.src;

    return {
      src: slide.src,
      title: slide.title,
      description: slide.description,
      download,
    };
  });

  const plugins = [
    Zoom,
    Fullscreen,
    Download,
    Counter,
    Captions,
    ...(showThumbnails && slides.length > 1 ? [Thumbnails] : []),
  ];

  return (
    <Lightbox
      open={open}
      close={onClose}
      index={index}
      slides={formattedSlides}
      on={{ view: handleView }}
      plugins={plugins}
      carousel={{
        finite: slides.length <= 1,
        preload: 2,
        padding: "16px",
        spacing: "30%",
      }}
      controller={{
        closeOnBackdropClick: true,
      }}
      zoom={{
        maxZoomPixelRatio: 5,
        zoomInMultiplier: 1.5,
        doubleClickMaxStops: 2,
        scrollToZoom: true,
      }}
      thumbnails={{
        position: "bottom",
        width: 80,
        height: 50,
        border: 2,
        borderRadius: 4,
        padding: 4,
        gap: 8,
        showToggle: true,
      }}
      animation={{
        fade: 250,
        swipe: 300,
      }}
    />
  );
});
