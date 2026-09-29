# 91 与 telegram-media-vault 融合改造说明

本项目基于 [91](https://github.com/z08w12/91) 进行了架构级深度改造，完全融合了 [telegram-media-vault](https://github.com/z08w12/telegram-media-vault) 的核心转存与展示能力。

---

## 一、 本次融合新增与改造的核心功能

### 1. Telegram 图片与图集转存 (Photo & Albums)
- **多尺寸自动择优**：自动选取 Telegram 发送照片中的最高分辨率原图。
- **文档图片识别**：支持直接发送无压缩的原图文件（`document` 格式的 `.jpg`, `.png`, `.webp`, `.gif` 等）。
- **图集相册完美支持 (Media Group)**：Telegram 发送多张照片或图集时，自动聚合消息组并分批入库，保留原始标题并自动追加序号后缀（例如 `标题 - 1`, `标题 - 2`）。
- **轻量封面生成**：图片入库后，自动通过 FFmpeg 在 0 秒偏移处生成缩略图，并自动禁用视频预览切片生成（避免无谓开销）。

### 2. X (Twitter) 链接视频转存
- **链接正则智能匹配**：Bot 接收文本或带附带文字的消息时，自动提取 `x.com` 或 `twitter.com` 状态链接。
- **调用 yt-dlp 高效解析**：底层调用系统 `yt-dlp` CLI，提取推文最佳音画视频流合并为 MP4，自动提取推文文字作为标题与简介，自动提取推主用户名（如 `@elonmusk`）作为作者。
- **重复防重防护**：自动在数据库比对 X 帖子唯一状态 ID，已转存过的帖子即时友好提示 `✅ 这个 X 帖子已经转存过`。

### 3. 标签与 #Hashtag 自动提取打标
- **正文标签解析**：自动解析 Telegram 消息与 X 帖子中的 `#hashtag`，与 91 统一标签池无缝打通。
- **来源标识打标**：
  - 来自 Telegram 的媒体自动打上 `TG` 标签；
  - 来自 X (Twitter) 的媒体自动打上 `X` 标签；
  - 图片媒体自动打上 `图片` 标签。

### 4. 前端媒体卡片与图片浏览体验 (React + Vite)
- **媒体卡片识别**：媒体列表卡片中，图片时长徽标显示为 `图片`，自动关闭无效的视频悬浮预览，点击直达详情页。
- **交互式图片浏览组件 (`ImageViewer.tsx`)**：
  - 居中自适应高质量渲染；
  - 支持点击放大 / 全屏沉浸式画廊浏览（支持 Esc 退出）；
  - 提供 “下载原图” 与 “新标签页打开原图” 快捷操作；
  - 继承海报模糊背景（Ambient 氛围灯光效果）。

---

## 二、 存量数据一键迁移脚本

在项目根目录提供了 `scripts/migrate_telegram_media_vault.py`：

```bash
# 在 VPS 上执行迁移（将 telegram-media 历史数据导入 91）
python3 /opt/video-site-91/scripts/migrate_telegram_media_vault.py \
  --old-data /var/lib/telegram-media \
  --91-data /opt/video-site-91/data
```

脚本将自动执行：
1. 读取旧版 `media.db` 中状态为 `ready` 的所有图片与视频记录；
2. 将文件通过硬链接或复制平滑迁移至 91 的媒体库目录；
3. 将元数据、发布时间、推文作者、标签自动导入 91 的 `video-site.db`；
4. 写入去重索引，防止未来向 Bot 转发重复文件。

---

## 三、 推送至 GitHub 与部署

### 1. 本地提交并推送到你的 GitHub 仓库
在 `D:\other\91` 目录下执行：

```bash
git add .
git commit -m "feat: merge telegram-media-vault features (photos, X video extraction, image viewer)"
git push origin main
```

### 2. 发布新版本（触发 GitHub Actions 自动打包构建）
如果需要打 Release 包供一键升级：

```bash
git tag v0.3.5
git push origin v0.3.5
```
GitHub Actions 会自动在 Linux ARM64 和 AMD64 环境下完成构建并发布 Release。

### 3. 在 ARM VPS 上部署运行
在 VPS 上更新二进制并重启：
```bash
sudo 91 restart
```
并在 `/opt/video-site-91/config.yaml` 或 Web 管理后台启用 Telegram 并配置白名单用户 ID 即可开始使用。
