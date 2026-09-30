import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Download, Eye, Heart, Images, Maximize2 } from "lucide-react";
import { useLocation, useSearchParams } from "react-router";
import { AdminEmptyVisual } from "@/admin/AdminEmptyVisual";
import { AppShell } from "@/components/AppShell";
import { InfiniteFeedStatus } from "@/components/InfiniteFeedStatus";
import { ListingLoadError } from "@/components/ListingLoadError";
import { PhotoLightbox, type PhotoSlide } from "@/components/PhotoLightbox";
import { PromoStrip } from "@/components/PromoStrip";
import { SearchPanel } from "@/components/SearchPanel";
import { SortToolbar, type ViewMode } from "@/components/SortToolbar";
import { TagCloud } from "@/components/TagCloud";
import { photosFeedSource } from "@/lib/infiniteFeedSource";
import {
  readListingSort,
  readListingView,
  withListingNavigation,
  withListingPage,
  withListingView,
} from "@/lib/listingSearchParams";
import { MOBILE_VIDEO_PAGE_SIZE, useIsMobile } from "@/lib/responsive";
import { useRouteActivity } from "@/lib/routeActivity";
import { useInfiniteListing } from "@/lib/useInfiniteListing";
import {
  useListingRestoreTarget,
  useListingScrollRestore,
} from "@/lib/useListingScrollRestore";
import type { SortKey } from "@/types";
import "@/styles/photos-page.css";

const DESKTOP_PAGE_SIZE = 24;

