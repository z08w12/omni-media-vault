import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { useLocation, useNavigate, useNavigationType } from "react-router";
import {
  consumePrefetchedVideoDetail,
  consumePrefetchedVideoRecommendations,
  deleteVideo,
  fetchTags,
  fetchVideoCollectionSummary,
  fetchVideoDetail,
  fetchVideoRecommendations,
  fetchVideoSubtitles,
  readCachedTags,
  recordView,
  updateVideoTags,
} from "@/data/videos";
import { useAuth } from "@/admin/AuthContext";
import { useDocumentScrollLock } from "@/lib/useDocumentScrollLock";
import { resolveVideoReturnPath } from "@/lib/videoReturnPath";
import { readVideoListingBackground } from "@/lib/videoListingBackground";
import { navigationHistory } from "@/lib/navigationHistory";
import {
  filterDeletedVideos,
  getDeletedVideoIDs,
  isVideoDeleted,
  subscribeVideoDeletions,
} from "@/lib/videoDeletions";
import { scrollPageTo, usePageScrollRoot } from "@/lib/pageScroll";
import type {
  TagItem,
  VideoCollectionSummary,
  VideoDetail,
  VideoItem,
} from "@/types";
import type { VideoReactionCounts } from "@/lib/videoReaction";

const DETAIL_CACHE_LIMIT = 20;
const RECOMMENDATIONS_CACHE_LIMIT = 80;
const COLLECTION_SUMMARY_CACHE_LIMIT = 20;

type VideoDetailSnapshot = {
  detail: VideoDetail;
};

const cachedVideoDetailsByID = new Map<string, VideoDetailSnapshot>();
const cachedRecommendationsByID = new Map<string, VideoItem[]>();
const cachedCollectionSummariesByID = new Map<
  string,
  VideoCollectionSummary | null
>();

subscribeVideoDeletions((id) => {
  cachedVideoDetailsByID.delete(id);
  cachedRecommendationsByID.delete(id);
  cachedCollectionSummariesByID.delete(id);
  for (const [key, videos] of cachedRecommendationsByID) {
    cachedRecommendationsByID.set(key, filterDeletedVideos(videos));
  }
  cachedCollectionSummariesByID.clear();
});

function readCachedVideoDetail(id: string): VideoDetailSnapshot | null {
  if (isVideoDeleted(id)) return null;
  return cachedVideoDetailsByID.get(id) ?? null;
}

function rememberVideoDetail(snapshot: VideoDetailSnapshot) {
  const id = snapshot.detail.id;
  if (isVideoDeleted(id)) return;
  cachedVideoDetailsByID.delete(id);
  cachedVideoDetailsByID.set(id, snapshot);

  if (cachedVideoDetailsByID.size > DETAIL_CACHE_LIMIT) {
    const oldestID = cachedVideoDetailsByID.keys().next().value;
    if (oldestID) cachedVideoDetailsByID.delete(oldestID);
  }
}

function rememberCollectionSummary(
  id: string,
  summary: VideoCollectionSummary | null
) {
  cachedCollectionSummariesByID.delete(id);
  cachedCollectionSummariesByID.set(id, summary);
  if (cachedCollectionSummariesByID.size > COLLECTION_SUMMARY_CACHE_LIMIT) {
    const oldestID = cachedCollectionSummariesByID.keys().next().value;
    if (oldestID) cachedCollectionSummariesByID.delete(oldestID);
  }
}

function readCachedRecommendations(id: string): VideoItem[] | null {
  return cachedRecommendationsByID.get(id) ?? null;
}

function rememberRecommendations(id: string, videos: VideoItem[]) {
  if (cachedRecommendationsByID.has(id)) return;
  if (cachedRecommendationsByID.size >= RECOMMENDATIONS_CACHE_LIMIT) {
    const oldestID = cachedRecommendationsByID.keys().next().value;
    if (oldestID) cachedRecommendationsByID.delete(oldestID);
  }
  cachedRecommendationsByID.set(id, filterDeletedVideos(videos));
}

