# Omni-Media-Vault（全能媒体库）

<p align="center">
  😄 个人私有全能媒体站 · 视频 / 高清图集 / Telegram全媒体转存 / 多平台视频解析采集 😄
</p>

本项目基于现代化私有流媒体站架构进行深度定制与重构，专注于打造极致的个人私有流媒体中心。系统融合了 **Telegram 全媒体转存**、**多平台视频一键提取（X/Twitter、YouTube、TikTok、抖音、B站等）**、**独立高清相册图集**、**抖音式全屏沉浸流（Shorts）**、**智能话题标签**、**浏览器兼容性转码队列** 以及 **磁盘预留配额安全保护**。

---

## 🌟 核心功能特性

### 1. 全平台视频转存与媒体采集
- **Telegram 全媒体高速转存（Bot API + MTProto User API 双通道引擎）**：
  - **原生机器人高速转存**：通过 Telegram 机器人直接发送视频或照片，全面支持**单张照片与多图相册（Media Group）**自动批量下载与归档。
  - **本地 Bot API 高速模式**：突破官方公网 Bot API 的 20MB/50MB 限制，最大支持 **2GB** 巨幅大文件下载。
  - **MTProto 客户端协议（Telethon 原生通道）**：
    - 支持直接向机器人发送频道/群组消息链接（如 `https://t.me/<channel>/<msg_id>` 或私密群组 `https://t.me/c/<id>/<msg_id>`）；
    - **绕过敏感/受限内容限制**：彻底突破 Telegram 网页端“Please open Telegram to view this post”的限制，以真实客户端 RPC 协议高速拉取成人/受限频道内容；
    - **相册（Media Group）图文全量归档**：智能侦测相册 `grouped_id`，自动将该条动态内的**全部高清图片与原片视频完整转存并分别入库**，前台直出高清画廊与独立视频卡片。
  - **全流程实时进度推送**：交互会话实时推送下载与导入进度（包含百分比、文件大小、速率与耗时预估）。
- **多平台视频一键提取（基于 yt-dlp 深度定制）**：
  - 向 Telegram 机器人直接发送视频链接，后台自动调用定制化 `yt-dlp` 解析引擎并下载最高画质视频。
  - **主流平台原生支持**：
    - **X (Twitter)**：推文视频、短片与动图高画质提取
    - **YouTube**：标准横屏视频与 YouTube Shorts 竖屏短视频
    - **TikTok**：国际版短视频无水印解析与下载
    - **抖音 (Douyin)**：集成专属签名插件与反爬机制，完整支持 `v.douyin.com` 短链接与长链接无水印下载
    - **Bilibili (B站)**：支持 BV 号、av 号以及 `b23.tv` 短链接解析
    - **通用网页视频**：兼容支持 Instagram、快手等几乎所有 yt-dlp 支持的流媒体站点
- **Cookie 全生命周期管理与安全隔离体系**：
  - **后台可视化 Cookie 管理中心**：配置面板提供独立的「Cookie 管理」专区，支持直观查看各平台 Cookie 的状态、有效条数、大小、SHA-256 及修改时间；
  - **在线代码编辑与文件上传**：提供 Monospace 等宽代码编辑器支持在线修改，支持拖拽/选择直接上传本地 `cookies.txt`；
  - **安全备份与一键回滚**：每次保存前自动生成 `.bak` 镜像备份，支持误修改时一键还原上一版本；
  - **格式严格预检**：内置 Netscape 格式预检校验器，自动拦截用户误粘贴的 HTML 登录过期页或 JSON 接口数据，避免爬虫崩溃；
  - **平台级安全隔离机制**：解析抓取时自动按目标域名生成临时凭据物理隔离，避免跨平台串扰，大幅降低账号风控与封禁风险。
- **智能标签与分类归档**：
  - 自动打标系统来源标签（如 `Telegram`、`X`、`YouTube`、`抖音`、`TikTok`、`Bilibili`、`图片` 等）。
  - 自动从 Telegram 配文、视频标题及推文/网页描述中提取 `#话题`，秒级转换为分类标签，支持秒级筛选与检索。

### 2. 沉浸式多媒体浏览体验
- **专业流媒体播放器**：基于 ArtPlayer 深度定制，支持 MP4、WebM、HLS 切片流，支持画质自适应、手势滑动调音/调亮、双击快进、播放倍速调节与画中画。
- **抖音式短视频 / 沉浸流模式 (Shorts)**：
  - 一键切换全屏沉浸流，上下无缝滑动刷片，带来极佳的移动端与大屏刷片体验。
  - 配备快捷侧边栏互动：一键跳转原始链接、关联标签直达、视频信息详情。
