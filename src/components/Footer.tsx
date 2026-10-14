import { useDevice } from "@/platform";
import { Monitor, Smartphone, Sparkles } from "lucide-react";

export function Footer() {
  const { override, setOverride } = useDevice();

  return (
    <footer className="footer">
      <div className="container footer__inner" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "12px" }}>
        <a
          className="footer__copy"
          href="https://github.com/z08w12/omni-media-vault"
          target="_blank"
          rel="noopener noreferrer"
        >
          © {new Date().getFullYear()} 91
        </a>

        {/* 视口/设备模式手动切换器 */}
        <div className="footer__device-switcher" style={{ display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", opacity: 0.8 }}>
          <span style={{ color: "rgba(255, 255, 255, 0.45)" }}>视图模式:</span>
          <button
            type="button"
            onClick={() => setOverride("auto")}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "4px",
              padding: "2px 8px",
              borderRadius: "4px",
              background: override === "auto" ? "rgba(56, 189, 248, 0.2)" : "transparent",
              color: override === "auto" ? "#38bdf8" : "inherit",
              border: "1px solid",
              borderColor: override === "auto" ? "rgba(56, 189, 248, 0.4)" : "rgba(255, 255, 255, 0.1)",
              cursor: "pointer",
            }}
            title="根据屏幕和设备能力自动识别"
          >
            <Sparkles size={12} />
            自动
          </button>
          <button
            type="button"
            onClick={() => setOverride("desktop")}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "4px",
              padding: "2px 8px",
              borderRadius: "4px",
              background: override === "desktop" ? "rgba(56, 189, 248, 0.2)" : "transparent",
              color: override === "desktop" ? "#38bdf8" : "inherit",
              border: "1px solid",
              borderColor: override === "desktop" ? "rgba(56, 189, 248, 0.4)" : "rgba(255, 255, 255, 0.1)",
              cursor: "pointer",
            }}
            title="强制使用电脑桌面大屏布局"
          >
            <Monitor size={12} />
            电脑版
          </button>
          <button
            type="button"
            onClick={() => setOverride("mobile")}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: "4px",
              padding: "2px 8px",
              borderRadius: "4px",
              background: override === "mobile" ? "rgba(56, 189, 248, 0.2)" : "transparent",
              color: override === "mobile" ? "#38bdf8" : "inherit",
              border: "1px solid",
              borderColor: override === "mobile" ? "rgba(56, 189, 248, 0.4)" : "rgba(255, 255, 255, 0.1)",
              cursor: "pointer",
            }}
            title="强制使用手机移动端触控布局"
          >
            <Smartphone size={12} />
            手机版
          </button>
        </div>
      </div>
    </footer>
  );
}
