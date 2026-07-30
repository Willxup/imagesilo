<p align="center">
  <img src="./assets/brand/imagesilo-logo-card.png" alt="ImageSilo" width="560" />
</p>

<p align="center">
  <a href="./README.md">English</a> ｜ <a href="./README.zh.md"><strong>简体中文</strong></a>
</p>

<h1 align="center">ImageSilo</h1>

<p align="center">给图片一个轻量、稳定的自托管归处。</p>

<p align="center">
  <a href="https://github.com/Willxup/imagesilo/releases"><img src="https://img.shields.io/github/v/release/Willxup/imagesilo?include_prereleases&amp;style=flat-square" alt="最新版本" /></a>
  <a href="https://github.com/Willxup/imagesilo/actions/workflows/verify.yml"><img src="https://img.shields.io/github/actions/workflow/status/Willxup/imagesilo/verify.yml?branch=main&amp;style=flat-square&amp;label=CI" alt="CI 状态" /></a>
  <a href="https://github.com/Willxup/imagesilo/pkgs/container/imagesilo"><img src="https://img.shields.io/badge/Docker-GHCR-2496ED?style=flat-square&amp;logo=docker&amp;logoColor=white" alt="GHCR Docker 镜像" /></a>
  <img src="https://img.shields.io/badge/Linux-amd64%20%7C%20arm64-FCC624?style=flat-square&amp;logo=linux&amp;logoColor=black" alt="Linux amd64 和 arm64" />
</p>

ImageSilo 是一个 Docker 优先的自托管图床：单个 Go 进程、SQLite 元数据和本地文件存储。它提供安全上传、稳定公开 URL、历史别名、具名 API Token、图片处理和响应式 React 管理界面，不依赖 Redis、外部数据库或后台任务服务。

> 当前版本为 `v0.1.0-rc.1`。生产部署应固定不可变版本标签或 digest；ImageSilo 不会自动发布 `latest`。

## 功能特性

- 严格解码 JPEG、PNG、WebP 和 GIF，并设置 16 MP 像素安全边界
- 默认保持原始字节，也可明确压缩或将静态图片转换为 WebP
- 标准公开 URL 和历史别名支持 Range、ETag、条件请求与 HEAD
- 图片交付热路径完全使用内存索引，每次读取不查询 SQLite
- 支持公开/私密可见性、管理员 Session、CSRF、防登录爆破和分 Scope API Token
- 内置管理界面可上传、搜索、筛选、查看详情、批量修改和永久删除图片
- 支持旧 URL 导入、SHA-256 字节校验、存储巡检和内存索引重建
- 以非 root 多架构容器运行，图片处理并发默认固定为 `1`

## 快速开始

Docker 是唯一支持的生产运行方式。以下命令使用 named volume 启动一个本地体验实例：

```bash
export IMAGESILO_IMAGE=ghcr.io/willxup/imagesilo:v0.1.0-rc.1

docker pull "$IMAGESILO_IMAGE"
docker volume create imagesilo-data

docker run --rm --interactive --tty \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE" \
  admin create --email admin@example.com

docker run --detach \
  --name imagesilo \
  --restart unless-stopped \
  --publish 127.0.0.1:8080:8080 \
  --env IMAGESILO_COOKIE_SECURE=false \
  --env IMAGESILO_PROCESSING_CONCURRENCY=1 \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE"
```

打开 `http://127.0.0.1:8080/admin/login`，使用刚创建的管理员账号登录。

生产环境应在反向代理终止 HTTPS、保持 `IMAGESILO_COOKIE_SECURE=true`，并在停止写入时完整备份 `/data`。对外开放前请先阅读[部署指南](./docs/deployment.md)。

## 设计边界

| 范围 | ImageSilo V1 |
| --- | --- |
| 运行时 | 一个容器内的单个 Go 进程 |
| 元数据 | `/data/db` 中的 SQLite |
| 图片 | `/data/images` 中的本地文件 |
| 缓存 | `/data/cache` 中的本地缩略图 |
| 图片处理 | libvips，默认并发 `1` |
| 平台 | `linux/amd64`、`linux/arm64` |
| 生产存储 | Docker named volume 或本地 bind mount |

不支持 NFS 和 SMB，也不能让两个 ImageSilo 容器同时写入同一个 `/data`。

## 本地开发

### 前置依赖

- Go 1.26.5
- Node.js 26.5.0
- npm

### 构建与验证

```bash
npm --prefix web ci
make check
make build
```

创建管理员并运行本地二进制：

```bash
IMAGESILO_DATA_DIR="$PWD/data" \
  ./bin/imagesilo admin create --email admin@example.com

IMAGESILO_DATA_DIR="$PWD/data" \
IMAGESILO_COOKIE_SECURE=false \
  ./bin/imagesilo serve
```

`/healthz` 用于存活检查，`/readyz` 用于 SQLite 就绪检查。运行 `make e2e` 可执行单 worker 浏览器闭环。

## 文档导航

| 主题 | 文档 |
| --- | --- |
| Docker 部署、备份、升级与回滚 | [部署指南](./docs/deployment.md) |
| API Token、curl、PicGo 与 ShareX | [API Token 使用](./docs/api-token-usage.md) |
| 旧图片与历史 URL 迁移 | [导入指南](./docs/imports.md) |
| 巡检、清理与索引重建 | [运维指南](./docs/operations.md) |
| 架构与数据模型 | [架构](./docs/architecture.md) · [数据模型](./docs/data-model.md) |
| 安全与轻量资源证据 | [安全审计](./docs/security-audit.md) · [性能基线](./docs/performance-baseline.md) |
| 发布镜像与验收 | [发布指南](./docs/release.md) |
| HTTP API 契约 | [OpenAPI](./api/openapi.yaml) |

## 项目状态

阶段 0—7 已完成，包括原生 amd64/arm64 验证和公开的 `v0.1.0-rc.1` 发布候选。生产迁移仍是独立的环境相关步骤；当前证据与剩余工作见[开发状态](./docs/development-status.md)。
