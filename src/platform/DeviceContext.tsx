import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type DeviceMode = "desktop" | "mobile";
export type DeviceOverride = "auto" | "desktop" | "mobile";

export type DeviceContextValue = {
  device: DeviceMode;
  isMobile: boolean;
  isDesktop: boolean;
  override: DeviceOverride;
  setOverride: (mode: DeviceOverride) => void;
  hasTouch: boolean;
};

const MOBILE_BREAKPOINT_QUERY = "(max-width: 768px)";
const STORAGE_KEY = "omv_device_override";

const DeviceContext = createContext<DeviceContextValue | null>(null);

function detectIsMobileScreen(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia(MOBILE_BREAKPOINT_QUERY).matches;
}

function detectHasTouch(): boolean {
  if (typeof window === "undefined") return false;
  return (
    "ontouchstart" in window ||
    navigator.maxTouchPoints > 0 ||
    // @ts-expect-error legacy msMaxTouchPoints
    Boolean(navigator.msMaxTouchPoints)
  );
}

export function DeviceProvider({ children }: { children: ReactNode }) {
  const [override, setOverrideState] = useState<DeviceOverride>(() => {
    if (typeof window === "undefined") return "auto";
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved === "desktop" || saved === "mobile" || saved === "auto") {
        return saved;
      }
    } catch {
      // ignore
    }
    return "auto";
  });

  const [isMobileScreen, setIsMobileScreen] = useState<boolean>(detectIsMobileScreen);
  const [hasTouch] = useState<boolean>(detectHasTouch);

  useEffect(() => {
    const media = window.matchMedia(MOBILE_BREAKPOINT_QUERY);
    const handler = (e: MediaQueryListEvent) => {
      setIsMobileScreen(e.matches);
    };

    media.addEventListener("change", handler);
    return () => {
      media.removeEventListener("change", handler);
    };
  }, []);

  const setOverride = useCallback((mode: DeviceOverride) => {
    setOverrideState(mode);
    try {
      localStorage.setItem(STORAGE_KEY, mode);
    } catch {
      // ignore
    }
  }, []);

  const device: DeviceMode = useMemo(() => {
    if (override === "desktop") return "desktop";
    if (override === "mobile") return "mobile";
    return isMobileScreen ? "mobile" : "desktop";
  }, [override, isMobileScreen]);

  const value: DeviceContextValue = useMemo(
    () => ({
      device,
      isMobile: device === "mobile",
      isDesktop: device === "desktop",
      override,
      setOverride,
      hasTouch,
    }),
    [device, override, setOverride, hasTouch]
  );

  return (
    <DeviceContext.Provider value={value}>
      {children}
    </DeviceContext.Provider>
  );
}

export function useDevice(): DeviceContextValue {
  const context = useContext(DeviceContext);
  if (!context) {
    // Graceful fallback if used outside DeviceProvider
    const isMobile = detectIsMobileScreen();
    return {
      device: isMobile ? "mobile" : "desktop",
      isMobile,
      isDesktop: !isMobile,
      override: "auto",
      setOverride: () => {},
      hasTouch: detectHasTouch(),
    };
  }
  return context;
}