- **三屏画面模式**：竖屏视频支持三屏并行对比播放，带来前所未有的全景沉浸感。
- **独立图集专区与高清画廊**：
  - 视频与照片相册彻底隔离，拥有独立图集专区。
  - 瀑布流响应式排版与无缝无限滚动加载。
  - 内置高清查看器（Lightbox），支持手势自由缩放、全屏轮播幻灯片与快捷原图下载。

### 3. 高性能私有存储与系统保护
- **纯粹高效的私有存储架构**：彻底剥离复杂的第三方网盘依赖，专注高性能本地存储与挂载磁盘，支持海量媒体的极速读取与元数据秒级检索。
- **磁盘预留保护（Disk Reserve Quota）**：
  - 后台管理面板支持可视化配置磁盘预留阈值（`DiskReserveBytes`）。
  - 当服务器剩余磁盘空间低于预设阈值时，自动拦截新的转存任务，坚实守护主机系统与数据库安全。
- **浏览器兼容性检测与一键转码**：
  - 后台智能识别视频编码（如 HEVC/H.265 或非常规音频编码在部分浏览器中无法正常播放的问题）。
  - 内置基于 FFmpeg 的后台异步转码队列，一键将高码率或不兼容视频转换为全平台高兼容性的 MP4 (H.264 + AAC + faststart)。

### 4. 现代化管理后台与安全体系
- **全功能管理后台**：集中化管理视频与图集、爬虫采集任务、Telegram 机器人状态、系统日志、系统设置、标签与用户权限。
- **平台 Cookie 可视化管理中心**：专设 Cookie 管理面板，支持 YouTube、抖音、TikTok、X 等多平台凭据的查看、在线代码编辑、本地上传、格式预检与一键回滚。
- **Telegram MTProto 客户端协议助手**：后台直连测试与交互式手机验证码登录生成 Session，轻松开启受限/敏感频道媒体与图文相册全量提取。
- **一次性分享（阅后即焚）**：生成的分享链接仅能打开播放一次，无需登录即可观看，保护私密性。
- **企业级鉴权与防爆破**：密码采用加盐 `bcrypt` 存储，无预置后门；内置 30 分钟内连续失败 3 次自动封禁 IP 机制；网络请求防 SSRF 与安全沙箱隔离。
- **自定义爬虫扩展**：支持接入标准化 Python 爬虫脚本，定时增量抓取流媒体资源。

---

## 🚀 快速开始

### 方式一：一键安装脚本（推荐生产环境）

```bash
sudo apt update && sudo apt install -y curl ca-certificates
curl -fsSL https://raw.githubusercontent.com/z08w12/omni-media-vault/main/install.sh -o install.sh
sudo bash install.sh
```

部署完成后，在浏览器访问：`http://服务器IP:9191/`（若配置了反向代理与域名，推荐通过标准 HTTPS 端口访问）

安装后系统自动注册 `91` 管理命令（保持原有极简运维习惯）：
```bash
91                  # 打开可视化交互管理菜单
91 status           # 查看服务运行状态
91 logs             # 实时追踪服务日志
91 restart          # 重启服务
91 stop             # 停止服务
91 reset-password   # 重置管理员密码（随机生成安全密钥）
91 update           # 更新到最新版本
```

---

### 方式二：Docker Compose 部署

**1. 准备目录**
```bash
mkdir omni-media-vault && cd omni-media-vault
```

**2. 拉取仓库内置 `docker-compose.yml`**
```bash
curl -fsSL https://raw.githubusercontent.com/z08w12/omni-media-vault/main/docker-compose.yml -o docker-compose.yml
```

**3. 启动容器**
```bash
docker compose up -d
```

**常用 Docker 管理命令：**
```bash
docker compose pull && docker compose up -d             # 更新并重启
docker exec -it video-site-91 ./server reset-password   # 重置密码
docker compose logs -f                                  # 查看容器日志
```

---

### 方式三：源码本地一键部署与更新（推荐二次开发）

如果在服务器上需要保留 Git 源码并支持即时定制：

```bash
git clone https://github.com/z08w12/omni-media-vault.git
cd omni-media-vault
sudo bash deploy.sh install
```

后续如有代码更新，只需在仓库目录下执行：
```bash
git pull
sudo bash deploy.sh update
```
脚本会自动重新编译前端与后端二进制，并安全平滑重启系统服务。

---

## 📁 数据存放位置

