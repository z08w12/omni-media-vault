import { memo } from "react";
import { NavLink, useLocation } from "react-router";
import { Film, Images, LogOut, PlaySquare, Shield, UploadCloud, User } from "lucide-react";
import { useAuth } from "@/admin/AuthContext";
import "@/styles/mobile-tabbar.css";

type MobileTabBarProps = {
  hidden?: boolean;
};

export const MobileTabBar = memo(function MobileTabBar({ hidden = false }: MobileTabBarProps) {
  const location = useLocation();
  const { status, isAdmin, logout } = useAuth();

  const isShorts = location.pathname.startsWith("/shorts");
  const shouldHide = hidden || (isShorts && false);

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // ignore
    }
  };

  return (
    <nav
      className={`mobile-tab-bar ${shouldHide ? "mobile-tab-bar--hidden" : ""}`}
      aria-label="移动端底部导航"
    >
      <NavLink
        to="/list"
        className={({ isActive }) =>
          `mobile-tab-item ${
            isActive || location.pathname === "/" ? "mobile-tab-item--active" : ""
          }`
        }
      >
        <div className="mobile-tab-item__icon-wrap">
          <Film size={20} strokeWidth={2} />
        </div>
        <span className="mobile-tab-item__label">视频</span>
      </NavLink>

      <NavLink
        to="/shorts"
        className={({ isActive }) =>
          `mobile-tab-item ${isActive ? "mobile-tab-item--active" : ""}`
        }
      >
        <div className="mobile-tab-item__icon-wrap">
          <PlaySquare size={20} strokeWidth={2} />
        </div>
        <span className="mobile-tab-item__label">短视频</span>
      </NavLink>

      <NavLink
        to="/photos"
        className={({ isActive }) =>
          `mobile-tab-item ${isActive ? "mobile-tab-item--active" : ""}`
        }
      >
        <div className="mobile-tab-item__icon-wrap">
          <Images size={20} strokeWidth={2} />
        </div>
        <span className="mobile-tab-item__label">图集</span>
      </NavLink>

      {isAdmin ? (
        <>
          <NavLink
            to="/upload"
            className={({ isActive }) =>
              `mobile-tab-item ${isActive ? "mobile-tab-item--active" : ""}`
            }
          >
            <div className="mobile-tab-item__icon-wrap">
              <UploadCloud size={20} strokeWidth={2} />
            </div>
            <span className="mobile-tab-item__label">上传</span>
          </NavLink>

          <NavLink
            to="/admin"
            className={({ isActive }) =>
              `mobile-tab-item ${isActive ? "mobile-tab-item--active" : ""}`
            }
          >
            <div className="mobile-tab-item__icon-wrap">
              <Shield size={20} strokeWidth={2} />
            </div>
            <span className="mobile-tab-item__label">管理</span>
          </NavLink>
        </>
      ) : status === "authed" ? (
        <button
          type="button"
          className="mobile-tab-item"
          onClick={handleLogout}
        >
          <div className="mobile-tab-item__icon-wrap">
            <LogOut size={20} strokeWidth={2} />
          </div>
          <span className="mobile-tab-item__label">退出</span>
        </button>
      ) : (
        <NavLink
          to="/login"
          className={({ isActive }) =>
            `mobile-tab-item ${isActive ? "mobile-tab-item--active" : ""}`
          }
        >
          <div className="mobile-tab-item__icon-wrap">
            <User size={20} strokeWidth={2} />
          </div>
          <span className="mobile-tab-item__label">登录</span>
        </NavLink>
      )}
    </nav>
  );
});
