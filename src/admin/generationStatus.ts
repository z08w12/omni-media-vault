export type ScanOutcome = "succeeded" | "partial" | "failed" | "canceled" | "skipped";

export const scanOutcomeLabels: Record<ScanOutcome, string> = {
  succeeded: "已完成",
  partial: "部分完成",
  failed: "失败",
  canceled: "已取消",
  skipped: "已跳过",
};

export const scanOutcomeClasses: Record<ScanOutcome, string> = {
  succeeded: "idle",
  partial: "cooling",
  failed: "error",
  canceled: "queued",
  skipped: "queued",
};

export function generationStateLabel(state: string): string {
  if (Object.prototype.hasOwnProperty.call(scanOutcomeLabels, state)) {
    return scanOutcomeLabels[state as ScanOutcome];
  }
  if (state === "scanning") return "扫盘中";
  if (state === "generating") return "生成中";
  if (state === "cooling") return "冷却中";
  if (state === "queued") return "排队中";
  return "空闲";
}

export function generationStateClass(state: string): string {
  if (Object.prototype.hasOwnProperty.call(scanOutcomeClasses, state)) {
    return scanOutcomeClasses[state as ScanOutcome];
  }
  if (state === "scanning" || state === "generating" || state === "cooling" || state === "queued") {
    if (state === "scanning") return "generating";
    return state;
  }
  return "idle";
}
