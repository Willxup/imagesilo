# 阶段 7 安全与轻量资源审计

审计日期：2026-07-29。

## 已自动化的边界

| 风险 | 自动化证据 |
|---|---|
| 路径穿越、双重编码和保留路由 | `internal/delivery/path_test.go`、`internal/platform/storage/filesystem_test.go` |
| Session 轮换、CSRF、登录限速和安全响应头 | `internal/httpapi/phase_two_test.go` |
| 私密图片、Token Scope、吊销、过期和越权删除 | `internal/httpapi/phase_two_test.go`、`internal/httpapi/phase_five_test.go`、桌面 Playwright |
| 损坏、伪装、截断和超像素图片 | `internal/platform/processor/processor_test.go` |
| 标准 URL/别名、Range、416、ETag、条件请求和 HEAD | `internal/httpapi/delivery_handler_test.go`、`scripts/container-smoke.sh` |
| 上传/删除/巡检/重建后的临时文件 | `internal/httpapi/phase_seven_test.go`、`scripts/container-smoke.sh` |
| goroutine、文件描述符和优雅停止 | `scripts/container-smoke.sh`、`scripts/remote-release-acceptance.sh` |
| 非 root、固定 UID/GID、只读数据目录错误和精简镜像 | `deploy/docker/Dockerfile`、`scripts/container-smoke.sh` |

容器协议检查只追加 4 次串行公开读取；公开交付的两个重叠读使用微型内存夹具，仅证明并发安全，不是压力测试。持续图片处理 benchmark 仍是每架构并发 `1`、总计 `16` 个请求。

## 依赖扫描

`govulncheck` 曾在实际调用链发现 `golang.org/x/image v0.38.0` 的两个 WebP 解码漏洞：`GO-2026-5061` 和 `GO-2026-4961`。依赖已原位升级到修复两项的 `v0.43.0`，没有增加新模块或服务。

`npm audit` 报告 React Router 的 `GHSA-qwww-vcr4-c8h2`。官方公告说明它只影响 unstable RSC APIs，ImageSilo 的 Vite SPA 没有 RSC、SSR、Server Action 或框架模式；截至审计时 npm 尚未发布公告标注的 8.3.0 修复版。该项作为不可达代码路径的临时例外记录，并在每次发布前复查。

## 镜像审计

本机原生 arm64 构建的 Docker 未压缩大小为 `117,585,383` 字节。最终镜像中的 ImageSilo 二进制为 `10,754,496` 字节，精简后的 `/opt/vips/lib` 为 `2,562,441` 字节；不包含 Node、Go、编译器、源码、测试素材或 libvips 的头文件、pkg-config、静态库和命令行工具。

受限验收脚本先在本机以 `0.5` CPU、`256m` 内存、处理并发 `1` 自测 10 秒，共完成 5 轮串行 health/readiness/标准 URL/别名读取。cgroup 当前内存 `30,154,752` 字节、峰值 `30,932,992` 字节，goroutine `10 → 10`，文件描述符 `10 → 10`，临时文件为零。

阶段 7 对 16 MP 固定最坏样本追加了单请求 A/B。原样保存峰值 `57,864,192` 字节；WebP effort 4 的转换峰值 `229,818,368` 字节、耗时 `1,638 ms`、输出 `5,620,192` 字节；effort 1 的峰值 `197,984,256` 字节、耗时 `1,130 ms`、输出 `6,012,488` 字节。最终选择 effort 1：单次编码峰值下降约 13.9%，耗时下降约 31%，代价是该极端样本输出增加约 7%，不改变像素、质量值或格式。

空库运行基线继续记录在 `docs/performance-baseline.md`。镜像磁盘大小不等于运行内存，阶段 7 仍以实际 RSS、Go heap、cgroup peak、goroutine 和文件描述符为资源事实来源。
