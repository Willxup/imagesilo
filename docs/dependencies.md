# 依赖版本基线

解析日期：2026-07-29。全部版本排除了 alpha、beta、RC、preview 和 nightly；直接依赖使用精确版本，前端完整依赖树由 `package-lock.json` 锁定。

## 工具链与原生组件

| 组件 | 精确版本 | 官方来源 |
|---|---:|---|
| Go | 1.26.5 | `https://go.dev/dl/` |
| Node.js | 26.5.0 | `https://nodejs.org/dist/` |
| TypeScript | 5.9.3 | `https://registry.npmjs.org/typescript/5.9.3` |
| SQLite | 3.53.4 | `https://www.sqlite.org/download.html` |
| libvips | 8.18.4 | `https://github.com/libvips/libvips/releases/tag/v8.18.4` |

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

以下均锁定多架构 OCI index digest：

| 用途 | 镜像 | Digest |
|---|---|---|
| Go 构建 | `golang:1.26.5-bookworm` | `sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651` |
| Node 构建 | `node:26.5.0-bookworm-slim` | `sha256:2d49d876e96237d76de412761cf05dbfe5aee325cc4406a4d41d5824c5bb8beb` |
| 运行时 | `debian:bookworm-slim` | `sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818` |

这些 digest 在依赖升级时重新解析，并重新执行 amd64/arm64 构建与 smoke test。
