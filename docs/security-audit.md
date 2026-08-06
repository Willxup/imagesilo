# 阶段 7 安全与轻量资源审计

初始审计日期：2026-07-30。全仓复审与修复日期：2026-07-31。

## 2026-07-31 全仓复审修复

- 首次 Web 初始化必须使用进程启动时生成的 `isb_...` bootstrap token。日志只在未初始化时输出原文，服务内仅保存 SHA-256，成功后立即清除。
- Argon2 维持约 19 MiB 参数并全局串行执行；setup 在 KDF 前检查初始化状态和 token，登录与改密按凭据串行，避免内存放大和旧密码会话竞态。
- 上传与历史路径导入在读取正文前抢占处理槽，使用单遍 multipart 流并只落一次 `/data/tmp`；GIF 在完整解码前完成帧预算检查。
- 删除、可见性修改和 WebP 转换按 image ID 串行；转换无法生成新缩略图时会移除旧缩略图。
- Nginx Proxy Manager 部署默认信任代理地址头，按 `X-Real-IP`、最右有效 `X-Forwarded-For`、TCP 对端顺序取客户端 IP；原生监听默认仅为 `127.0.0.1:8080`，容器镜像明确监听 `:8080`。
- 标准 URL、别名、迁移目录和缩略图共享可配置的读取并发门；默认 `0` 表示不限制，正整数上限满载时返回 `503`。HTTP 服务同时设置请求头、正文、响应和空闲期限。
- 托管文件通过 root-scoped 打开；启动和重建会将文件大小不符的记录排除交付，巡检会把它们报告为不可用；迁移目录还会校验扩展名对应的真实图片 MIME。
- SQLite 数据目录收紧为 `0700`，主库及现存 WAL/SHM 收紧为 `0600`。
- 前端统一处理 401、清理用户查询缓存；Modal 具备焦点陷阱和背景 inert；筛选 URL、日期时区、批量快照、分页、设置草稿、失败原因、键盘操作和受限 localStorage 均有回归测试。
- 发布工作流验证 tag 提交属于 `main`，使用 fail-closed GHCR 查询、提交绑定 content tag、平台 digest 和幂等 manifest；benchmark 仍只跑并发 1，同时加入 CPU、内存、PID、成功率、p95 和峰值门禁。
- Docker build context 改为白名单；远程脚本仅接受规范化的 `/var/tmp/imagesilo-*`，名称参数受限且不再递归改权。TailAdmin 与 Outfit 的许可证随源码及镜像分发。

为保持轻量，图片正文不会复制进 Go 全图缓存，也不会在每次启动对全部图库重新计算 SHA-256；交付使用内存元数据索引、ETag、文件流和操作系统页缓存。启动与巡检执行廉价的存在性、普通文件和大小校验，卷本身仍属于可信本地存储边界。React Router 当前唯一生产依赖 High 公告仅影响未使用的 RSC API，继续作为明确例外记录；新增 npm 生产依赖 Critical 会阻断 release。项目自身采用何种许可证仍需仓库所有者选择，本次没有擅自代选。

本轮本机原生 arm64 生产镜像大小为 `118,770,769` 字节。受限 benchmark 使用 1 CPU、768 MiB、256 PID、处理并发 1 和 16 个连续 16 MP 转 WebP 请求：16/16 成功、busy 为 0、p95 为 `1,725.309 ms`，cgroup 当前 `139,796,480` 字节、峰值 `345,550,848` 字节，其中 anon `84,873,216` 字节、file `52,105,216` 字节。10 万图片加 10 万别名的交付索引为 `148.70` bytes/path，100 万次标准/别名命中及 miss 的 `lookupFailures` 均为 0。

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

阶段 7 对 16 MP 固定最坏样本追加了 effort 1/4 A/B。effort 1 的单请求峰值和耗时一度分别下降约 13.9% 和 31%，但双架构连续 16 请求后匿名内存升到 arm64 `249,716,736` 字节、amd64 `258,211,840` 字节，容器峰值也反而超过 `497 MB`。因此该候选被撤回，最终继续使用 effort 4；轻量决策以持续工作集为准，不以单请求速度为准。

最终 [Verify run 30479364047](https://github.com/Willxup/imagesilo/actions/runs/30479364047) 在原生 amd64/arm64 上全部通过。16 个 WebP 输出合计 `89,923,072` 字节；两个架构结束文件页缓存均约 `91.4 MB`，与输出总量吻合。结束匿名内存分别为 amd64 `86,945,792` 字节、arm64 `101,023,744` 字节，证明约 397 MB 的 cgroup 峰值不能全部解释为进程泄漏。

外部原生 amd64 共享主机从提交 `b866eeb` 构建候选：专用 BuildKit 固定到 CPU 0、1 CPU quota、1 GiB 内存和 1 GiB memory+swap 上限，完成后删除。随后容器以 0.5 CPU、256 MiB、128 PID、处理并发 1 连续运行 600 秒，完成 118 轮串行字节校验；cgroup 当前 `14,135,296` 字节、峰值 `35,938,304` 字节、结束匿名内存 `8,814,592` 字节，goroutine `10 → 8`、FD `10 → 10`，最终 SIGTERM 退出码为 0。证据归拢在专用 `/var/tmp/imagesilo-*` 目录；builder、容器和临时候选镜像均已移除。

空库运行基线继续记录在 `docs/performance-baseline.md`。镜像磁盘大小不等于运行内存，阶段 7 仍以实际 RSS、Go heap、cgroup peak、goroutine 和文件描述符为资源事实来源。

## 发布证据

`v0.1.0-rc.1` 的 tag push 自动触发 [Release image run 30505989284](https://github.com/Willxup/imagesilo/actions/runs/30505989284)，质量门、原生 amd64/arm64 构建、完整容器 smoke、平台镜像推送和 manifest 校验全部成功。工作流已移除手动触发入口，只接受 `v*` tag push。

公开 OCI manifest 为 `sha256:573233d00455ab6e8dad9d875ad0ede7f770436ce24bb05f6d21bc02fac053cc`；amd64 子 manifest 为 `sha256:ee410a57912c125234e5547520a1f5b38a253649cfeec88aec34d53903838ad8`，arm64 子 manifest 为 `sha256:a5437e2348e74a9aaa53e0ac7f685c379153d9183362651e1a88a077f9f9452d`。独立匿名 GHCR token 请求返回 HTTP 200，证明无需仓库或包权限即可拉取。

2026-08-06 复核已将 `docker/login-action` 升级到声明 Node.js 24 runtime 的 4.6.0，并继续固定到已验证的完整 commit SHA。仓库内 `actions/checkout`、`actions/setup-go`、`actions/setup-node` 和 `docker/login-action` 的当前 pin 均声明 `node24`，原 Node.js 20 弃用提示已从配置源消除。