export type RelatedImageItem = {
  id?: string;
  src: string;
  title: string;
  poster?: string;
};

export function useVideoDetailCore(id?: string) {
  const deletedVideoIDs = useSyncExternalStore(
    subscribeVideoDeletions,
    getDeletedVideoIDs,
    getDeletedVideoIDs
  );
  const scrollRootRef = usePageScrollRoot();
  const navigate = useNavigate();
  const location = useLocation();
  const navigationType = useNavigationType();
  const [entryNavigationType] = useState(navigationType);
  const { isAdmin } = useAuth();
  const locationState = location.state as { from?: unknown } | null;

  const [initialSnapshot] = useState<VideoDetailSnapshot | null>(() =>
    id ? readCachedVideoDetail(id) : null
  );
  const [detail, setDetail] = useState<VideoDetail | null>(
    initialSnapshot?.detail ?? null
  );
  const [initialRecommendations] = useState<VideoItem[] | null>(() =>
    id ? readCachedRecommendations(id) : null
  );
  const [recommendations, setRecommendations] = useState<VideoItem[]>(
    initialRecommendations ?? []
  );

  const visibleRecommendations = useMemo(
    () => filterDeletedVideos(recommendations, deletedVideoIDs),
    [recommendations, deletedVideoIDs]
  );

  const relatedImages = useMemo<RelatedImageItem[]>(() => {
    return visibleRecommendations
      .filter(
        (v) =>
          v.duration === "图片" ||
          Boolean(v.thumbnail && /\.(jpe?g|png|webp|gif|bmp)(\?.*)?$/i.test(v.thumbnail))
      )
      .map((v) => ({
        id: v.id,
        src: `/api/video/${v.id}/source`,
        title: v.title,
        poster: v.thumbnail,
      }));
  }, [visibleRecommendations]);

  const [recommendationsLoading, setRecommendationsLoading] = useState(
    !!id && initialRecommendations === null
  );
  const [recommendationsError, setRecommendationsError] = useState("");
  const [recommendationsLoadVersion, setRecommendationsLoadVersion] = useState(0);

  const [availableTags, setAvailableTags] = useState<TagItem[]>(
    () => readCachedTags() ?? []
  );
  const [availableTagsLoading, setAvailableTagsLoading] = useState(false);
  const [availableTagsError, setAvailableTagsError] = useState("");
  const [availableTagsLoadVersion, setAvailableTagsLoadVersion] = useState(0);

  const [collectionSummary, setCollectionSummary] =
    useState<VideoCollectionSummary | null>(() =>
      id ? (cachedCollectionSummariesByID.get(id) ?? null) : null
    );

  const [loading, setLoading] = useState(initialSnapshot === null);
  const [detailError, setDetailError] = useState("");
  const [detailLoadVersion, setDetailLoadVersion] = useState(0);

  const [tagSaving, setTagSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteSource, setDeleteSource] = useState(false);
  const [deleteSaving, setDeleteSaving] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const reactionCountsRef = useRef<
    (VideoReactionCounts & { videoId: string }) | null
  >(null);

  useDocumentScrollLock(deleteOpen && isAdmin);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      setDetailError("");
      document.title = "视频不存在";
      return;
    }
    let active = true;
    if (entryNavigationType !== "POP") {
      scrollPageTo(scrollRootRef, { top: 0, behavior: "auto" });
    }
    if (initialSnapshot) {
      rememberVideoDetail(initialSnapshot);
      document.title = initialSnapshot.detail.title;
    }

    if (!initialSnapshot) setLoading(true);
    setDetailError("");

    const prefetchedDetail =
      detailLoadVersion === 0 ? consumePrefetchedVideoDetail(id) : null;
    const detailRequest = prefetchedDetail ?? fetchVideoDetail(id);

    detailRequest
      .then((d) => {
        if (!active) return;
        let stableDetail = isVideoDeleted(id) ? null : d;
        const localReactionCounts = reactionCountsRef.current;
        if (stableDetail && localReactionCounts?.videoId === stableDetail.id) {
          stableDetail = {
            ...stableDetail,
            likes: localReactionCounts.likes,
            dislikes: localReactionCounts.dislikes,
          };
        }

        if (stableDetail) {
          rememberVideoDetail({ detail: stableDetail });
        } else {
          cachedVideoDetailsByID.delete(id);
          cachedRecommendationsByID.delete(id);
        }
        setDetail(stableDetail);
        setDetailError("");
        setLoading(false);
        document.title = stableDetail ? stableDetail.title : "视频不存在";
      })
      .catch(() => {
        if (!active) return;
        setDetailError("视频信息暂时无法加载，请稍后重试");
        setLoading(false);
        if (!initialSnapshot) document.title = "视频加载失败";
      });
    return () => {
      active = false;
    };
  }, [
    detailLoadVersion,
    entryNavigationType,
    id,
    initialSnapshot,
    scrollRootRef,
  ]);

  useEffect(() => {
    if (!id || !detail) return;
    if (
      detail.transcodeStatus !== "pending" &&
      detail.transcodeStatus !== "processing"
    )
      return;

    const timer = window.setInterval(() => {
      fetchVideoDetail(id)
        .then((latest) => {
          if (latest) {
            rememberVideoDetail({ detail: latest });
            setDetail(latest);
          }
        })
        .catch(() => {
          // ignore transient poll error
        });
    }, 3000);

    return () => {
      window.clearInterval(timer);
    };
  }, [id, detail?.transcodeStatus]);

  useEffect(() => {
    if (!id) {
      setRecommendations([]);
      setRecommendationsLoading(false);
      setRecommendationsError("");
      return;
    }
    if (initialRecommendations !== null && recommendationsLoadVersion === 0) {
      consumePrefetchedVideoRecommendations(id);
      setRecommendationsLoading(false);
      setRecommendationsError("");
      return;
    }

    const controller = new AbortController();
    let active = true;
    const prefetchedRecommendations =
      recommendationsLoadVersion === 0
        ? consumePrefetchedVideoRecommendations(id)
        : null;
    const recommendationsRequest =
      prefetchedRecommendations ??
      fetchVideoRecommendations(id, { signal: controller.signal });

    setRecommendationsLoading(true);
    setRecommendationsError("");
    recommendationsRequest
      .then((videos) => {
        if (!active) return;
        rememberRecommendations(id, videos);
        setRecommendations(cachedRecommendationsByID.get(id) ?? videos);
      })
      .catch((error: unknown) => {
        if (
          active &&
          !(error instanceof DOMException && error.name === "AbortError")
        ) {
          setRecommendationsError("推荐视频加载失败，请稍后重试");
        }
      })
      .finally(() => {
        if (active) setRecommendationsLoading(false);
      });

    return () => {
      active = false;
      if (!prefetchedRecommendations) controller.abort();
    };
  }, [id, initialRecommendations, recommendationsLoadVersion]);

  useEffect(() => {
    if (!isAdmin) {
      setAvailableTagsLoading(false);
      setAvailableTagsError("");
      return;
    }

    const cached = readCachedTags();
    if (cached !== null) {
      setAvailableTags(cached);
      setAvailableTagsLoading(false);
      setAvailableTagsError("");
      return;
    }

    let active = true;
    setAvailableTagsLoading(true);
    setAvailableTagsError("");
    fetchTags()
      .then((tagList) => {
        if (active) setAvailableTags(tagList);
      })
      .catch(() => {
        if (active) setAvailableTagsError("标签加载失败，请稍后重试");
      })
      .finally(() => {
        if (active) setAvailableTagsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [availableTagsLoadVersion, isAdmin]);

  useEffect(() => {
    if (!id || deletedVideoIDs.has(id) || !detail?.collectionCandidate) {
      if (id) cachedCollectionSummariesByID.delete(id);
      setCollectionSummary(null);
      return;
    }

    const controller = new AbortController();
    let active = true;
    fetchVideoCollectionSummary(id, { signal: controller.signal })
      .then((summary) => {
        if (!active) return;
        rememberCollectionSummary(id, summary);
        setCollectionSummary(summary);
      })
      .catch((error: unknown) => {
        if (
          active &&
          !(error instanceof DOMException && error.name === "AbortError")
        ) {
          setCollectionSummary(
            cachedCollectionSummariesByID.get(id) ?? null
          );
        }
      });
    return () => {
      active = false;
      controller.abort();
    };
  }, [detail?.collectionCandidate, id, deletedVideoIDs]);

  const handleTagsChange = useCallback(
    async (nextTags: string[]) => {
      if (!detail) return;
      setTagSaving(true);
      try {
        const updated = await updateVideoTags(detail.id, nextTags);
        const nextDetail = { ...detail, tags: updated.tags ?? [] };
        setDetail(nextDetail);
        rememberVideoDetail({ detail: nextDetail });
      } finally {
        setTagSaving(false);
      }
    },
    [detail]
  );

  const handleOpenDelete = useCallback(() => {
    if (!isAdmin || !detail || deleteSaving) return;
    setDeleteSource(false);
    setDeleteError("");
    setDeleteOpen(true);
  }, [isAdmin, detail, deleteSaving]);

  const handleCloseDelete = useCallback(() => {
    if (deleteSaving) return;
    setDeleteOpen(false);
    setDeleteError("");
  }, [deleteSaving]);

  const handleConfirmDelete = useCallback(async () => {
    if (!detail || deleteSaving) return;
    setDeleteSaving(true);
    setDeleteError("");
    try {
      await deleteVideo(detail.id, { deleteSource });
      if (!navigationHistory.isCurrent(location.key)) return;
      const background = readVideoListingBackground(location.state);
      const delta = background
        ? navigationHistory.backDelta(location.key, background.key)
        : null;
      if (delta !== null) {
        navigate(delta);
        return;
      }
      const from = typeof locationState?.from === "string" ? locationState.from : null;
      navigate(resolveVideoReturnPath(from), { replace: true });
    } catch {
      setDeleteError(
        deleteSource
          ? "删除失败，源文件未能删除，请检查WebDAV是否有删除权限"
          : "删除失败，请稍后重试。"
      );
      setDeleteSaving(false);
    }
  }, [detail, deleteSaving, deleteSource, location, locationState, navigate]);

  const handleFirstPlay = useCallback(() => {
    if (!detail) return;
    recordView(detail.id).catch(() => undefined);
  }, [detail]);

  const handleReactionCountsChange = useCallback(
    (counts: VideoReactionCounts) => {
      if (id) {
        reactionCountsRef.current = { videoId: id, ...counts };
      }
      setDetail((current) => {
        if (!current) return current;
        const nextDetail = {
          ...current,
          likes: counts.likes,
          dislikes: counts.dislikes,
        };
        rememberVideoDetail({ detail: nextDetail });
        return nextDetail;
      });
    },
    [id]
  );

  const loadSubtitles = useCallback(() => {
    if (!id) return Promise.resolve([]);
    return fetchVideoSubtitles(id);
  }, [id]);

  const retryDetail = useCallback(() => {
    setDetailLoadVersion((v) => v + 1);
  }, []);

  const retryRecommendations = useCallback(() => {
    setRecommendationsLoadVersion((v) => v + 1);
  }, []);

  const retryAvailableTags = useCallback(() => {
    setAvailableTagsLoadVersion((v) => v + 1);
  }, []);

  const isPhotoMedia = Boolean(
    detail &&
      (detail.duration === "图片" ||
        (detail.mediaType ? detail.mediaType.startsWith("image/") : false) ||
        Boolean(
          detail.videoSrc &&
            /\.(jpe?g|png|webp|gif|bmp)(\?.*)?$/i.test(detail.videoSrc)
        ))
  );

  const isPikPakMp4 = isPikPakMp4Detail(detail);

  return {
    id,
    detail,
    loading,
    detailError,
    retryDetail,
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
  };
}

function isPikPakMp4Detail(detail: VideoDetail | null) {
  return Boolean(
    detail &&
      detail.mediaType?.toLowerCase() === "video/mp4" &&
      detail.sourceLabel?.toLowerCase().includes("pikpak") === true
  );
}
