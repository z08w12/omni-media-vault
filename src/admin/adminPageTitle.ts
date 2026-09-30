export const ADMIN_PAGE_TITLES = [
  { path: "/admin/crawlers", title: "爬虫管理" },
  { path: "/admin/videos", title: "媒体管理" },
  { path: "/admin/tags", title: "标签管理" },
  { path: "/admin/users", title: "用户管理" },
  { path: "/admin/backup", title: "备份恢复" },
  { path: "/admin/logs", title: "日志查看" },
  { path: "/admin/telegram", title: "Telegram" },
  { path: "/admin/settings", title: "配置面板" },
] as const;

export const DEFAULT_ADMIN_PAGE_TITLE = "后台管理";

function normalizeAdminPath(pathname: string): string {
  return pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
}

/** Resolve both current routes and future nested detail routes to one page title. */
export function getAdminPageTitle(pathname: string): string {
  const normalizedPath = normalizeAdminPath(pathname);
  const page = ADMIN_PAGE_TITLES.find(
    ({ path }) => normalizedPath === path || normalizedPath.startsWith(`${path}/`)
  );
  return page?.title ?? DEFAULT_ADMIN_PAGE_TITLE;
}

export function shouldShowAdminPageHeader(_pathname: string, _search = ""): boolean {
  return true;
}
