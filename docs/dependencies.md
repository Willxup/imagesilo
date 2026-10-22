# 依赖版本基线

依赖解析基线日期：2026-07-29；Go/Node.js 安全下限与基础镜像策略复核日期：2026-10-22。工具链使用最低版本要求和稳定系列滚动更新，不选 alpha、beta、RC、preview 或 nightly；应用依赖仍通过 `go.mod`、`go.sum` 与 `web/package-lock.json` 管理，不随工具链策略放宽。

## 工具链与原生组件

| 组件 | 版本要求 | 官方来源 |
|---|---:|---|
| Go | >=1.26.10 | `https://go.dev/dl/` |
| Node.js | >=24.22.1 | `https://nodejs.org/dist/` |
| TypeScript | 5.9.3 | `https://registry.npmjs.org/typescript/5.9.3` |
| SQLite | 3.53.4 | `https://www.sqlite.org/download.html` |
| libvips | 8.18.4 | `https://github.com/libvips/libvips/releases/tag/v8.18.4` |

Go 最低版本为 1.26.10，以修复标准库 `crypto/x509` 的 [CVE-2026-111943](https://www.cve.org/CVERecord?id=CVE-2026-111943)（[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)）。`go.mod` 的 `go` 指令声明最低版本，不添加精确 `toolchain` 指令。

Node.js 最低版本为 24.22.1，对应 [2026 年 10 月安全发布](https://nodejs.org/en/blog/vulnerability/october-2026-security-releases)。`web/package.json` 声明 `>=24.22.1`，`web/.npmrc` 强制检查 engines，Docker 前端阶段也复制该配置。Node.js 仅用于前端构建，不包含在最终运行镜像中。

本地不保留 `.go-version`、`.node-version` 等精确版本文件，允许共用满足最低要求的工具链。CI 通过 `check-latest: true` 跟随最新 Go `1.26.x` 和 Node.js `24.x`，不自动跨版本系列。本地允许更高系列不代表所有未来版本都已验证或没有漏洞，安全下限仍需维护。历史 benchmark 中的工具链版本保留为测量记录。已发布的二进制和容器镜像需要以新版本重新构建并部署才能获得修复。

本地编译使用 `github.com/mattn/go-sqlite3` 自带的 SQLite amalgamation；阶段 0 在下载模块后核对其 `SQLITE_VERSION`，不得把系统 SQLite 版本误当作二进制实际版本。

## Go 直接依赖

| 模块 | 精确版本 | 解决的问题 |
|---|---:|---|
| `github.com/go-chi/chi/v5` | 5.3.1 | 轻量 HTTP 路由与标准中间件 |
| `github.com/mattn/go-sqlite3` | 1.14.49 | SQLite CGO 驱动 |
| `github.com/google/uuid` | 1.6.0 | UUIDv7 生成与解析 |
| `golang.org/x/crypto` | 0.54.0 | Argon2id 密码哈希 |
| `golang.org/x/image` | 0.43.0 | 便携构建的 WebP 解码与缩略图缩放 |
| `golang.org/x/term` | 0.45.0 | 管理员 CLI 无回显读取密码 |
| `github.com/davidbyttow/govips/v2` | 2.18.0 | 阶段 3 的 libvips 边界 |

## 前端直接依赖

精确版本记录在 `web/package.json`；`web/package-lock.json` 是完整、可复现的依赖树。Radix primitive 只在真实组件出现时逐个引入，不预装整套组件。

TypeScript 的 `latest` 标签在解析日为 7.0.2，但 `openapi-typescript` 7.13.0 的 peer dependency 明确为 TypeScript 5.x，`typescript-eslint` 8.65.0 也尚未声明 TypeScript 7 支持。因此锁定可解析依赖树中的最新 TypeScript 5.x（5.9.3），不使用 `--force` 或 `--legacy-peer-deps` 隐藏不兼容。

React Router 7.18.2 在解析日命中 `GHSA-qwww-vcr4-c8h2`。GitHub 官方公告明确写明“仅使用 unstable RSC APIs 时受影响”，首个修复版本标为 8.3.0；但 npm 在 2026-07-29 尚未发布 8.3.0，最新正式版仍是 7.18.2。ImageSilo 只使用 Vite SPA 的 BrowserRouter，不启用 React Server Components、SSR、Server Action 或框架模式，因此没有该公告中的服务端 Action 执行面。每次发布仍重新检查，一旦官方发布兼容修复版立即升级。

`openapi-typescript` 的构建期传递依赖通过 npm `overrides` 锁定到已修复的 `js-yaml` 4.3.0、`minimatch` 10.2.6 和 `brace-expansion` 5.0.8。它只解析仓库内受信任的 `api/openapi.yaml`，不处理用户输入。

## Docker 基础镜像

以下使用滚动标签，不固定补丁版本或 digest；构建与运行环境均保留 Debian Bookworm，保证 CGO/libvips 的 libc 兼容性：

| 用途 | 镜像 | 更新范围 |
|---|---|---|
| Go 构建 | `golang:1.26-bookworm` | Go 1.26 系列及 Bookworm 更新 |
| Node 构建 | `node:24-bookworm-slim` | Node.js 24 系列及 Bookworm 更新 |
| 运行时 | `debian:bookworm-slim` | Bookworm 更新 |

`make docker-build` 和 Release 原生构建均使用 `--pull`，手动构建也必须加 `--pull`。同一源码在不同时间构建可能使用不同基础镜像；新发布仍需通过 amd64/arm64 构建与 smoke test。该策略不会更新已运行的容器，也不会覆盖已发布的不可变镜像。
