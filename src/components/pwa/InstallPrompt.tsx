import { useState, useEffect, memo } from "react";
import { Download, Share, X } from "lucide-react";
import { useDevice } from "@/platform";
import "@/styles/pwa-install.css";

const DISMISSED_STORAGE_KEY = "omv_pwa_install_dismissed_at";
const DISMISS_DURATION_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

export const InstallPrompt = memo(function InstallPrompt() {
  const { isMobile } = useDevice();
  const [deferredPrompt, setDeferredPrompt] = useState<any>(null);
  const [visible, setVisible] = useState(false);
  const [isIOS, setIsIOS] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;

    // Check if already running in standalone PWA mode
    const isStandalone =
      window.matchMedia("(display-mode: standalone)").matches ||
      // @ts-expect-error legacy iOS Safari navigator.standalone
      Boolean(window.navigator.standalone);

    if (isStandalone) {
      return;
    }

    // Check if recently dismissed by user
    try {
      const dismissedAt = localStorage.getItem(DISMISSED_STORAGE_KEY);
      if (dismissedAt && Date.now() - Number(dismissedAt) < DISMISS_DURATION_MS) {
        return;
      }
    } catch {
      // ignore localStorage errors
    }

    // Detect iOS
    const userAgent = window.navigator.userAgent.toLowerCase();
    const isIOSDevice =
      /iphone|ipad|ipod/.test(userAgent) &&
      !(window as any).MSStream;

    if (isIOSDevice) {
      setIsIOS(true);
      // Wait a few seconds after initial load before gently showing iOS A2HS guidance
      const timer = setTimeout(() => {
        setVisible(true);
      }, 3000);
      return () => clearTimeout(timer);
    }

    // Android / Chrome / Edge beforeinstallprompt listener
    const handleBeforeInstallPrompt = (e: Event) => {
      e.preventDefault();
      setDeferredPrompt(e);
      setVisible(true);
    };

    window.addEventListener("beforeinstallprompt", handleBeforeInstallPrompt);

    return () => {
      window.removeEventListener("beforeinstallprompt", handleBeforeInstallPrompt);
    };
  }, []);

  const handleInstallClick = async () => {
    if (!deferredPrompt) return;
    try {
      deferredPrompt.prompt();
      const choiceResult = await deferredPrompt.userChoice;
      if (choiceResult.outcome === "accepted") {
        setVisible(false);
      }
    } catch (err) {
      console.warn("[PWA] Installation prompt failed:", err);
    } finally {
      setDeferredPrompt(null);
    }
  };

  const handleDismiss = () => {
    setVisible(false);
    try {
      localStorage.setItem(DISMISSED_STORAGE_KEY, String(Date.now()));
    } catch {
      // ignore
    }
  };

  if (!visible || !isMobile) {
    return null;
  }

  return (
    <aside
      className="pwa-install-banner"
      role="dialog"
      aria-label="添加到主屏幕以获得更好体验"
    >
      <div className="pwa-install-inner">
        <div className="pwa-install-header">
          <img
            src="/app-icon-192.png"
            alt="OmniVault App Icon"
            className="pwa-install-icon"
          />
          <div className="pwa-install-text">
            <h4 className="pwa-install-title">安装 OmniVault 应用</h4>
            <p className="pwa-install-desc">
              添加到手机主屏幕，享受全屏沉浸播放与原生 App 体验
            </p>
          </div>
          <button
            type="button"
            className="pwa-install-close"
            onClick={handleDismiss}
            aria-label="关闭提示"
          >
            <X size={18} />
          </button>
        </div>

        {isIOS ? (
          <div className="pwa-ios-instructions">
            <span>轻触 Safari 底部的</span>
            <Share size={14} style={{ display: "inline-block", verticalAlign: "middle" }} />
            <span>【分享】按钮，然后选择【添加到主屏幕】即可</span>
          </div>
        ) : (
          <div className="pwa-install-actions">
            <button
              type="button"
              className="pwa-install-btn pwa-install-btn--dismiss"
              onClick={handleDismiss}
            >
              稍后再说
            </button>
            <button
              type="button"
              className="pwa-install-btn pwa-install-btn--primary"
              onClick={handleInstallClick}
            >
              <Download size={13} style={{ display: "inline-block", marginRight: 4 }} />
              添加到主屏幕
            </button>
          </div>
        )}
      </div>
    </aside>
  );
});
