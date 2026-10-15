import { useDevice } from "@/platform/DeviceContext";

export const MOBILE_VIDEO_PAGE_SIZE = 14;

export function useIsMobile(): boolean {
  return useDevice().isMobile;
}
