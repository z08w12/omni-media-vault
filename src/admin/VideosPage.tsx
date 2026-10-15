import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import { Link, useSearchParams } from "react-router";
import {
  Ban,
  Check,
  Edit,
  Film,
  RefreshCw,
  Image,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import * as api from "./api";
import { useToast } from "@/components/ToastContext";
import { Modal } from "./Modal";
import { ConfirmModal } from "./ConfirmModal";
import { formatBytes } from "./storageFormat";
import { AdminEmptyVisual } from "./AdminEmptyVisual";
import { AdminPagination } from "./AdminPagination";
import { SpiderIcon } from "./icons/SpiderIcon";
import {
  adminVideosSourceFilter,
  readAdminVideosMediaType,
  readAdminVideosPage,
  readAdminVideosSourceKey,
  withAdminVideosMediaType,
  withAdminVideosPage,
  withAdminVideosSourceKey,
  type AdminVideosMediaType,
  type AdminVideosSourceKey,
} from "./videosSearchParams";
import { useAdminFloatingActionSpace } from "./useAdminFloatingActionSpace";
import {
  useAdminRouteActive,
  useAdminRouteRevalidation,
} from "./AdminRouteCache";
import { SearchPanel } from "@/components/SearchPanel";
import { FilterAllIcon } from "@/components/icons/FilterAllIcon";
import { LocalStorageIcon } from "@/components/icons/LocalStorageIcon";
import { TelegramIcon } from "@/components/icons/TelegramIcon";
import {
  useVideoCompatibility,
  markVideoPlaybackUnsupported,
} from "@/lib/videoCompatibility";

const DESKTOP_CURRENT_VIDEOS_PAGE_SIZE = 16;
const MOBILE_CURRENT_VIDEOS_PAGE_SIZE = 10;
const DESKTOP_BLACKLIST_PAGE_SIZE = 20;
const MOBILE_BLACKLIST_PAGE_SIZE = 20;
const CURRENT_VIDEO_SKELETON_CARD_COUNT = 6;
const VIDEOS_MOBILE_QUERY = "(max-width: 640px)";
const REGEN_PREVIEW_STATUS = "generating";
const REGEN_PREVIEW_POLL_INTERVAL_MS = 2000;
const REGEN_PREVIEW_TRACK_TIMEOUT_MS = 30 * 60 * 1000;
const LOCAL_UPLOAD_SOURCE_ID = "local-upload";

function requestVideoSourceCatalog() {
  return Promise.allSettled([
    api.listCrawlers(),
    api.listVideos({ driveId: LOCAL_UPLOAD_SOURCE_ID, page: 1, size: 1 }),
    Promise.all([
      api.getTelegramStatus(),
      api.listVideos({ sourceKind: "telegram", page: 1, size: 1 }),
    ]).then(([status, videos]) => status.connection.enabled && videos.total > 0),
  ]);
}

type VideoViewKey = "current" | "blacklist";
type PageSetter = Dispatch<SetStateAction<number>>;

type RegenPreviewState = {
  expiresAt: number;
  originalUpdatedAt: number;
};

type VideoAdvancedFilterValues = {
  createdFrom: string;
  createdTo: string;
  durationMinMinutes: string;
  durationMaxMinutes: string;
};

const EMPTY_VIDEO_FILTERS: VideoAdvancedFilterValues = {
  createdFrom: "",
  createdTo: "",
  durationMinMinutes: "",
  durationMaxMinutes: "",
};

/**
 * 视频管理容器：顶部按来源筛选正常视频，黑名单作为独立管理视图；
 * 当前来源、管理视图和页码同步到 URL。
 */
export function VideosPage() {
  const floatingActionPageRef = useAdminFloatingActionSpace<HTMLElement>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [crawlers, setCrawlers] = useState<api.AdminCrawler[]>([]);
  const [hasLocalUploads, setHasLocalUploads] = useState(false);
  const [hasTelegramVideos, setHasTelegramVideos] = useState(false);
  const [sourceCatalogLoaded, setSourceCatalogLoaded] = useState(false);
  const sourceCatalogRequestRef = useRef(0);
  const { show } = useToast();
  const rawTab = searchParams.get("tab");
  const activeView: VideoViewKey = rawTab === "blacklist" ? "blacklist" : "current";
  const activeSourceKey = readAdminVideosSourceKey(searchParams);
  const activeSourceFilter = adminVideosSourceFilter(activeSourceKey);
  const activeMediaType = readAdminVideosMediaType(searchParams);
  const page = readAdminVideosPage(searchParams);
  const selectMediaType = useCallback(
    (nextMediaType: AdminVideosMediaType) => {
      setSearchParams(
        (prev) => {
          const next = withAdminVideosMediaType(prev, nextMediaType);
          next.delete("page");
          return next;
        },
        { replace: true }
      );
    },
    [setSearchParams]
  );
  const setPage = useCallback<PageSetter>((nextPage) => {
    setSearchParams(
      (prev) => {
        const currentPage = readAdminVideosPage(prev);
        const resolvedPage = typeof nextPage === "function" ? nextPage(currentPage) : nextPage;
        return withAdminVideosPage(prev, resolvedPage);
      },
      { replace: true }
    );
  }, [setSearchParams]);

  const refreshSourceCatalog = useCallback(
    async (reportErrors = true) => {
      const requestID = ++sourceCatalogRequestRef.current;
      const results = await requestVideoSourceCatalog();
      if (requestID !== sourceCatalogRequestRef.current) return;
      const [crawlerResult, localUploadResult, telegramResult] = results;

      if (crawlerResult.status === "fulfilled") {
        setCrawlers(crawlerResult.value ?? []);
      } else if (reportErrors) {
        show(
          crawlerResult.reason instanceof Error
            ? crawlerResult.reason.message
            : "爬虫来源加载失败",
          "error"
        );
      }
      if (localUploadResult.status === "fulfilled") {
        setHasLocalUploads(localUploadResult.value.total > 0);
      } else if (reportErrors) {
        show(
          localUploadResult.reason instanceof Error
            ? localUploadResult.reason.message
            : "本地存储来源加载失败",
          "error"
        );
      }
      if (telegramResult.status === "fulfilled") {
        setHasTelegramVideos(telegramResult.value);
      } else if (reportErrors) {
        show("Telegram 来源加载失败", "error");
      }
      setSourceCatalogLoaded(true);
    },
    [show]
  );

  useEffect(() => {
    void refreshSourceCatalog();
    return () => {
      sourceCatalogRequestRef.current += 1;
    };
  }, [refreshSourceCatalog]);

  useAdminRouteRevalidation(() => {
    void refreshSourceCatalog(false);
  });

  function selectSource(sourceKey: AdminVideosSourceKey) {
    setSearchParams(
      (prev) => {
        const next = withAdminVideosSourceKey(prev, sourceKey);
        next.delete("tab");
        next.delete("page");
        return next;
      },
      { replace: true }
    );
  }

  function openBlacklist() {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set("tab", "blacklist");
        if (activeView !== "blacklist") next.delete("page");
        return next;
      },
      { replace: true }
    );
  }

  return (
    <section
      ref={floatingActionPageRef}
      className="admin-page admin-page--with-floating-actions admin-videos-page"
    >
      <VideoSourceNavigation
        crawlers={crawlers}
        hasLocalUploads={hasLocalUploads}
        hasTelegramVideos={hasTelegramVideos}
        sourceCatalogLoaded={sourceCatalogLoaded}
        activeSourceKey={activeSourceKey}
        blacklistActive={activeView === "blacklist"}
        onSelectSource={selectSource}
        onOpenBlacklist={openBlacklist}
      />
      {activeView === "current" && (
        <CurrentVideosTab
          crawlers={crawlers}
          sourceDriveId={activeSourceFilter.driveId}
          sourceCrawlerId={activeSourceFilter.crawlerId}
          sourceKind={activeSourceFilter.sourceKind}
          mediaType={activeMediaType}
          onSelectMediaType={selectMediaType}
          page={page}
          setPage={setPage}
        />
      )}
      {activeView === "blacklist" && (
        <BlacklistTab
          crawlers={crawlers}
          page={page}
          setPage={setPage}
        />
      )}
    </section>
  );
}

function VideoSourceNavigation({
  crawlers,
  hasLocalUploads,
  hasTelegramVideos,
  sourceCatalogLoaded,
  activeSourceKey,
  blacklistActive,
  onSelectSource,
  onOpenBlacklist,
}: {
  crawlers: api.AdminCrawler[];
  hasLocalUploads: boolean;
  hasTelegramVideos: boolean;
  sourceCatalogLoaded: boolean;
  activeSourceKey: AdminVideosSourceKey;
  blacklistActive: boolean;
  onSelectSource: (sourceKey: AdminVideosSourceKey) => void;
  onOpenBlacklist: () => void;
}) {
  const sourceItems: Array<{
    key: AdminVideosSourceKey;
    label: string;
    crawler?: api.AdminCrawler;
    upload?: boolean;
    telegram?: boolean;
    all?: boolean;
  }> = [{ key: "all", label: "全部", all: true }];

  if (sourceCatalogLoaded) {
    sourceItems.push(
      ...(hasLocalUploads
        ? [
            {
              key: `drive:${LOCAL_UPLOAD_SOURCE_ID}` as AdminVideosSourceKey,
              label: "本地存储",
              upload: true,
            },
          ]
        : []),
      ...(hasTelegramVideos
        ? [{ key: "telegram" as const, label: "Telegram", telegram: true }]
        : []),
      ...crawlers.map((crawler) => ({
        key: `crawler:${crawler.id}` as AdminVideosSourceKey,
        label: crawler.name || crawler.id,
        crawler,
      }))
    );
  }

  return (
    <div className="admin-video-source-nav">
      <div className="admin-video-source-tabs" role="group" aria-label="视频来源筛选">
        {sourceItems.map((source) => {
          const selected = !blacklistActive && activeSourceKey === source.key;
          return (
            <button
              key={source.key}
              type="button"
              aria-pressed={selected}
              className={`admin-video-source-tab${selected ? " is-active" : ""}`}
              title={source.label}
              onClick={() => onSelectSource(source.key)}
            >
              <VideoSourceNavigationIcon
                crawler={source.crawler}
                upload={source.upload}
                telegram={source.telegram}
                all={source.all}
              />
              <span className="admin-video-source-tab__label">{source.label}</span>
            </button>
          );
        })}
      </div>
      <button
        type="button"
        className={`admin-video-source-nav__blacklist${blacklistActive ? " is-active" : ""}`}
        aria-pressed={blacklistActive}
        onClick={onOpenBlacklist}
      >
        <Ban size={14} aria-hidden="true" />
        <span>黑名单管理</span>
      </button>
    </div>
  );
}

