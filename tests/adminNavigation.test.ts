import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

// Shared admin coverage retained when the retired drive form suite was removed.
const source = (path: string) =>
  readFileSync(new URL(`../src/${path}`, import.meta.url), "utf8");
const adminLayoutSource = source("admin/AdminLayout.tsx");
const appSource = source("App.tsx");
const loginPageSource = source("admin/LoginPage.tsx");
const crawlerPageSource = source("admin/CrawlersPage.tsx");
const adminControlsCss = source("styles/admin-controls.css");
const adminCss = source("styles/admin.css");
const loginCss = source("styles/login.css");
test("system sidebar contains navigation only while global actions live in the layout", () => {
  assert.match(
    adminLayoutSource,
    /admin-nav__group-label">系统[\s\S]*?to="\/admin\/backup"[\s\S]*?to="\/admin\/logs"[\s\S]*?to="\/admin\/settings"/
  );
  assert.doesNotMatch(adminLayoutSource, /to="\/admin\/theme"/);
  assert.doesNotMatch(adminLayoutSource, /admin-nav__action|admin-sidebar__mobile-panel/);
  assert.match(adminLayoutSource, /<AdminGlobalActions[\s\S]*?onCheckUpdate=[\s\S]*?onLogout=/);
});

test("admin layout and its stylesheet load as one lazy route chunk", () => {
  assert.doesNotMatch(appSource, /import \{ AdminLayout \} from "@\/admin\/AdminLayout";/);
  assert.match(
    appSource,
    /const AdminLayout\s*=\s*lazy\(\(\) =>[\s\S]*?import\("@\/admin\/AdminLayout"\)[\s\S]*?default: module\.AdminLayout/
  );
  assert.match(
    adminLayoutSource,
    /import "@\/styles\/admin-controls\.css";\s*import "@\/styles\/admin\.css";/
  );
  assert.match(
    loginPageSource,
    /import "@\/styles\/admin-controls\.css";\s*import "@\/styles\/login\.css";/
  );
  for (const selector of ["admin-form", "admin-password-input", "admin-btn"]) {
    const baseRule = new RegExp(`^\\.${selector}\\s*\\{`, "m");
    assert.match(adminControlsCss, baseRule);
    assert.doesNotMatch(adminCss, baseRule);
    assert.doesNotMatch(loginCss, baseRule);
  }
  assert.match(
    appSource,
    /import \{ CrawlersPageLoading \} from "@\/admin\/CrawlersPageLoading";/
  );
  assert.doesNotMatch(appSource, /<Suspense fallback=\{null\}>\s*<Routes>/);
  assert.match(appSource, /function PageSuspense\(\{[\s\S]*fallback = null,[\s\S]*fallback\?: ReactNode/);
  assert.match(appSource, /path="\/admin"[\s\S]*?<PageSuspense>\s*<AdminLayout \/>\s*<\/PageSuspense>/);
  assert.match(
    appSource,
    /path="crawlers"[\s\S]*<PageSuspense fallback=\{<CrawlersPageLoading \/>\}>[\s\S]*<CrawlersPage \/>[\s\S]*<\/PageSuspense>/
  );
});

test("empty crawler list renders the shared empty visual", () => {
  assert.match(crawlerPageSource, /className="admin-page admin-page--with-floating-actions admin-crawlers-page"/);
  assert.match(
    crawlerPageSource,
    /<AdminEmptyVisual[\s\S]*?variant="empty"[\s\S]*?text="暂无爬虫"[\s\S]*?className="admin-crawler-empty"/
  );
  assert.doesNotMatch(crawlerPageSource, /admin-crawler-list__controls/);
  assert.doesNotMatch(crawlerPageSource, /<SpiderIcon size=\{28\} \/>/);
  assert.match(
    adminCss,
    /\.admin-page\s*\{[^}]*display\s*:\s*flex;[^}]*flex\s*:\s*1 1 auto;[^}]*flex-direction\s*:\s*column;[^}]*min-height\s*:\s*0/s
  );
  assert.doesNotMatch(
    adminCss,
    /\.admin-crawlers-page\s*\{[^}]*min-height\s*:\s*calc\(100(?:d)?vh/s
  );
  assert.match(
    adminCss,
    /\.admin-crawler-empty\s*\{[^}]*flex\s*:\s*1 1 auto;[^}]*min-height\s*:\s*0;[^}]*justify-content\s*:\s*center/s
  );
});