export default function PhotosPage() {
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const routeActive = useRouteActivity();
  const keyword = params.get("q") ?? "";
  const tag = params.get("tag") ?? "";
  const sort = readListingSort(params);
  const view = readListingView(params);
  const isMobile = useIsMobile();
  const pageSize = isMobile ? MOBILE_VIDEO_PAGE_SIZE : DESKTOP_PAGE_SIZE;

  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [lightboxIndex, setLightboxIndex] = useState(0);

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
  const loadMore = listing.loadMore;
  const items = listing.items;

  // 1. IntersectionObserver 触底观测（采用 callback ref 动态获取 DOM 节点，解决骨架屏切换时 ref 为空导致监听丢失的问题）
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

  // 2. 窗口滚动监听双重兜底：滚动到距离底部 600px 以内时自动加载下一页
  useEffect(() => {
    if (!routeActive || !hasMore || loadingMore) return;

    const checkAndLoadMore = () => {
      if (!hasMore || loadingMore) return;
      const scrollHeight = document.documentElement.scrollHeight;
      const scrollTop = window.scrollY || document.documentElement.scrollTop;
      const clientHeight = window.innerHeight || document.documentElement.clientHeight;

      if (scrollHeight - (scrollTop + clientHeight) < 600) {
        loadMore();
      }
    };

    window.addEventListener("scroll", checkAndLoadMore, { passive: true });
    window.addEventListener("resize", checkAndLoadMore, { passive: true });
    checkAndLoadMore();

    return () => {
      window.removeEventListener("scroll", checkAndLoadMore);
      window.removeEventListener("resize", checkAndLoadMore);
    };
  }, [hasMore, loadingMore, loadMore, routeActive, items.length]);
  const hasContent = items.length > 0;
  const showSkeleton = listing.initialLoading && !hasContent;
  const showEmptyError = listing.failed && !hasContent;
  const showTailError = listing.failed && hasContent;
  const hasActiveFilter = keyword.trim().length > 0 || tag.trim().length > 0;
  const previousQueryKeyRef = useRef(queryKey);

  useEffect(() => {
    document.title = keyword
      ? `搜索图片 "${keyword}"`
      : tag
      ? `图集标签 ${tag}`
      : "图集专区";
  }, [keyword, tag]);

  useEffect(() => {
    if (!params.has("page")) return;
    setParams((current) => withListingPage(current, 1), { replace: true });
  }, [params, setParams]);

  useEffect(() => {
    if (previousQueryKeyRef.current === queryKey) return;
    previousQueryKeyRef.current = queryKey;
    window.scrollTo({ top: 0, behavior: "auto" });
  }, [queryKey]);

  const handleSortChange = useCallback(
    (nextSort: SortKey) => {
      setParams(
        (current) =>
          withListingNavigation(current, {
            q: keyword,
            tag,
            sort: nextSort,
            page: 1,
          }),
        { replace: true }
      );
    },
    [keyword, setParams, tag]
  );

  const handleViewChange = useCallback(
    (nextView: ViewMode) => {
      setParams((current) => withListingView(current, nextView), {
        replace: true,
      });
    },
    [setParams]
  );

  // 转换为 Lightbox 幻灯片数据
  const slides: PhotoSlide[] = useMemo(() => {
    return items.map((item) => ({
      src: item.thumbnail,
      title: item.title,
      description: item.author ? `作者：${item.author}` : undefined,
      downloadUrl: item.thumbnail,
      id: item.id,
    }));
  }, [items]);

  const handleOpenPhoto = useCallback((index: number) => {
    setLightboxIndex(index);
    setLightboxOpen(true);
  }, []);

  return (
    <AppShell>
      <div className="photos-page">
        <div className="container page-section listing-discovery-section">
          <PromoStrip />

          {/* 顶部专区标题横幅 */}
          <div className="photos-header">
            <div className="photos-header__title-row">
              <div className="photos-header__badge">
                <Images size={20} aria-hidden="true" />
                <span>图集专区</span>
              </div>
              <span className="photos-header__count">
                {listing.total > 0 ? `共 ${listing.total} 张精美图片` : ""}
              </span>
            </div>
            <p className="photos-header__desc">
              高清图片与相册专区，支持全屏极速浏览、手势缩放与流畅左右切换
            </p>
          </div>

          <SearchPanel
            variant="uiverse"
            placeholder="搜索图片标题或作者"
            navigationPath="/photos"
            className="search-panel--public search-panel--transparent"
          />
          <TagCloud linkBasePath="/photos" />
        </div>

        <div className="container page-section listing-primary-section">
          <SortToolbar
            sort={sort}
            view={view}
            sortDisabled={listing.initialLoading}
            onSortChange={handleSortChange}
            onViewChange={handleViewChange}
          />

          <main id="main-content" className="photos-main" tabIndex={-1}>
            {showSkeleton ? (
              <div className={`photos-grid view-${view}`}>
                {Array.from({ length: 12 }).map((_, i) => (
                  <div key={`skeleton-${i}`} className="photo-card is-skeleton">
                    <div className="photo-card__thumb-frame is-skeleton" />
                    <div className="photo-card__body">
                      <span className="skeleton-line is-title" />
                      <span className="skeleton-line is-meta" />
                    </div>
                  </div>
                ))}
              </div>
            ) : showEmptyError ? (
              <ListingLoadError
                hasContent={false}
                onRetry={listing.retry}
                emptyClassName="admin-empty-state admin-empty-state--plain listing-empty-state"
              />
            ) : !hasContent ? (
              <div className="photos-empty">
                <AdminEmptyVisual
                  variant={hasActiveFilter ? "no-results" : "empty"}
                  text={hasActiveFilter ? "未找到符合条件的图片" : "暂无图集内容"}
                  className="admin-empty-state admin-empty-state--plain listing-empty-state"
                />
              </div>
            ) : (
              <>
                <div className={`photos-grid view-${view}`}>
                  {items.map((item, index) => (
                    <article key={item.id} className="photo-card">
                      <div
                        className="photo-card__thumb-frame"
                        onClick={() => handleOpenPhoto(index)}
                        title="点击全屏高清预览"
                      >
                        <img
                          src={item.thumbnail}
                          alt={item.title}
                          className="photo-card__img"
                          loading={index < 8 ? "eager" : "lazy"}
                          decoding="async"
                        />
                        <div className="photo-card__hover-mask" aria-hidden="true">
                          <button
                            type="button"
                            className="photo-card__action-btn is-preview"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleOpenPhoto(index);
                            }}
                            title="全屏预览"
                          >
                            <Maximize2 size={18} />
                          </button>
                          <a
                            href={item.thumbnail}
                            download={item.title}
                            className="photo-card__action-btn is-download"
                            onClick={(e) => e.stopPropagation()}
                            target="_blank"
                            rel="noopener noreferrer"
                            title="下载原图"
                          >
                            <Download size={18} />
                          </a>
                        </div>
                        <span className="photo-card__tag">图片</span>
                      </div>

                      <div className="photo-card__body">
                        <h3 className="photo-card__title">
                          <button
                            type="button"
                            className="photo-card__title-btn"
                            onClick={() => handleOpenPhoto(index)}
                            title={item.title}
                          >
                            {item.title}
                          </button>
                        </h3>

                        <div className="photo-card__meta">
                          {item.author && (
                            <span className="photo-card__author" title={item.author}>
                              {item.author}
                            </span>
                          )}
                          <div className="photo-card__stats">
                            {typeof item.views === "number" && item.views > 0 && (
                              <span className="photo-card__stat" title={`浏览量：${item.views}`}>
                                <Eye size={12} />
                                {item.views}
                              </span>
                            )}
                            {typeof item.likes === "number" && item.likes > 0 && (
                              <span className="photo-card__stat" title={`点赞数：${item.likes}`}>
                                <Heart size={12} />
                                {item.likes}
                              </span>
                            )}
                          </div>
                        </div>
                      </div>
                    </article>
                  ))}
                </div>

                <div
                  ref={setSentinelEl}
                  className="photos-sentinel"
                  style={{ height: "20px", margin: "16px 0" }}
                  aria-hidden="true"
                />

                {loadingMore && <InfiniteFeedStatus state="loading" />}

                {hasMore && !loadingMore && !showTailError && (
                  <div className="photos-load-more" style={{ display: "flex", justifyContent: "center", margin: "24px 0" }}>
                    <button
                      type="button"
                      className="btn btn--secondary"
                      onClick={() => loadMore()}
                    >
                      加载更多图片
                    </button>
                  </div>
                )}

                {showTailError ? (
                  <ListingLoadError hasContent onRetry={listing.retry} />
                ) : listing.exhausted ? (
                  <InfiniteFeedStatus state="end" />
                ) : null}
              </>
            )}
          </main>
        </div>

        {/* 全屏现代多功能画廊查看器 */}
        <PhotoLightbox
          open={lightboxOpen}
          slides={slides}
          index={lightboxIndex}
          onClose={() => setLightboxOpen(false)}
          onIndexChange={(idx) => setLightboxIndex(idx)}
          showThumbnails={slides.length > 1}
        />
      </div>
    </AppShell>
  );
}