function VideoSourceNavigationIcon({
  crawler,
  upload,
  telegram,
  all,
}: {
  crawler?: api.AdminCrawler;
  upload?: boolean;
  telegram?: boolean;
  all?: boolean;
}) {
  if (all) {
    return <FilterAllIcon size={15} className="admin-video-source-tab__glyph is-all" />;
  }
  if (upload) {
    return <LocalStorageIcon size={15} className="admin-video-source-tab__glyph is-upload" />;
  }
  if (telegram) {
    return <TelegramIcon size={16} className="admin-video-source-tab__glyph is-telegram" />;
  }
  if (crawler) {
    return <SpiderIcon size={16} className="admin-video-source-tab__glyph is-crawler" />;
  }
  return null;
}

// ---------- 当前视频 ----------

function CurrentVideosTab({
  crawlers,
  sourceDriveId,
  sourceCrawlerId,
  sourceKind,
  mediaType = "all",
  onSelectMediaType,
  page,
  setPage,
}: {
  crawlers: api.AdminCrawler[];
  sourceDriveId: string;
  sourceCrawlerId: string;
  sourceKind: "" | "telegram";
  mediaType: AdminVideosMediaType;
  onSelectMediaType: (type: AdminVideosMediaType) => void;
  page: number;
  setPage: PageSetter;
}) {
  const routeActive = useAdminRouteActive();
  const [list, setList] = useState<api.AdminVideo[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [advancedFiltersOpen, setAdvancedFiltersOpen] = useState(false);
  const [draftFilters, setDraftFilters] = useState<VideoAdvancedFilterValues>(() => ({ ...EMPTY_VIDEO_FILTERS }));
  const [appliedFilters, setAppliedFilters] = useState<VideoAdvancedFilterValues>(() => ({ ...EMPTY_VIDEO_FILTERS }));
  const [total, setTotal] = useState(0);
  const [editing, setEditing] = useState<api.AdminVideo | null>(null);
  const [availableTags, setAvailableTags] = useState<api.AdminTag[]>([]);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [batchTagOpen, setBatchTagOpen] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteSource, setBatchDeleteSource] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<api.AdminVideo | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteSource, setDeleteSource] = useState(false);
  const [regenPreviewById, setRegenPreviewById] = useState<Record<string, RegenPreviewState>>({});
  const [transcodeTarget, setTranscodeTarget] = useState<api.AdminVideo | null>(null);
  const [transcodingAction, setTranscodingAction] = useState(false);
  const [transcodingIds, setTranscodingIds] = useState<Set<string>>(new Set());
  const previousTranscodingIdsRef = useRef<Set<string>>(new Set());
  const [displayedPage, setDisplayedPage] = useState<number | null>(null);
  const [resolvedListQueryKey, setResolvedListQueryKey] = useState("");
  const listRequestIdRef = useRef(0);
  const pageSize = useVideosPageSize(
    DESKTOP_CURRENT_VIDEOS_PAGE_SIZE,
    MOBILE_CURRENT_VIDEOS_PAGE_SIZE
  );
  const previousPageSizeRef = useRef(pageSize);
  const { show } = useToast();
  const { isBrowserPlayable, clearVideoPlaybackUnsupported } = useVideoCompatibility();
  const activeListQueryKey = JSON.stringify([
    page,
    pageSize,
    searchKeyword,
    sourceDriveId,
    sourceCrawlerId,
    sourceKind,
    mediaType,
    appliedFilters,
  ]);
  const activeListQueryKeyRef = useRef(activeListQueryKey);
  activeListQueryKeyRef.current = activeListQueryKey;

  async function refresh() {
    const requestId = ++listRequestIdRef.current;
    const queryKey = activeListQueryKey;
    setLoading(true);
    setLoadError("");
    try {
      const r = await api.listVideos({
        page,
        size: pageSize,
        keyword: searchKeyword,
        driveId: sourceDriveId,
        crawlerId: sourceCrawlerId,
        sourceKind,
        mediaType: mediaType === "all" ? undefined : mediaType,
        ...appliedFilters,
      });
      if (requestId !== listRequestIdRef.current || queryKey !== activeListQueryKeyRef.current) return;
      setList(r.items ?? []);
      setTotal(r.total ?? 0);
      setDisplayedPage(page);
      setResolvedListQueryKey(queryKey);
    } catch (e) {
      if (requestId !== listRequestIdRef.current || queryKey !== activeListQueryKeyRef.current) return;
      const message = e instanceof Error ? e.message : "加载失败";
      setLoadError(message);
      setResolvedListQueryKey(queryKey);
      show(message, "error");
    } finally {
      if (requestId === listRequestIdRef.current && queryKey === activeListQueryKeyRef.current) {
        setLoading(false);
      }
    }
  }

  async function refreshListOnly() {
    const queryKey = activeListQueryKey;
    try {
      const r = await api.listVideos({
        page,
        size: pageSize,
        keyword: searchKeyword,
        driveId: sourceDriveId,
        crawlerId: sourceCrawlerId,
        sourceKind,
        mediaType: mediaType === "all" ? undefined : mediaType,
        ...appliedFilters,
      });
      if (queryKey !== activeListQueryKeyRef.current) return;
      setList(r.items ?? []);
      setTotal(r.total ?? 0);
    } catch {
      // Polling is only used to clear optimistic preview-generation state.
    }
  }

  const trackedRegenCount = Object.keys(regenPreviewById).length;
  const hasGeneratingPreview = list.some((v) => v.previewStatus === REGEN_PREVIEW_STATUS);

  useAdminRouteRevalidation(() => {
    void refreshListOnly();
    void api.listTags().then((tagList) => setAvailableTags(tagList ?? [])).catch(() => undefined);
  });

  useEffect(() => {
    refresh();
  }, [page, searchKeyword, pageSize, sourceDriveId, sourceCrawlerId, sourceKind, mediaType, appliedFilters]);

  useEffect(() => {
    let active = true;
    void api.listTags()
      .then((tagList) => {
        if (active) setAvailableTags(tagList ?? []);
      })
      .catch((e) => {
        if (active) show(e instanceof Error ? e.message : "标签加载失败", "error");
      });
    return () => {
      active = false;
    };
  }, [show]);

  useEffect(() => {
    setSelectedIds(new Set());
  }, [searchKeyword, sourceDriveId, sourceCrawlerId, sourceKind, appliedFilters]);

  useEffect(() => {
    if (previousPageSizeRef.current === pageSize) return;
    previousPageSizeRef.current = pageSize;
    setPage(1);
  }, [pageSize, setPage]);

  useEffect(() => {
    if (!routeActive || (trackedRegenCount === 0 && !hasGeneratingPreview)) return;
    const timer = window.setInterval(() => {
      refreshListOnly();
    }, REGEN_PREVIEW_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [
    trackedRegenCount,
    hasGeneratingPreview,
    page,
    pageSize,
    searchKeyword,
    sourceDriveId,
    sourceCrawlerId,
    sourceKind,
    appliedFilters,
    routeActive,
  ]);

  useEffect(() => {
    if (trackedRegenCount === 0) return;
    const now = Date.now();
    setRegenPreviewById((current) => {
      const next = { ...current };
      let changed = false;
      const byId = new Map(list.map((v) => [v.id, v]));
      for (const [id, state] of Object.entries(current)) {
        const video = byId.get(id);
        const updatedAt = videoUpdatedAtMs(video);
        if (!video || now >= state.expiresAt || updatedAt > state.originalUpdatedAt) {
          delete next[id];
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [list, trackedRegenCount]);

  useEffect(() => {
    if (!routeActive) return;
    let cancelled = false;
    async function checkTranscodeQueue() {
      try {
        const queue = await api.getTranscodeQueue();
        if (cancelled) return;
        const activeSet = new Set<string>();
        if (queue.active && (queue.active.status === "pending" || queue.active.status === "processing")) {
          activeSet.add(queue.active.videoId);
        }
        for (const p of queue.pending) {
          if (p.videoId) activeSet.add(p.videoId);
        }

        const prev = previousTranscodingIdsRef.current;
        for (const id of prev) {
          if (!activeSet.has(id)) {
            const finished = queue.recent?.find((r) => r.videoId === id);
            if (finished) {
              if (finished.status === "completed") {
                clearVideoPlaybackUnsupported(finished.videoId);
                show(`视频「${finished.videoTitle || id}」转码成功！已替换为标准 MP4 并清理原文件。`, "success");
              } else if (finished.status === "failed") {
                show(`视频「${finished.videoTitle || id}」转码失败：${finished.error || "未知原因"}`, "error");
              }
            }
            refreshListOnly();
          }
        }
        previousTranscodingIdsRef.current = activeSet;
        setTranscodingIds(activeSet);
      } catch {
        // queue polling errors are silent
      }
    }

    checkTranscodeQueue();
    const pollInterval = transcodingIds.size > 0 ? 3000 : 10000;
    const timer = window.setInterval(checkTranscodeQueue, pollInterval);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [routeActive, transcodingIds.size, show]);

  const driveNameMap = new Map(crawlers.map((c) => [c.id, c.name || c.id]));
  driveNameMap.set(LOCAL_UPLOAD_SOURCE_ID, "本地存储");

  const listItems = list;
  const editingVideo = editing ? (listItems.find((v) => v.id === editing.id) ?? editing) : null;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const listQueryPending = loading || resolvedListQueryKey !== activeListQueryKey;
  const listQuerySettled = !loading && resolvedListQueryKey === activeListQueryKey;
  const showInitialLoading = displayedPage === null && !loadError && listQueryPending;
  useEffect(() => {
    if (listQuerySettled && page > totalPages) setPage(totalPages);
  }, [listQuerySettled, page, totalPages, setPage]);
  const showPagination = totalPages > 1;
  const activeAdvancedFilterCount = countVideoAdvancedFilters(appliedFilters);
  const hasActiveSearch =
    searchKeyword.trim().length > 0 ||
    !!sourceDriveId ||
    !!sourceCrawlerId ||
    !!sourceKind ||
    activeAdvancedFilterCount > 0;
  const allPageSelected =
    listItems.length > 0 && listItems.every((video) => selectedIds.has(video.id));

  async function handleRegen(v: api.AdminVideo) {
    try {
      await api.regenPreview(v.id);
      trackRegeneratingPreview([v]);
      show("已触发预览视频重生", "success");
    } catch (e) {
      show(e instanceof Error ? e.message : "触发失败", "error");
    }
  }

  async function handleBatchDelete() {
    if (selectedIds.size === 0) return;
    setBatchDeleteSource(false);
    setBatchDeleteOpen(true);
  }

  async function handleConfirmBatchTag(action: "add" | "replace" | "remove", tags: string[]) {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    try {
      const result = await api.batchUpdateVideoTags({
        ids,
        action,
        tags,
      });
      show(`批量设置标签成功，已更新 ${result.updated} 个媒体`, "success");
      setSelectedIds(new Set());
      setBatchTagOpen(false);
      refresh();
      void api.listTags().then((tagList) => setAvailableTags(tagList ?? [])).catch(() => undefined);
    } catch (e) {
      show(e instanceof Error ? e.message : "批量设置标签失败", "error");
      throw e;
    }
  }

  function trackRegeneratingPreview(videos: api.AdminVideo[]) {
    if (videos.length === 0) return;
    const startedAt = Date.now();
    setRegenPreviewById((current) => {
      const next = { ...current };
      for (const v of videos) {
        next[v.id] = {
          expiresAt: startedAt + REGEN_PREVIEW_TRACK_TIMEOUT_MS,
          originalUpdatedAt: videoUpdatedAtMs(v),
        };
      }
      return next;
    });
  }

  function isPreviewGenerating(v: api.AdminVideo) {
    return !!regenPreviewById[v.id] || v.previewStatus === REGEN_PREVIEW_STATUS;
  }

  function isTranscoding(v: api.AdminVideo) {
    return transcodingIds.has(v.id);
  }

  async function handleConfirmTranscode() {
    if (!transcodeTarget) return;
    const target = transcodeTarget;
    setTranscodingAction(true);
    try {
      await api.transcodeVideo(target.id);
      setTranscodingIds((prev) => {
        const next = new Set(prev);
        next.add(target.id);
        previousTranscodingIdsRef.current = next;
        return next;
      });
      setTranscodeTarget(null);
      show("已加入转码队列，将按单任务队列依次执行", "success");
    } catch (e) {
      show(e instanceof Error ? e.message : "提交转码失败", "error");
    } finally {
      setTranscodingAction(false);
    }
  }

  async function confirmDeleteVideo() {
    if (!deleteTarget) return;
    const target = deleteTarget;
    setDeleting(true);
    try {
      const result = await api.deleteVideo(target.id, { deleteSource });
      setDeleteTarget(null);
      setDeleteSource(false);
      setSelectedIds((ids) => {
        const next = new Set(ids);
        next.delete(target.id);
        return next;
      });
      show(result.deletedSource ? "已删除视频，并清理源文件" : "已删除视频", "success");
      if (listItems.length === 1 && page > 1) {
        setPage((p) => Math.max(1, p - 1));
      } else {
        refresh();
      }
    } catch (e) {
      show(e instanceof Error ? e.message : "删除失败", "error");
    } finally {
      setDeleting(false);
    }
  }

  async function confirmBatchDelete() {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    setBatchDeleting(true);
    try {
      let success = 0;
      let deletedSources = 0;
      const deletedIds = new Set<string>();
      for (const id of ids) {
        try {
          const result = await api.deleteVideo(id, { deleteSource: batchDeleteSource });
          success++;
          deletedIds.add(id);
          if (result.deletedSource) deletedSources++;
        } catch {
          // Keep deleting the rest of the selected videos; report aggregate failure below.
        }
      }
      const failed = ids.length - success;
      if (failed === 0) {
        const extra = deletedSources > 0 ? `，其中 ${deletedSources} 个清理了源文件` : "";
        show(`批量删除完成，成功 ${success} 个${extra}`, "success");
      } else {
        show(
          `批量删除完成，成功 ${success} / ${ids.length} 个，失败 ${failed} 个`,
          success > 0 ? "info" : "error"
        );
      }
      setSelectedIds(new Set());
      setBatchDeleteOpen(false);
      setBatchDeleteSource(false);
      const currentPageEmptied =
        listItems.length > 0 && listItems.every((video) => deletedIds.has(video.id));
      if (currentPageEmptied && page > 1) {
        setPage((p) => Math.max(1, p - 1));
      } else {
        refresh();
      }
    } finally {
      setBatchDeleting(false);
    }
  }

  const toggleSelect = (id: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectPageVideos = () => {
    setSelectedIds((current) => {
      const next = new Set(current);
      listItems.forEach((video) => next.add(video.id));
      return next;
    });
  };

  const handleSearch = useCallback((keyword: string) => {
    setSearchKeyword(keyword);
    setPage(1);
  }, [setPage]);

  function openAdvancedFilters() {
    setDraftFilters({ ...appliedFilters });
    setAdvancedFiltersOpen(true);
  }

  function applyAdvancedFilters(e: React.FormEvent) {
    e.preventDefault();
    const today = localDateInputValue(new Date());
    if (
      dateIsAfter(draftFilters.createdFrom, today) ||
      dateIsAfter(draftFilters.createdTo, today)
    ) {
      show("入库时间不能超过当天", "error");
      return;
    }
    if (dateRangeIsReversed(draftFilters.createdFrom, draftFilters.createdTo)) {
      show("入库时间开始日期不能晚于结束日期", "error");
      return;
    }
    if (numberRangeIsReversed(draftFilters.durationMinMinutes, draftFilters.durationMaxMinutes)) {
      show("视频时长最短值不能大于最长值", "error");
      return;
    }
    setAppliedFilters({ ...draftFilters });
    setPage(1);
    setAdvancedFiltersOpen(false);
  }

  function clearAdvancedFilters() {
    setDraftFilters({ ...EMPTY_VIDEO_FILTERS });
  }

  return (
    <div className={`admin-videos-current${selectedIds.size > 0 ? " has-bulk-actions" : ""}`}>
      <div className="admin-page__actions admin-videos-filter admin-videos-filter--current">
        <div className="admin-media-type-switch" role="tablist" aria-label="媒体类型筛选">
          <button
            type="button"
            role="tab"
            aria-selected={mediaType === "all"}
            className={`admin-media-type-btn ${mediaType === "all" ? "is-active" : ""}`}
            onClick={() => onSelectMediaType("all")}
          >
            全部
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mediaType === "video"}
            className={`admin-media-type-btn ${mediaType === "video" ? "is-active" : ""}`}
            onClick={() => onSelectMediaType("video")}
          >
            视频
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mediaType === "image"}
            className={`admin-media-type-btn ${mediaType === "image" ? "is-active" : ""}`}
            onClick={() => onSelectMediaType("image")}
          >
            图片
          </button>
        </div>
        <SearchBox keyword={searchKeyword} onSearch={handleSearch} />
        <div className="admin-videos-filter__current-actions" data-admin-floating-actions>
          <button
            type="button"
            className="admin-btn admin-videos-filter__search-action admin-video-advanced-toggle"
            onClick={openAdvancedFilters}
            aria-haspopup="dialog"
          >
            <SlidersHorizontal size={15} aria-hidden="true" />
            <span>筛选</span>
            {activeAdvancedFilterCount > 0 && (
              <span className="admin-video-advanced-toggle__count" aria-label={`已启用 ${activeAdvancedFilterCount} 项筛选`}>
                {activeAdvancedFilterCount}
              </span>
            )}
          </button>
        </div>
      </div>
      <Modal
        open={advancedFiltersOpen}
        title="筛选"
        onClose={() => setAdvancedFiltersOpen(false)}
        className="admin-modal--video-filters"
        footer={
          <>
            <button type="button" className="admin-btn admin-video-advanced-clear" onClick={clearAdvancedFilters}>
              清空筛选
            </button>
            <div className="admin-video-advanced-modal-actions">
              <button type="button" className="admin-btn" onClick={() => setAdvancedFiltersOpen(false)}>
                取消
              </button>
              <button type="submit" form="admin-video-advanced-filters" className="admin-btn is-primary">
                应用
              </button>
            </div>
          </>
        }
      >
        <AdvancedVideoFilters
          value={draftFilters}
          onChange={setDraftFilters}
          onSubmit={applyAdvancedFilters}
        />
      </Modal>

      {selectedIds.size > 0 && (
        <div className="admin-videos-list-toolbar" data-admin-floating-actions>
          <div className="admin-videos-bulk-actions">
            <span className="admin-videos-bulk-actions__count">已选择 {selectedIds.size} 项</span>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={selectPageVideos}
              disabled={listItems.length === 0 || allPageSelected}
            >
              全选本页
            </button>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={() => setSelectedIds(new Set())}
              disabled={selectedIds.size === 0}
            >
              取消选中
            </button>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={() => setBatchTagOpen(true)}
              disabled={selectedIds.size === 0}
            >
              批量打标签
            </button>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={handleBatchDelete}
              disabled={selectedIds.size === 0}
            >
              批量删除
            </button>
          </div>
        </div>
      )}

      {showInitialLoading ? (
        <VideoCardGridLoadingState />
      ) : loadError ? (
        <ErrorState message={loadError} onRetry={refresh} />
      ) : listItems.length === 0 ? (
        <AdminEmptyVisual
          variant={hasActiveSearch ? "no-results" : "empty"}
          text={hasActiveSearch ? "未查询到" : "当前库中没有媒体文件"}
          className="admin-empty-state admin-empty-state--plain"
        />
      ) : (
        <section
          className={`admin-videos-results${listQueryPending ? " is-page-loading" : ""}`}
          aria-label="媒体列表结果"
          aria-busy={listQueryPending || undefined}
        >
          <div
            className="admin-video-card-grid admin-videos-results__content"
            role="list"
            aria-label="媒体列表"
          >
            {listItems.map((v) => (
              <CurrentVideoCard
                key={v.id}
                video={v}
                driveName={driveNameMap.get(v.driveId) ?? v.driveId}
                selected={selectedIds.has(v.id)}
                transcoding={isTranscoding(v)}
                showTranscode={!isImageExtension(v.ext) && (!isBrowserPlayable(v) || isTranscoding(v))}
                onToggleSelect={() => toggleSelect(v.id)}
                onEdit={() => setEditing(v)}
                onTranscode={() => setTranscodeTarget(v)}
                onDelete={() => {
                  setDeleteSource(false);
                  setDeleteTarget(v);
                }}
              />
            ))}
          </div>
          {showPagination && (
            <AdminPagination
              page={displayedPage ?? page}
              totalPages={totalPages}
              total={total}
              itemLabel="项"
              pending={listQueryPending}
              onPage={setPage}
            />
          )}
        </section>
      )}

      {editingVideo && (
        <EditVideoModal
          video={editingVideo}
          availableTags={availableTags}
          previewGenerating={isPreviewGenerating(editingVideo)}
          onRegenPreview={() => handleRegen(editingVideo)}
          transcoding={isTranscoding(editingVideo)}
          unplayable={!isBrowserPlayable(editingVideo)}
          onTranscode={() => setTranscodeTarget(editingVideo)}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            refresh();
          }}
        />
      )}
      {transcodeTarget && (
        <ConfirmModal
          open
          title="转码为标准 MP4"
          message={`确认将视频「${transcodeTarget.title}」加入转码队列？\n\n转码将转换为浏览器广泛兼容的 MP4 (H.264+AAC) 格式。转码成功后将自动替换并删除不可播放的原文件。队列为单并发（同一时间只处理一个视频）。`}
          confirmText={transcodingAction ? "提交中..." : "开始转码"}
          danger={false}
          loading={transcodingAction}
          onCancel={() => {
            if (!transcodingAction) setTranscodeTarget(null);
          }}
          onConfirm={handleConfirmTranscode}
        />
      )}
      <ConfirmModal
        open={deleteTarget !== null}
        title="删除媒体"
        message={deleteTarget ? `确定要删除「${deleteTarget.title}」吗？` : ""}
        confirmText="确认"
        danger
        hideIcon
        centerMessage
        modalClassName="admin-modal--delete-confirm admin-modal--video-delete-flat"
        loading={deleting}
        onCancel={() => {
          if (!deleting) {
            setDeleteTarget(null);
            setDeleteSource(false);
          }
        }}
        onConfirm={confirmDeleteVideo}
      >
        <DeleteSourceOption checked={deleteSource} disabled={deleting} onChange={setDeleteSource} />
      </ConfirmModal>
      <ConfirmModal
        open={batchDeleteOpen}
        title="批量删除媒体"
        message={`确定要删除已选中的 ${selectedIds.size} 项媒体吗？`}
        confirmText="确认"
        danger
        hideIcon
        centerMessage
        modalClassName="admin-modal--delete-confirm admin-modal--video-delete-flat"
        loading={batchDeleting}
        onCancel={() => {
          if (!batchDeleting) {
            setBatchDeleteOpen(false);
            setBatchDeleteSource(false);
          }
        }}
        onConfirm={confirmBatchDelete}
      >
        <DeleteSourceOption checked={batchDeleteSource} disabled={batchDeleting} onChange={setBatchDeleteSource} />
      </ConfirmModal>
      <BatchTagModal
        open={batchTagOpen}
        selectedCount={selectedIds.size}
        availableTags={availableTags}
        onClose={() => setBatchTagOpen(false)}
        onBatchTag={handleConfirmBatchTag}
      />
    </div>
  );
}

// ---------- 拉黑视频 ----------

function BlacklistTab({
  crawlers,
  page,
  setPage,
}: {
  crawlers: api.AdminCrawler[];
  page: number;
  setPage: PageSetter;
}) {
  const [list, setList] = useState<api.AdminDeletedVideo[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [total, setTotal] = useState(0);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [removeTarget, setRemoveTarget] = useState<api.AdminDeletedVideo | null>(null);
  const [removing, setRemoving] = useState(false);
  const [sourceDeleteStatus, setSourceDeleteStatus] = useState<api.BlacklistSourceDeleteStatus | null>(null);
  const [sourceDeleteOpen, setSourceDeleteOpen] = useState(false);
  const [sourceDeleteTarget, setSourceDeleteTarget] = useState<api.AdminDeletedVideo | null>(null);
  const [batchSourceDeleteOpen, setBatchSourceDeleteOpen] = useState(false);
  const [sourceDeleteStarting, setSourceDeleteStarting] = useState(false);
  const [displayedPage, setDisplayedPage] = useState<number | null>(null);
  const [resolvedListQueryKey, setResolvedListQueryKey] = useState("");
  const listRequestIdRef = useRef(0);
  const pageSize = useVideosPageSize(
    DESKTOP_BLACKLIST_PAGE_SIZE,
    MOBILE_BLACKLIST_PAGE_SIZE
  );
  const previousPageSizeRef = useRef(pageSize);
  const { show } = useToast();
  const activeListQueryKey = JSON.stringify([page, pageSize, searchKeyword]);
  const activeListQueryKeyRef = useRef(activeListQueryKey);
  activeListQueryKeyRef.current = activeListQueryKey;

  async function refresh(silent = false) {
    const requestId = ++listRequestIdRef.current;
    const queryKey = activeListQueryKey;
    if (!silent) {
      setLoading(true);
      setLoadError("");
    }
    try {
      const r = await api.listBlacklist({ page, size: pageSize, keyword: searchKeyword });
      if (requestId !== listRequestIdRef.current || queryKey !== activeListQueryKeyRef.current) return;
      setList(r.items ?? []);
      setTotal(r.total ?? 0);
      setDisplayedPage(page);
      setResolvedListQueryKey(queryKey);
      setLoadError("");
    } catch (e) {
      if (requestId !== listRequestIdRef.current || queryKey !== activeListQueryKeyRef.current) return;
      if (!silent) {
        const message = e instanceof Error ? e.message : "加载失败";
        setLoadError(message);
        setResolvedListQueryKey(queryKey);
        show(message, "error");
      }
    } finally {
      if (
        !silent &&
        requestId === listRequestIdRef.current &&
        queryKey === activeListQueryKeyRef.current
      ) {
        setLoading(false);
      }
    }
  }

  useAdminRouteRevalidation(() => {
    void refresh(true);
  });

  useEffect(() => {
    void refresh();
  }, [page, searchKeyword, pageSize]);

  useEffect(() => {
    let active = true;
    void api.getBlacklistSourceDeleteStatus()
      .then((status) => {
        if (active) setSourceDeleteStatus(status);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!sourceDeleteStatus?.running) return;
    let active = true;
    let timer = 0;

    const poll = async () => {
      try {
        const status = await api.getBlacklistSourceDeleteStatus();
        if (!active) return;
        setSourceDeleteStatus(status);
        if (status.running) {
          timer = window.setTimeout(poll, 2000);
          return;
        }
        const summary = [`成功 ${status.deleted}`];
        if (status.skipped > 0) summary.push(`跳过 ${status.skipped}`);
        if (status.failed > 0) summary.push(`失败 ${status.failed}`);
        show(`源文件删除完成：${summary.join("，")}`, status.failed > 0 ? "info" : "success");
        setSelectedIds(new Set());
        void refresh();
      } catch {
        if (active) timer = window.setTimeout(poll, 2000);
      }
    };

    timer = window.setTimeout(poll, 1000);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [sourceDeleteStatus?.running]);

  useEffect(() => {
    if (previousPageSizeRef.current === pageSize) return;
    previousPageSizeRef.current = pageSize;
    setPage(1);
  }, [pageSize, setPage]);

  useEffect(() => {
    setSelectedIds(new Set());
  }, [searchKeyword]);

  const driveNameMap = new Map(crawlers.map((c) => [c.id, c.name || c.id]));
  driveNameMap.set(LOCAL_UPLOAD_SOURCE_ID, "本地存储");
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const listQueryPending = loading || resolvedListQueryKey !== activeListQueryKey;
  const listQuerySettled = !loading && resolvedListQueryKey === activeListQueryKey;
  const showInitialLoading = displayedPage === null && !loadError && listQueryPending;
  useEffect(() => {
    if (listQuerySettled && page > totalPages) setPage(totalPages);
  }, [listQuerySettled, page, totalPages, setPage]);
  const showPagination = totalPages > 1;
  const sourceDeleteRunning = !!sourceDeleteStatus?.running;
  const hasActiveSearch = searchKeyword.trim().length > 0;
  const hasBlacklistActions = list.length > 0;
  const selectableItems = list.filter(canDeleteBlacklistSource);
  const allPageSelected =
    selectableItems.length > 0 && selectableItems.every((video) => selectedIds.has(video.id));

  async function confirmRemove() {
    if (!removeTarget) return;
    const target = removeTarget;
    setRemoving(true);
    try {
      await api.removeBlacklist(target.id);
      setRemoveTarget(null);
      setSelectedIds((ids) => {
        const next = new Set(ids);
        next.delete(target.id);
        return next;
      });
      show(
        target.restorePolicy === "direct"
          ? "已取消拉黑，视频已恢复到媒体库"
          : target.restorePolicy === "crawler"
            ? "已取消拉黑，将在下次爬虫任务时生效"
            : "已取消拉黑，将在下次手动或定时扫盘时生效",
        "success"
      );
      if (list.length === 1 && page > 1) {
        setPage((p) => Math.max(1, p - 1));
      } else {
        refresh();
      }
    } catch (e) {
      show(e instanceof Error ? e.message : "操作失败", "error");
    } finally {
      setRemoving(false);
    }
  }

  async function startSourceDelete(
    options: { deleteAllSources?: boolean; ids?: string[] },
    onAccepted: () => void,
    startedMessage: string
  ) {
    setSourceDeleteStarting(true);
    try {
      const result = await api.startBlacklistSourceDelete(options);
      setSourceDeleteStatus(result.status);
      if (!result.accepted) {
        show(result.message || "源文件删除任务已在运行", "info");
        return;
      }
      onAccepted();
      show(startedMessage, "info");
    } catch (e) {
      show(e instanceof Error ? e.message : "启动删除任务失败", "error");
    } finally {
      setSourceDeleteStarting(false);
    }
  }

  async function confirmSourceDeleteAll() {
    await startSourceDelete(
      { deleteAllSources: true },
      () => {
        setSourceDeleteOpen(false);
        setSelectedIds(new Set());
      },
      "已开始后台顺序删除全部黑名单源文件"
    );
  }

  async function confirmSourceDeleteTarget() {
    if (!sourceDeleteTarget) return;
    const target = sourceDeleteTarget;
    await startSourceDelete(
      { ids: [target.id] },
      () => {
        setSourceDeleteTarget(null);
        setSelectedIds((ids) => {
          const next = new Set(ids);
          next.delete(target.id);
          return next;
        });
      },
      "已开始后台删除该拉黑视频源文件"
    );
  }

  async function confirmBatchSourceDelete() {
    const ids = [...selectedIds];
    if (ids.length === 0) return;
    await startSourceDelete(
      { ids },
      () => {
        setBatchSourceDeleteOpen(false);
        setSelectedIds(new Set());
      },
      `已开始后台顺序删除 ${ids.length} 个拉黑视频源文件`
    );
  }

  const toggleSelect = (v: api.AdminDeletedVideo) => {
    if (!canDeleteBlacklistSource(v)) return;
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(v.id)) next.delete(v.id);
      else next.add(v.id);
      return next;
    });
  };

  const selectPageBlacklist = () => {
    setSelectedIds((current) => {
      const next = new Set(current);
      selectableItems.forEach((video) => next.add(video.id));
      return next;
    });
  };

  const handleSearch = useCallback((keyword: string) => {
    setSearchKeyword(keyword);
    setPage(1);
  }, [setPage]);

  return (
    <div className={`admin-videos-blacklist${selectedIds.size > 0 ? " has-bulk-actions" : ""}`}>
      <div className="admin-page__actions admin-videos-filter admin-videos-filter--blacklist">
        <SearchBox keyword={searchKeyword} onSearch={handleSearch} />
        {hasBlacklistActions && (
          <div
            className="admin-videos-filter__actions admin-blacklist-source-delete"
            data-admin-floating-actions
          >
            {sourceDeleteStatus?.running && (
              <span className="admin-blacklist-source-delete__status">
                正在删除 {sourceDeleteStatus.processed}/{sourceDeleteStatus.total}
              </span>
            )}
            <button
              type="button"
              className="admin-btn admin-videos-filter__batch admin-videos-filter__search-action admin-blacklist-source-delete__button"
              disabled={sourceDeleteStatus?.running || (sourceDeleteStatus?.pending ?? total) <= 0}
              onClick={() => setSourceDeleteOpen(true)}
            >
              <Trash2 size={15} aria-hidden="true" />
              {sourceDeleteStatus?.running ? "删除中" : "删除全部"}
            </button>
          </div>
        )}
      </div>

      {selectedIds.size > 0 && (
        <div
          className="admin-videos-list-toolbar"
          data-admin-floating-actions
        >
          <div className="admin-videos-bulk-actions">
            <span className="admin-videos-bulk-actions__count">已选择 {selectedIds.size} 项</span>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={selectPageBlacklist}
              disabled={sourceDeleteRunning || selectableItems.length === 0 || allPageSelected}
            >
              全选本页
            </button>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={() => setSelectedIds(new Set())}
              disabled={selectedIds.size === 0}
            >
              取消选中
            </button>
            <button
              type="button"
              className="admin-btn admin-videos-bulk-actions__btn"
              onClick={() => setBatchSourceDeleteOpen(true)}
              disabled={sourceDeleteRunning || selectedIds.size === 0}
            >
              批量删除
            </button>
          </div>
        </div>
      )}

      {showInitialLoading ? null : loadError ? (
        <ErrorState message={loadError} onRetry={refresh} />
      ) : list.length === 0 ? (
        <AdminEmptyVisual
          variant={hasActiveSearch ? "no-results" : "empty"}
          text={hasActiveSearch ? "未查询到" : "暂无拉黑视频"}
          className="admin-empty-state admin-empty-state--plain"
        />
      ) : (
        <section
          className={`admin-videos-results${listQueryPending ? " is-page-loading" : ""}`}
          aria-label="拉黑视频列表结果"
          aria-busy={listQueryPending || undefined}
        >
          <table className="admin-table admin-table--static-rows admin-blacklist-table admin-videos-results__content">
            <tbody>
              {list.map((v) => {
                const sourceDeletable = canDeleteBlacklistSource(v);
                const isSelected = selectedIds.has(v.id);
                const selectionDisabled = !sourceDeletable || sourceDeleteRunning;
                const selectionLabel = v.fileName || v.id;
                const sourceName = driveNameMap.get(v.driveId) ?? v.driveId;

                return (
                <tr key={v.id}>
                  <td className="admin-blacklist-select-cell">
                    <label
                      className={`admin-video-card__select admin-blacklist-row-select${selectionDisabled ? " is-disabled" : ""}`}
                      title={selectionDisabled ? "当前不可选择" : isSelected ? "取消选择" : "选择视频"}
                    >
                      <input
                        className="admin-video-card__select-input"
                        type="checkbox"
                        checked={isSelected}
                        disabled={selectionDisabled}
                        onChange={() => toggleSelect(v)}
                        aria-label={isSelected ? `取消选择「${selectionLabel}」` : `选择「${selectionLabel}」`}
                      />
                      <span
                        className={`admin-video-card__select-box${isSelected ? " is-checked" : ""}`}
                        aria-hidden="true"
                      >
                        {isSelected && <Check size={12} />}
                      </span>
                    </label>
                  </td>
                  <td data-label="文件名">
                    <div className="admin-blacklist-filecell">
                      <span className="admin-blacklist-filename" title={v.fileName || undefined}>{v.fileName || <span className="admin-text-faint">（无文件名）</span>}</span>
                      {v.reason === "duplicate" && <span className="admin-blacklist-reason-pill">重复文件</span>}
                    </div>
                  </td>
                  <td data-label="来源" className="admin-mono-cell admin-blacklist-source-cell">
                    <span className="admin-blacklist-source-name" title={sourceName}>{sourceName}</span>
                  </td>
                  <td className="is-actions" data-label="操作">
                    <div className="admin-blacklist-actions">
                      {v.restorePolicy !== "none" ? (
                        <button
                          type="button"
                          className="admin-btn"
                          onClick={() => setRemoveTarget(v)}
                          disabled={sourceDeleteRunning}
                          title={sourceDeleteRunning ? "源文件删除任务运行中" : "取消拉黑"}
                        >
                          取消拉黑
                        </button>
                      ) : v.reason === "duplicate" ? (
                        v.canonicalVideoId && v.canonicalTitle ? (
                          <Link
                            className="admin-btn"
                            to={`/video/${encodeURIComponent(v.canonicalVideoId)}`}
                            title={v.canonicalTitle}
                          >
                            查看保留视频
                          </Link>
                        ) : null
                      ) : (
                        <span className="admin-blacklist-unavailable">
                          不可恢复
                        </span>
                      )}
                      {sourceDeletable && (
                        <VideoDeleteIconButton
                          onClick={() => setSourceDeleteTarget(v)}
                          disabled={sourceDeleteRunning}
                          title="删除"
                          ariaLabel={`删除 ${v.fileName || v.id}`}
                        />
                      )}
                    </div>
                  </td>
                </tr>
                );
              })}
            </tbody>
          </table>
          {showPagination && (
            <AdminPagination
              page={displayedPage ?? page}
              totalPages={totalPages}
              total={total}
              itemLabel="视频"
              pending={listQueryPending}
              onPage={setPage}
            />
          )}
        </section>
      )}

      <ConfirmModal
        open={sourceDeleteOpen}
        title="删除全部黑名单源文件"
        message={`确定删除全部待处理的黑名单源文件吗？当前共 ${sourceDeleteStatus?.pending ?? total} 个。`}
        confirmText="确认"
        danger
        hideIcon
        centerMessage
        modalClassName="admin-modal--delete-confirm admin-modal--source-delete-flat"
        loading={sourceDeleteStarting}
        onCancel={() => {
          if (!sourceDeleteStarting) setSourceDeleteOpen(false);
        }}
        onConfirm={confirmSourceDeleteAll}
      />

      <ConfirmModal
        open={sourceDeleteTarget !== null}
        title="删除源文件"
        message={sourceDeleteTarget ? `确定删除「${sourceDeleteTarget.fileName || sourceDeleteTarget.id}」的源文件吗？` : ""}
        confirmText="确认"
        danger
        hideIcon
        centerMessage
        modalClassName="admin-modal--delete-confirm admin-modal--source-delete-flat"
        loading={sourceDeleteStarting}
        onCancel={() => {
          if (!sourceDeleteStarting) setSourceDeleteTarget(null);
        }}
        onConfirm={confirmSourceDeleteTarget}
      />

      <ConfirmModal
        open={batchSourceDeleteOpen}
        title="批量删除拉黑视频源文件"
        message={`确定删除已选中的 ${selectedIds.size} 个拉黑视频源文件吗？`}
        confirmText="确认"
        danger
        hideIcon
        centerMessage
        modalClassName="admin-modal--delete-confirm admin-modal--source-delete-flat"
        loading={sourceDeleteStarting}
        onCancel={() => {
          if (!sourceDeleteStarting) setBatchSourceDeleteOpen(false);
        }}
        onConfirm={confirmBatchSourceDelete}
      />

      <ConfirmModal
        open={removeTarget !== null}
        title="取消拉黑"
        message={
          removeTarget
            ? `确定取消拉黑「${removeTarget.fileName || removeTarget.id}」吗？`
            : ""
        }
        confirmText="确认"
        centerMessage
        loading={removing}
        onCancel={() => {
          if (!removing) setRemoveTarget(null);
        }}
        onConfirm={confirmRemove}
      />
    </div>
  );
}

// ---------- 共享小组件 ----------

function AdvancedVideoFilters({
  value,
  onChange,
  onSubmit,
}: {
  value: VideoAdvancedFilterValues;
  onChange: (value: VideoAdvancedFilterValues) => void;
  onSubmit: (event: React.FormEvent) => void;
}) {
  function updateField(key: keyof VideoAdvancedFilterValues, nextValue: string) {
    onChange({ ...value, [key]: nextValue });
  }

  const today = localDateInputValue(new Date());

  return (
    <form
      id="admin-video-advanced-filters"
      className="admin-video-advanced-filters"
      aria-label="视频高级筛选"
      onSubmit={onSubmit}
    >
      <div className="admin-video-advanced-filters__grid">
        <fieldset className="admin-video-advanced-range">
          <legend>入库时间</legend>
          <div className="admin-video-advanced-range__inputs is-date-range">
            <label>
              {!value.createdFrom && (
                <span className="admin-video-advanced-range__placeholder" aria-hidden="true">
                  年/月/日
                </span>
              )}
              <input
                type="date"
                className={!value.createdFrom ? "is-empty" : undefined}
                aria-label="入库开始日期"
                value={value.createdFrom}
                max={earlierDateInputValue(value.createdTo, today)}
                onClick={(event) => openNativeDatePicker(event.currentTarget)}
                onChange={(event) => updateField("createdFrom", event.target.value)}
              />
            </label>
            <span className="admin-video-advanced-range__separator">至</span>
            <label>
              {!value.createdTo && (
                <span className="admin-video-advanced-range__placeholder" aria-hidden="true">
                  年/月/日
                </span>
              )}
              <input
                type="date"
                className={!value.createdTo ? "is-empty" : undefined}
                aria-label="入库截止日期"
                value={value.createdTo}
                min={value.createdFrom || undefined}
                max={today}
                onClick={(event) => openNativeDatePicker(event.currentTarget)}
                onChange={(event) => updateField("createdTo", event.target.value)}
              />
            </label>
          </div>
        </fieldset>

        <fieldset className="admin-video-advanced-range">
          <legend>视频时长(分钟)</legend>
          <div className="admin-video-advanced-range__inputs is-duration-range">
            <label>
              <input
                type="number"
                aria-label="视频最短时长（分钟）"
                min={1}
                max={525600}
                step={1}
                inputMode="numeric"
                placeholder="不限"
                value={value.durationMinMinutes}
                onChange={(event) => updateField("durationMinMinutes", event.target.value)}
              />
            </label>
            <span className="admin-video-advanced-range__separator">至</span>
            <label>
              <input
                type="number"
                aria-label="视频最长时长（分钟）"
                min={value.durationMinMinutes || 1}
                max={525600}
                step={1}
                inputMode="numeric"
                placeholder="不限"
                value={value.durationMaxMinutes}
                onChange={(event) => updateField("durationMaxMinutes", event.target.value)}
              />
            </label>
          </div>
        </fieldset>
      </div>
    </form>
  );
}

function SearchBox({
  keyword,
  onSearch,
}: {
  keyword: string;
  onSearch: (keyword: string) => void;
}) {
  return (
    <SearchPanel
      className="admin-videos-filter__search search-panel--transparent"
      value={keyword}
      onSearch={onSearch}
      variant="uiverse"
      placeholder=""
    />
  );
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="admin-error-state">
      <strong>加载失败</strong>
      <span>{message}</span>
      <button type="button" className="admin-btn" onClick={onRetry}>
        <RefreshCw size={13} /> 重试
      </button>
    </div>
  );
}

function VideoCardGridLoadingState() {
  return (
    <div
      className="admin-video-card-grid admin-video-card-grid--skeleton"
      role="status"
      aria-label="视频加载中"
      aria-busy="true"
    >
      {Array.from({ length: CURRENT_VIDEO_SKELETON_CARD_COUNT }, (_, index) => (
        <div
          key={index}
          className="admin-video-card-skeleton admin-card-skeleton-surface"
          aria-hidden="true"
        />
      ))}
    </div>
  );
}

function canDeleteBlacklistSource(v: api.AdminDeletedVideo) {
  return !v.sourceDeleted;
}

function DeleteSourceOption({
  checked,
  disabled,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="admin-delete-source-option">
      <input type="checkbox" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span>
        <strong>同时删除视频源文件</strong>
      </span>
    </label>
  );
}

function VideoDeleteIconButton({
  onClick,
  title,
  ariaLabel,
  disabled = false,
}: {
  onClick: () => void;
  title: string;
  ariaLabel: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      className="admin-btn admin-video-action-icon-button is-danger"
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-label={ariaLabel}
    >
      <Trash2 size={15} aria-hidden="true" />
    </button>
  );
}

function isImageExtension(ext?: string): boolean {
  const clean = (ext || "").toLowerCase().replace(/^\./, "").trim();
  return ["jpg", "jpeg", "png", "webp", "gif", "bmp"].includes(clean);
}

function CurrentVideoCard({
  video,
  driveName,
  selected,
  transcoding = false,
  showTranscode = false,
  onToggleSelect,
  onEdit,
  onTranscode,
  onDelete,
}: {
  video: api.AdminVideo;
  driveName: string;
  selected: boolean;
  transcoding?: boolean;
  showTranscode?: boolean;
  onToggleSelect: () => void;
  onEdit: () => void;
  onTranscode: () => void;
  onDelete: () => void;
}) {
  const isImage = isImageExtension(video.ext);

  return (
    <article className="admin-video-card" role="listitem">
      <div className="admin-video-card__content">
        <VideoTitleCell video={video} />
      </div>

      <dl className="admin-video-card__meta">
        <div className="admin-video-card__meta-item">
          <dt>来源</dt>
          <dd className="admin-video-card__source" title={driveName}>
            {driveName}
          </dd>
        </div>
        <div className="admin-video-card__meta-item is-duration">
          <dt>{isImage ? "类型" : "时长"}</dt>
          <dd>{isImage ? "图片" : formatDur(video.durationSeconds)}</dd>
        </div>
      </dl>

      <footer className="admin-video-card__actions">
        <div className="admin-video-card__utility-actions">
          <button
            type="button"
            className="admin-btn admin-video-action-icon-button"
            onClick={onEdit}
            title={isImage ? "编辑媒体" : "编辑视频"}
            aria-label={isImage ? "编辑媒体" : "编辑视频"}
          >
            <Edit size={15} />
          </button>
          {showTranscode && (
            <button
              type="button"
              className={`admin-btn admin-video-action-icon-button${transcoding ? " is-busy" : ""}`}
              onClick={onTranscode}
              disabled={transcoding}
              title={transcoding ? "转码处理中（队列单任务进行）..." : "转码为标准 MP4（H.264+AAC）"}
              aria-label="转码为标准 MP4"
            >
              {transcoding ? (
                <RefreshCw size={15} className="admin-spin" />
              ) : (
                <Film size={15} />
              )}
            </button>
          )}
          <VideoDeleteIconButton
            onClick={onDelete}
            title={isImage ? "删除媒体" : "删除视频"}
            ariaLabel={isImage ? "删除媒体" : "删除视频"}
          />
        </div>
        <label className="admin-video-card__select" title={selected ? "取消选择" : "选择"}>
          <input
            className="admin-video-card__select-input"
            type="checkbox"
            checked={selected}
            onChange={onToggleSelect}
            aria-label={selected ? `取消选择「${video.title}」` : `选择「${video.title}」`}
          />
          <span
            className={`admin-video-card__select-box${selected ? " is-checked" : ""}`}
            aria-hidden="true"
          >
            {selected && <Check size={12} />}
          </span>
        </label>
      </footer>
    </article>
  );
}

function VideoTitleCell({ video: v }: { video: api.AdminVideo }) {
  return (
    <div className="admin-video-title-cell">
      <div className="admin-video-thumb-wrap" aria-hidden="true">
        {v.thumbnailUrl ? (
          <img className="admin-video-thumb" src={v.thumbnailUrl} alt="" loading="lazy" decoding="async" />
        ) : (
          <div className="admin-video-thumb-placeholder">
            <Image size={14} />
          </div>
        )}
      </div>
      <div className="admin-video-title-body">
        <div className="admin-video-title" title={v.title}>{v.title}</div>
        {fileMeta(v) && <div className="admin-video-filemeta">{fileMeta(v)}</div>}
        {(v.tags ?? []).length > 0 && (
          <div className="admin-pills admin-video-title-tags">
            {(v.tags ?? []).map((t) => (
              <span
                key={t}
                className="admin-pill admin-video-tag-source"
                data-source={v.tagSources?.[t] ?? "unknown"}
                title={tagAssignmentTitle(v, t)}
              >
                <span>{t}</span>
                {v.tagSources?.[t] && (
                  <small>{tagAssignmentSourceLabel(v.tagSources[t])}</small>
                )}
              </span>
            ))}
          </div>
        )}
        <VideoFileMetaPills video={v} />
      </div>
    </div>
  );
}

function PreviewStatus({ s }: { s: string }) {
  if (s === REGEN_PREVIEW_STATUS) return <span className="admin-status is-generating">生成中</span>;
  if (s === "ready") return <span className="admin-status is-ok">就绪</span>;
  if (s === "failed") return <span className="admin-status is-error">失败</span>;
  if (s === "disabled") return <span className="admin-status">已关闭</span>;
  if (s === "skipped") return <span className="admin-status">跳过</span>;
  return <span className="admin-status is-pending">待生成</span>;
}

function VideoFileMetaPills({ video }: { video: api.AdminVideo }) {
  const parts = fileMetaParts(video);
  if (parts.length === 0) return null;

  return (
    <div className="admin-video-filemeta-pills" aria-label="视频文件信息">
      {parts.map((part, index) => (
        <span key={`${part}-${index}`} className="admin-video-filemeta-pill">
          {part}
        </span>
      ))}
    </div>
  );
}

function formatDur(sec: number): string {
  if (!sec) return "—";
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
}

function videoUpdatedAtMs(video?: api.AdminVideo): number {
  if (!video?.updatedAt) return 0;
  const value = Date.parse(video.updatedAt);
  return Number.isFinite(value) ? value : 0;
}

function useVideosPageSize(desktopPageSize: number, mobilePageSize: number) {
  const [pageSize, setPageSize] = useState(() =>
    window.matchMedia(VIDEOS_MOBILE_QUERY).matches ? mobilePageSize : desktopPageSize
  );

  useEffect(() => {
    const media = window.matchMedia(VIDEOS_MOBILE_QUERY);
    const update = () => {
      setPageSize(media.matches ? mobilePageSize : desktopPageSize);
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [desktopPageSize, mobilePageSize]);

  return pageSize;
}

function BatchTagModal({
  open,
  selectedCount,
  availableTags,
  onClose,
  onBatchTag,
}: {
  open: boolean;
  selectedCount: number;
  availableTags: api.AdminTag[];
  onClose: () => void;
  onBatchTag: (action: "add" | "replace" | "remove", tags: string[]) => Promise<void>;
}) {
  const [action, setAction] = useState<"add" | "replace" | "remove">("add");
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [customTagInput, setCustomTagInput] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const { show } = useToast();

  const handleAddCustomTag = () => {
    const raw = customTagInput.trim();
    if (!raw) return;
    const parts = raw
      .split(/[,，\s]+/)
      .map((s) => s.trim().replace(/^#/, ""))
      .filter(Boolean);
    if (parts.length === 0) return;
    setSelectedTags((prev) => Array.from(new Set([...prev, ...parts])));
    setCustomTagInput("");
  };

  const handleToggleTag = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const handleSubmit = async () => {
    if (selectedTags.length === 0 && action !== "replace") {
      show("请选择或输入至少一个标签", "error");
      return;
    }
    setSubmitting(true);
    try {
      await onBatchTag(action, selectedTags);
      onClose();
    } catch {
      // Toast already shown in parent
    } finally {
      setSubmitting(false);
    }
  };

  if (!open) return null;

  return (
    <Modal
      open
      title={`批量设置标签 (已选择 ${selectedCount} 项)`}
      ariaLabel="批量设置标签"
      className="admin-modal--video-edit"
      onClose={onClose}
      footer={
        <div style={{ display: "flex", justifyContent: "flex-end", gap: "8px", width: "100%" }}>
          <button type="button" className="admin-btn" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button
            type="button"
            className="admin-btn is-primary"
            onClick={handleSubmit}
            disabled={submitting}
          >
            {submitting ? "处理中..." : "确认应用"}
          </button>
        </div>
      }
    >
      <div className="admin-form" style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
        <div>
          <label style={{ display: "block", marginBottom: "8px", fontWeight: 600, fontSize: "14px" }}>
            操作模式
          </label>
          <div style={{ display: "flex", gap: "14px", flexWrap: "wrap" }}>
            <label className="admin-check" style={{ cursor: "pointer", display: "inline-flex", alignItems: "center", gap: "6px" }}>
              <input
                type="radio"
                name="batch-tag-action"
                checked={action === "add"}
                onChange={() => setAction("add")}
              />
              <span>追加标签 (推荐，保留已有标签)</span>
            </label>
            <label className="admin-check" style={{ cursor: "pointer", display: "inline-flex", alignItems: "center", gap: "6px" }}>
              <input
                type="radio"
                name="batch-tag-action"
                checked={action === "replace"}
                onChange={() => setAction("replace")}
              />
              <span>覆盖标签 (完全替换)</span>
            </label>
            <label className="admin-check" style={{ cursor: "pointer", display: "inline-flex", alignItems: "center", gap: "6px" }}>
              <input
                type="radio"
                name="batch-tag-action"
                checked={action === "remove"}
                onChange={() => setAction("remove")}
              />
              <span>移除标签 (从已有标签中删除)</span>
            </label>
          </div>
        </div>

        <div>
          <label style={{ display: "block", marginBottom: "8px", fontWeight: 600, fontSize: "14px" }}>
            已选标签 ({selectedTags.length})
          </label>
          <div
            style={{
              minHeight: "44px",
              padding: "8px 12px",
              background: "rgba(0, 0, 0, 0.25)",
              border: "1px solid rgba(255, 255, 255, 0.1)",
              borderRadius: "8px",
              display: "flex",
              flexWrap: "wrap",
              gap: "8px",
              alignItems: "center",
            }}
          >
            {selectedTags.length === 0 ? (
              <span style={{ fontSize: "13px", color: "rgba(255, 255, 255, 0.4)" }}>
                {action === "replace"
                  ? "未选任何标签（确认应用后将清空所选媒体的全部标签）"
                  : `请在下方输入或点击勾选要${action === "remove" ? "移除" : "添加"}的标签`}
              </span>
            ) : (
              selectedTags.map((tag) => (
                <span
                  key={tag}
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "6px",
                    background:
                      action === "remove"
                        ? "rgba(239, 68, 68, 0.2)"
                        : "rgba(56, 189, 248, 0.2)",
                    border: `1px solid ${
                      action === "remove"
                        ? "rgba(239, 68, 68, 0.4)"
                        : "rgba(56, 189, 248, 0.4)"
                    }`,
                    color: action === "remove" ? "#fca5a5" : "#38bdf8",
                    padding: "3px 8px",
                    borderRadius: "6px",
                    fontSize: "12px",
                    fontWeight: 600,
                  }}
                >
                  #{tag}
                  <button
                    type="button"
                    onClick={() => handleToggleTag(tag)}
                    style={{
                      background: "transparent",
                      border: "none",
                      color: "inherit",
                      cursor: "pointer",
                      padding: "0 2px",
                      fontSize: "14px",
                      lineHeight: 1,
                    }}
                    title="移除"
                  >
                    ×
                  </button>
                </span>
              ))
            )}
          </div>
        </div>

        <div>
          <label style={{ display: "block", marginBottom: "6px", fontSize: "13px", color: "rgba(255, 255, 255, 0.7)" }}>
            快捷输入新标签 (回车或点击添加)
          </label>
          <div style={{ display: "flex", gap: "8px" }}>
            <input
              type="text"
              placeholder="输入标签名，如: 空心柚七、街拍（多个可用逗号隔开）"
              value={customTagInput}
              onChange={(e) => setCustomTagInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  handleAddCustomTag();
                }
              }}
              style={{
                flex: 1,
                padding: "8px 12px",
                background: "rgba(255, 255, 255, 0.05)",
                border: "1px solid rgba(255, 255, 255, 0.15)",
                borderRadius: "6px",
                color: "#fff",
                fontSize: "13px",
              }}
            />
            <button
              type="button"
              className="admin-btn"
              onClick={handleAddCustomTag}
              disabled={!customTagInput.trim()}
              style={{ whiteSpace: "nowrap" }}
            >
              添加标签
            </button>
          </div>
        </div>

        {availableTags.length > 0 && (
          <div>
            <label style={{ display: "block", marginBottom: "6px", fontSize: "13px", color: "rgba(255, 255, 255, 0.7)" }}>
              从现有标签池快速选择
            </label>
            <div
              style={{
                maxHeight: "180px",
                overflowY: "auto",
                display: "flex",
                flexWrap: "wrap",
                gap: "8px",
                padding: "8px",
                background: "rgba(0, 0, 0, 0.15)",
                border: "1px solid rgba(255, 255, 255, 0.08)",
                borderRadius: "6px",
              }}
            >
              {availableTags.map((tag) => {
                const checked = selectedTags.includes(tag.label);
                return (
                  <button
                    type="button"
                    key={tag.id}
                    onClick={() => handleToggleTag(tag.label)}
                    style={{
                      padding: "4px 10px",
                      borderRadius: "6px",
                      border: checked
                        ? "1px solid #38bdf8"
                        : "1px solid rgba(255, 255, 255, 0.12)",
                      background: checked
                        ? "rgba(56, 189, 248, 0.2)"
                        : "rgba(255, 255, 255, 0.04)",
                      color: checked ? "#38bdf8" : "rgba(255, 255, 255, 0.8)",
                      fontSize: "12px",
                      cursor: "pointer",
                      transition: "all 0.15s ease",
                    }}
                  >
                    #{tag.label}
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </Modal>
  );
}

function EditVideoModal({
  video,
  availableTags,
  previewGenerating,
  onRegenPreview,
  transcoding = false,
  unplayable = false,
  onTranscode,
  onClose,
  onSaved,
}: {
  video: api.AdminVideo;
  availableTags: api.AdminTag[];
  previewGenerating: boolean;
  onRegenPreview: () => Promise<void>;
  transcoding?: boolean;
  unplayable?: boolean;
  onTranscode?: () => void;
  onClose: () => void;
  onSaved: () => void;
}) {
  const idPrefix = useId();
  const [selectedTags, setSelectedTags] = useState(video.tags ?? []);
  const [saving, setSaving] = useState(false);
  const [regeningPreview, setRegeningPreview] = useState(false);
  const { show } = useToast();

  useEffect(() => {
    const ext = (video.ext || "").toLowerCase().replace(/^\./, "");
    if (ext !== "mp4" || unplayable) return;
    const testVideo = document.createElement("video");
    testVideo.preload = "metadata";
    testVideo.src =
      video.driveId === "local-upload"
        ? `/p/upload/${encodeURIComponent(video.id)}`
        : `/p/stream/${encodeURIComponent(video.driveId)}/${encodeURIComponent(video.fileId)}`;
    const handleError = () => {
      if (testVideo.error && (testVideo.error.code === 3 || testVideo.error.code === 4)) {
        markVideoPlaybackUnsupported(video.id);
      }
    };
    testVideo.addEventListener("error", handleError);
    return () => {
      testVideo.removeEventListener("error", handleError);
      testVideo.src = "";
    };
  }, [video, unplayable]);

  async function handleSave() {
    setSaving(true);
    try {
      await api.updateVideo(video.id, {
        tags: selectedTags,
      });
      show("已保存", "success");
      onSaved();
    } catch (e) {
      show(e instanceof Error ? e.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  }

  async function handleRegenPreview() {
    setRegeningPreview(true);
    try {
      await onRegenPreview();
    } finally {
      setRegeningPreview(false);
    }
  }

  const previewBusy = previewGenerating || regeningPreview;

  return (
    <Modal
      open
      title="编辑媒体"
      ariaLabel="编辑媒体"
      className="admin-modal--video-edit"
      onClose={onClose}
      footer={
        <>
          <Link
            className="admin-btn admin-video-edit-view-link"
            to={`/video/${encodeURIComponent(video.id)}`}
            target="_blank"
            rel="noreferrer"
          >
            查看媒体详情
          </Link>
          <div className="admin-video-edit-footer-actions">
            <button type="button" className="admin-btn" onClick={onClose}>
              取消
            </button>
            <button type="button" className="admin-btn is-primary" onClick={handleSave} disabled={saving}>
              {saving ? "保存中..." : "保存"}
            </button>
          </div>
        </>
      }
    >
      <div className="admin-form admin-video-edit-form">
        <section className="admin-video-edit-section">
          <h3>基本信息</h3>
          <div className="admin-video-edit-basics">
            <div className="admin-form__row">
              <label htmlFor={`${idPrefix}-video-title`}>标题</label>
              <input id={`${idPrefix}-video-title`} value={video.title} readOnly aria-readonly="true" />
            </div>
            <div className="admin-form__row">
              <label htmlFor={`${idPrefix}-video-author`}>作者</label>
              <input id={`${idPrefix}-video-author`} value={video.author ?? ""} readOnly aria-readonly="true" />
            </div>
          </div>
        </section>

        <section className="admin-video-edit-section">
          <h3>标签</h3>
          <div className="admin-tag-picker admin-video-tag-picker">
            {availableTags.map((tag) => (
              <label key={tag.id} className="admin-check admin-video-tag-option">
                <input
                  type="checkbox"
                  checked={selectedTags.includes(tag.label)}
                  onChange={() => setSelectedTags(toggleTag(selectedTags, tag.label))}
                />
                <span className="admin-video-tag-option__label" title={tag.label}>{tag.label}</span>
              </label>
            ))}
          </div>
        </section>

        <section className="admin-video-edit-section">
          <h3>媒体信息</h3>
          <dl className="admin-video-edit-meta">
            <div className="admin-video-edit-meta__item">
              <dt>来源盘</dt>
              <dd>{video.driveId}</dd>
            </div>
            <div className="admin-video-edit-meta__item">
              <dt>文件信息</dt>
              <dd>{fileMeta(video) || "—"}</dd>
            </div>
            <div className="admin-video-edit-meta__item is-preview">
              <dt>预览视频</dt>
              <dd className="admin-video-preview-actions">
                <PreviewStatus s={previewGenerating ? REGEN_PREVIEW_STATUS : video.previewStatus} />
                <button
                  type="button"
                  className="admin-btn admin-video-preview-button"
                  onClick={handleRegenPreview}
                  disabled={saving || previewBusy}
                >
                  {previewBusy ? "生成中..." : "重新生成预览"}
                </button>
              </dd>
            </div>
            <div className="admin-video-edit-meta__item is-transcode">
              <dt>视频转码</dt>
              <dd className="admin-video-transcode-actions">
                <span
                  className={`admin-status ${
                    transcoding
                      ? "is-generating"
                      : unplayable
                      ? "is-warning"
                      : "is-ok"
                  }`}
                >
                  {transcoding
                    ? "转码中 / 排队中"
                    : unplayable
                    ? `${video.ext ? video.ext.toUpperCase() : "未知格式"} (不支持播放)`
                    : "MP4 (正常可播)"}
                </span>
                <button
                  type="button"
                  className="admin-btn admin-video-transcode-button"
                  onClick={onTranscode}
                  disabled={saving || transcoding}
                >
                  {transcoding
                    ? "转码处理中..."
                    : unplayable
                    ? "转码为 MP4"
                    : "重新转码为 MP4"}
                </button>
              </dd>
            </div>
          </dl>
        </section>
      </div>
    </Modal>
  );
}

function tagAssignmentSourceLabel(source: string): string {
  if (source === "manual") return "人工";
  if (source === "auto") return "自动";
  if (source === "crawler") return "爬虫";
  if (source === "telegram") return "Telegram";
  if (source === "legacy") return "自动生成";
  return source || "未知";
}

function tagAssignmentTitle(video: api.AdminVideo, label: string): string {
  const source = video.tagSources?.[label];
  const evidence = video.tagEvidence?.[label];
  return [source ? `来源：${tagAssignmentSourceLabel(source)}` : "", evidence ? `依据：${evidence}` : ""]
    .filter(Boolean)
    .join("；");
}

function fileMeta(v: api.AdminVideo): string {
  return fileMetaParts(v).join(" · ");
}

function fileMetaParts(v: api.AdminVideo): string[] {
  return [normalizeExt(v.ext), v.size > 0 ? formatBytes(v.size) : ""].filter(Boolean);
}

function normalizeExt(ext: string): string {
  const value = (ext ?? "").replace(/^\./, "").trim();
  return value ? value.toUpperCase() : "";
}

function countVideoAdvancedFilters(value: VideoAdvancedFilterValues): number {
  let count = 0;
  if (value.createdFrom || value.createdTo) count++;
  if (value.durationMinMinutes || value.durationMaxMinutes) count++;
  return count;
}

function dateRangeIsReversed(from: string, to: string): boolean {
  return !!from && !!to && from > to;
}

function dateIsAfter(value: string, maximum: string): boolean {
  return !!value && value > maximum;
}

function earlierDateInputValue(value: string, maximum: string): string {
  return value && value < maximum ? value : maximum;
}

function localDateInputValue(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function numberRangeIsReversed(min: string, max: string): boolean {
  return !!min && !!max && Number(min) > Number(max);
}

function openNativeDatePicker(input: HTMLInputElement) {
  if (typeof input.showPicker !== "function") return;
  try {
    input.showPicker();
  } catch {
    // Some browsers expose showPicker but restrict when it may be called.
  }
}

function toggleTag(tags: string[], label: string): string[] {
  return tags.includes(label) ? tags.filter((tag) => tag !== label) : [...tags, label];
}