**一键脚本部署**
- 运行配置文件：`/opt/video-site-91/config.yaml`
- SQLite 数据库：`/opt/video-site-91/data/video-site.db`
- 转存媒体库：`/opt/video-site-91/data/telegram/library/`
- 各平台受保护 Cookie 目录：`/opt/video-site-91/data/cookies/`
- 缩略图、预览片段、转码临时目录：`/opt/video-site-91/data/`

**Docker Compose 部署**
- 运行配置文件：`./data/config.yaml`
- 数据库文件：`./data/video-site.db`
- 转存媒体库：`./data/telegram/library/`
- 各平台 Cookie 目录：`./data/cookies/`

---

## 🛠️ Telegram 与多平台转存配置

### 1. 准备 Telegram 相关凭据
- **Bot Token**：在 Telegram 中向官方 `@BotFather` 发送 `/newbot` 指令创建机器人并获取 Token。
- **API ID 与 API Hash**（用于启用本地高速 API 或 MTProto 客户端协议）：登录 [my.telegram.org](https://my.telegram.org)，在 *API development tools* 中申请。

### 2. 部署本地 Telegram Bot API 服务（可选，支持 2GB 大文件高速转存）
Telegram 官方公网接口对 Bot 限制单文件最大下载 20MB。为转存高清长视频与大文件，推荐启用本地 Bot API 容器：
- **一键脚本部署**：编辑 `/opt/video-site-91/telegram.yml`，填入 `TELEGRAM_API_ID` 与 `TELEGRAM_API_HASH`，执行 `sudo docker compose -f telegram.yml up -d --pull always` 并执行 `sudo 91 restart`。
- **Docker Compose 部署**：在编排配置中配置 `telegram-bot-api` 容器，并与主程序挂载相同的媒体共享目录。

> [!TIP]
> 详细架构流程图与编排配置模板，请参阅：👉 [Telegram 本地高速 API 部署指南](backend/docs/Enable_Telegram.md)

### 3. 配置 Telegram 客户端协议（MTProto User API，突破受限/敏感频道与相册转存）
针对部分设置了成人/敏感内容保护的频道（网页端会提示 `Please open Telegram to view this post`）以及带有相册组（包含多图+视频）的动态：
1. 前往 [my.telegram.org](https://my.telegram.org) 登录并进入 *API development tools* 获取 `API ID` 与 `API Hash`；
2. 登录 Web 管理后台，进入 **配置面板 → Telegram**，在「Telegram 客户端协议」专区勾选启用，并填入 `API ID` 与 `API Hash`；
3. **快速生成凭据 Session**：点击「使用手机号辅助登录生成 Session」按钮，输入手机号和 Telegram 发送的验证码（若开启 2FA 需输入二次密码），系统将全自动登录并持久化保存 Session；
4. 点击「测试当前连接」即可实时核验登录状态。配置完成后，直接向机器人发送频道/群组消息链接（如 `https://t.me/renqi85/53569`），系统将全自动解析并将所有图片与视频分别入库！

### 4. 平台 Cookie 管理与配置
对于抖音、TikTok、YouTube、X 等平台的高画质或私密内容提取，可以在后台导入 Cookie：
1. 登录 Web 管理后台，进入 **配置面板 → Cookie 管理**；
2. 点击对应平台卡片（如 YouTube、抖音、TikTok、X/Twitter 等）的「编辑」按钮，直接粘贴导出的 Netscape 格式 cookies.txt 内容，或点击「上传」直接导入文件；
3. 保存时系统自动进行 Netscape 语法预检并生成 `.bak` 镜像备份，如需还原可随时点击「回滚」。

### 5. Web 后台启用与使用
1. 登录 Web 管理后台，进入 **配置面板 → Telegram**，填入 Bot Token 并开启 Telegram 支持保存；
2. 私聊你的机器人发送 `/id` 获取 Telegram 数字 ID，将其填入后台「允许的用户 ID」白名单保存；
3. 向机器人发送视频、照片图集（Media Group），或直接发送各平台视频链接（X/Twitter、YouTube、TikTok、抖音、B站等），机器人将自动解析并实时推送下载与入库进度！

---

## 📜 使用须知

- 本项目仅供个人学习、技术研究及私有化部署使用。
- 用户使用转存与爬虫功能时请遵守当地法律法规，尊重版权方的合法权益。

---

## 👏 致谢

- [91](https://github.com/nianzhibai/91) — 优秀的开源私有流媒体站基座
- [telegram-media-vault](https://github.com/z08w12/telegram-media-vault) — Telegram 媒体转存与推特抓取参考实现
- [ArtPlayer](https://github.com/zhw2590582/ArtPlayer) — 现代化功能完善的 HTML5 播放器
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — 强大的命令行音视频提取利器
