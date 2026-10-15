import { useCallback, useRef } from "react";
import { previewController } from "@/lib/previewController";

export type PreviewIntentInput = {
  previewEnabled: boolean;
  pointerType?: string;
  canHover: boolean;
  previewActive: boolean;
};

export const HOVER_DELAY_MS = 300;
export const TOUCH_PREVIEW_DELAY_MS = 200;

export function shouldInterceptPreviewTap(input: PreviewIntentInput): boolean {
  return (
    input.previewEnabled &&
    isTouchLike(input.pointerType, input.canHover) &&
    !input.previewActive
  );
}

export function shouldStartInstantPreview(input: {
  pointerType?: string;
}): boolean {
  return input.pointerType === "touch";
}

function isTouchLike(pointerType: string | undefined, canHover: boolean): boolean {
  if (pointerType === "mouse") return false;
  if (pointerType === "touch") return true;
  return !canHover;
}

export type PreviewState = "idle" | "intent" | "loading" | "playing" | "error";

export function usePreviewIntent({
  videoId,
  previewSrc,
  inView,
  isImage = false,
  setPreviewState,
  onActivatePreview,
}: {
  videoId: string;
  previewSrc?: string;
  inView: boolean;
  isImage?: boolean;
  setPreviewState: (state: PreviewState) => void;
  onActivatePreview: () => void;
}) {
  const previewIntentTimerRef = useRef<number | null>(null);
  const touchPreviewArmedRef = useRef(false);

  function startTouchPreviewIntent() {
    if (!previewController.isEnabled() || !previewSrc) return;
    clearPreviewIntentTimer();
    touchPreviewArmedRef.current = true;
    previewController.setActiveId(videoId);
    setPreviewState("intent");
    previewIntentTimerRef.current = window.setTimeout(() => {
      previewIntentTimerRef.current = null;
      if (
        !previewController.isEnabled() ||
        !touchPreviewArmedRef.current ||
        previewController.getActiveId() !== videoId
      ) {
        return;
      }
      startPreviewNow({ requireInView: false });
    }, TOUCH_PREVIEW_DELAY_MS);
  }

  function clearPreviewIntentTimer() {
    if (previewIntentTimerRef.current === null) return;
    window.clearTimeout(previewIntentTimerRef.current);
    previewIntentTimerRef.current = null;
  }

  const startPreviewNow = useCallback(
    (options: { requireInView: boolean }) => {
      if (isImage || !previewController.isEnabled() || !previewSrc) return;
      if (options.requireInView && !inView) return;
      clearPreviewIntentTimer();
      previewController.setActiveId(videoId);
      onActivatePreview();
      setPreviewState("loading");
    },
    [inView, isImage, onActivatePreview, previewSrc, setPreviewState, videoId]
  );

  const startPreviewIntent = useCallback(() => {
    if (!previewController.isEnabled() || !previewSrc) return;
    if (!inView) return;
    if (previewIntentTimerRef.current) return;
    setPreviewState("intent");

    previewIntentTimerRef.current = window.setTimeout(() => {
      previewIntentTimerRef.current = null;
      startPreviewNow({ requireInView: true });
    }, HOVER_DELAY_MS);
  }, [inView, previewSrc, setPreviewState, startPreviewNow]);

  return {
    touchPreviewArmedRef,
    clearPreviewIntentTimer,
    startPreviewNow,
    startPreviewIntent,
    startTouchPreviewIntent,
  };
}
