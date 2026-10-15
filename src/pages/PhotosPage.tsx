import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useSearchParams } from "react-router";
import { AdaptiveView } from "@/platform";
import { photosFeedSource } from "@/lib/infiniteFeedSource";
import { readListingSort } from "@/lib/listingSearchParams";
import { MOBILE_VIDEO_PAGE_SIZE, useIsMobile } from "@/lib/responsive";
import { useRouteActivity } from "@/lib/routeActivity";
import { useInfiniteListing } from "@/lib/useInfiniteListing";
import {
  useListingRestoreTarget,
  useListingScrollRestore,
} from "@/lib/useListingScrollRestore";
import { useTagAlbums } from "@/hooks/useTagAlbums";
import { DesktopPhotosView } from "@/views/photos/DesktopPhotosView";
import { MobilePhotosView } from "@/views/photos/MobilePhotosView";
import type { PhotoSlide } from "@/components/PhotoLightbox";

const DESKTOP_PAGE_SIZE = 24;

export default function PhotosPage() {
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const routeActive = useRouteActivity();
  const keyword = params.get("q") ?? "";
  const tag = params.get("tag") ?? "";
  const sort = readListingSort(params);
  const isMobile = useIsMobile();
  const pageSize = isMobile ? MOBILE_VIDEO_PAGE_SIZE : DESKTOP_PAGE_SIZE;

  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [lightboxIndex, setLightboxIndex] = useState(0);
  const [currentSlides, setCurrentSlides] = useState<PhotoSlide[]>([]);

  const source = useMemo(
    () => photosFeedSource({ q: keyword, tag, sort, pageSize }),
    [keyword, tag, sort, pageSize]
  );
  const queryKey = source.key;

  const restoreTarget = useListingRestoreTarget({
    historyKey: location.key,
    queryKey,
    pageSize,
  });
  const listing = useInfiniteListing(source, {
    pausePagination: !routeActive,
    restoreCount: restoreTarget.count,
    restoreFeedToken: restoreTarget.feedToken,
  });
  useListingScrollRestore({
    target: restoreTarget,
    queryKey,
    requestedCount: listing.requestedCount,
    feedToken: listing.feedToken,
    itemCount: listing.items.length,
    active: routeActive,
  });

  const [sentinelEl, setSentinelEl] = useState<HTMLDivElement | null>(null);
  const hasMore = listing.hasMore;
  const loadingMore = listing.loadingMore;
  const initialLoading = listing.initialLoading;
  const failed = listing.failed;
  const error = listing.error ? (listing.error.message || "加载失败") : null;
  const reload = listing.reload;
  const retry = listing.retry;
  const loadMore = listing.loadMore;
  const items = listing.items;

  // Tag album aggregation
  const {
    albums,
    loadingAlbums,
    albumsError,
    reloadAlbums,
    displayMode,
    setDisplayMode,
  } = useTagAlbums(items);

  // 1. IntersectionObserver bottom trigger
  useEffect(() => {
    if (!sentinelEl || !hasMore || loadingMore || !routeActive) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          loadMore();
        }
      },
      { rootMargin: "600px" }
    );
    observer.observe(sentinelEl);
    return () => observer.disconnect();
  }, [sentinelEl, hasMore, loadingMore, loadMore, routeActive]);

  // 2. Window scroll bottom trigger fallback
  useEffect(() => {
    if (!routeActive || !hasMore || loadingMore) return;

    const checkAndLoadMore = () => {
      if (!hasMore || loadingMore) return;
      const scrollHeight = document.documentElement.scrollHeight;
      const scrollTop = window.scrollY || document.documentElement.scrollTop;
      const clientHeight = window.innerHeight;
      if (scrollHeight - scrollTop - clientHeight < 600) {
        loadMore();
      }
    };

    window.addEventListener("scroll", checkAndLoadMore, { passive: true });
    return () => window.removeEventListener("scroll", checkAndLoadMore);
  }, [hasMore, loadingMore, loadMore, routeActive]);

  const handleSelectTag = useCallback(
    (newTag: string) => {
      const nextParams = new URLSearchParams(params);
      if (newTag) {
        nextParams.set("tag", newTag);
      } else {
        nextParams.delete("tag");
      }
      setParams(nextParams);
    },
    [params, setParams]
  );

  const handleOpenLightbox = useCallback(
    (slides: PhotoSlide[], index: number) => {
      setCurrentSlides(slides);
      setLightboxIndex(index);
      setLightboxOpen(true);
    },
    []
  );

  // 当处于大图全屏状态且底层加载了新图片时，平滑动态追加到全屏幻灯片列表
  useEffect(() => {
    if (!lightboxOpen) return;
    setCurrentSlides((prev) => {
      const existingIds = new Set(prev.map((s) => s.id));
      const newSlides: PhotoSlide[] = [];
      for (const p of items) {
        if (!existingIds.has(p.id)) {
          const fullSourceUrl = `/api/video/${p.id}/source`;
          const thumbUrl = p.thumbnail || p.previewSrc || fullSourceUrl;
          newSlides.push({
            src: fullSourceUrl,
            thumbnail: thumbUrl,
            title: p.title,
            downloadUrl: fullSourceUrl,
            id: p.id,
          });
          existingIds.add(p.id);
        }
      }
      return newSlides.length > 0 ? [...prev, ...newSlides] : prev;
    });
  }, [items, lightboxOpen]);

  // 全屏浏览滑动到接近末尾时（剩余 <= 3 张），预加载下一批图片
  const handleLightboxIndexChange = useCallback(
    (index: number) => {
      setLightboxIndex(index);
      if (hasMore && !loadingMore && index >= currentSlides.length - 3) {
        loadMore();
      }
    },
    [hasMore, loadingMore, currentSlides.length, loadMore]
  );

  const handleCloseLightbox = useCallback(() => {
    setLightboxOpen(false);
    // 退出大图查看器时平滑联动定位到最后浏览的图片
    const lastViewedSlide = currentSlides[lightboxIndex];
    if (lastViewedSlide?.id) {
      const cardEl = document.querySelector(
        `[data-photo-id="${lastViewedSlide.id}"]`
      );
      if (cardEl) {
        cardEl.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    }
  }, [currentSlides, lightboxIndex]);

  const viewProps = {
    items,
    albums,
    displayMode,
    setDisplayMode,
    hasMore,
    loadingMore,
    initialLoading,
    failed,
    error,
    onReload: reload,
    onRetry: retry,
    loadingAlbums,
    albumsError,
    onReloadAlbums: reloadAlbums,
    onSetSentinel: setSentinelEl,
    keyword,
    tag,
    onSelectTag: handleSelectTag,
    lightboxOpen,
    lightboxIndex,
    slides: currentSlides,
    onOpenLightbox: handleOpenLightbox,
    onCloseLightbox: handleCloseLightbox,
    onLightboxIndexChange: handleLightboxIndexChange,
  };

  return (
    <AdaptiveView
      desktop={<DesktopPhotosView {...viewProps} />}
      mobile={<MobilePhotosView {...viewProps} />}
    />
  );
}
