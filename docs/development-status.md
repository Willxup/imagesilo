# 开发状态与准备度

更新日期：2026-07-30

## 结论

项目已完成阶段 0—7 的开发、双架构验证和首个公开发布候选。阶段 8 仅等待用户提供旧系统数据副本、目标生产服务器与反向代理窗口，以执行生产迁移和回滚演练。

仓库开始时为空，远程仓库为 `git@github.com:Willxup/imagesilo.git`，因此 Go module 使用 `github.com/Willxup/imagesilo`。

## 当前环境证据

- 本机为 macOS arm64，Go、Node、Clang 和 Docker buildx CLI 可用。
- 本机 OrbStack Docker daemon 可用，arm64 原生镜像与 amd64 模拟镜像均可构建和运行。
- 本机未安装 libvips；阶段 3 的处理器以 Docker 中锁定的 libvips 8.18.4 为可复现构建边界，不把开发机全局库当作事实来源。
- arm64 本机原生运行和 amd64 OrbStack 模拟用于开发反馈；GitHub 官方原生 amd64/arm64 runner 用于阶段门证据。

## 已完成

- 阶段 0：依赖、工具链和 Docker 基础镜像 digest 已锁定；Go、React、OpenAPI、SQLite 迁移、Makefile 和 Compose 基础工程通过完整检查。
- 阶段 1 的代码与本机闭环：无回显管理员创建、Argon2id、Session 哈希内存索引、流式 JPEG 上传、原子落盘、失败补偿、图片列表、内存交付索引、Range/ETag、React 登录/上传/列表、重启恢复全部完成。
- arm64 原生容器闭环通过；amd64 OrbStack 模拟容器闭环通过。
- 阶段 2 本机闭环：Session 轮换、退出、改密、过期清理、CSRF、双维度登录限速、安全响应头、四种 API Token Scope、只显示一次与哈希存储、吊销/过期内存失效、默认/单次/现有图片可见性、公开/私密缓存策略，以及 Session/Bearer 私密读取全部完成。
- 阶段 2 的私密读取集成测试会先关闭 SQLite，再分别使用管理员 Session 和 `images:read_private` Token 读取图片，直接证明热路径没有 SQL 回源。
- 阶段 2 的 React 管理界面已加入上传可见性、图片可见性切换、API Token 创建/一次性复制/吊销、默认可见性和修改密码；OpenAPI 生成类型、curl、PicGo 与 ShareX 示例已同步。
- Phase 2 arm64 与 amd64 本地容器均通过管理员初始化、CSRF 上传、字节哈希、非 root、健康检查、持久卷重启恢复。
- 阶段 3 已完成 JPEG、PNG、WebP、GIF 严格检测与真实解码、默认字节保持、独立压缩与 WebP 转换、动态 GIF 保持、缩略图缓存、源/目标 SHA-256、16 MP 像素边界和无队列处理安全门。
- 阶段 3 的默认 `processingConcurrency` 为 1；浏览器多文件上传继续使用本地队列。GitHub Actions 只持续复测并发 1，不再重复执行阶段选型时的一次性 2/4/8 benchmark。
- libvips 固定为 8.18.4，运行镜像仅包含 V1 所需 JPEG、PNG、WebP 与内建 GIF 能力，不包含 Pango、SVG/PDF、ImageMagick、TIFF 或 HEIF。
- 阶段 4 已完成严格别名路径规范化、保留路由保护、映射创建/列表/解析/删除 API、直接字节交付、409 冲突语义、启动重建和别名热路径零 SQL。
- 图片、别名、Session 与 API Token 使用同一个索引变更屏障：普通写操作共享进入，完整重建独占进入，直链读取不经过屏障；并发测试证明重建不会恢复已退出 Session、已吊销 Token、旧可见性或旧别名状态。
- 10,000/100,000 路径原生 amd64/arm64 基准均通过；100,000 路径完整 SQLite Loader 低于 160 ms，Go heap 增量约 14.16 MiB，不引入 Redis、LRU 或 TTL。
- 阶段 5 已完成图片 keyset 分页、搜索、完整筛选、详情、单张/批量永久删除、批量可见性、明确逐项结果、系统概览、手动只读巡检和手动完整索引重建。
- 永久删除严格按“SQLite 级联删除 → 图片及关联别名索引移除 → 正式文件和缩略图删除”执行；文件清理失败不恢复数据库，而是返回 `cleanupPending` 并写结构化告警日志。
- React 已完成上传本地队列、进度/取消/重试、图片网格与列表、只读缩略图、详情和多格式链接复制、历史路径、系统状态、设置、深浅主题、中英文及响应式导航。
- Playwright 使用单 worker 和临时数据目录，桌面与手机闭环各只上传一张 1 × 1 WebP；空库启动实测 Go heap alloc 324,624 B、RSS 14,352,384 B、4 个 goroutine，阶段 5 没有新增运行时依赖。
- 阶段 6 已完成通用单张导入：原始字节流式写入并校验，图片与一个历史别名在同一个 SQLite 事务中提交，已存在别名会在解码前快速返回冲突且不留下图片记录、正式文件或缩略图。
- 通用 TSV 清单脚本严格串行导入，每项成功后自动访问旧 URL 并核对 SHA-256；原生 amd64/arm64 容器 smoke 都实际完成了导入、重复路径对账和旧 URL 字节校验。
- 日常维护复用现有单个维护 goroutine：启动仅清理超过 24 小时的临时文件，每日只删除超过安全时限且 SQLite 无记录的孤儿文件，删除失败保留到下次重试，数据库缺失文件只报告且不自动重建索引。
- 单张正式文件缺失不会阻止启动或 readiness；该图片及其别名不进入交付索引，系统页明确显示缺失数量和最多 100 个 Image ID。阶段 6 空库启动实测 Go heap alloc 326,864 B、RSS 14,614,528 B、4 个 goroutine。
- 阶段 7 已补齐公开交付的 Range/416/条件请求/HEAD、微型重叠读、重复生命周期临时文件检查，以及桌面 E2E 的 API Token 私密上传、Token 读取、搜索筛选、详情和永久删除。
- 容器 smoke 已验证固定 UID/GID、exec-form ENTRYPOINT、健康检查、只读 `/data` 权限错误、运行镜像精简内容、goroutine/FD/临时文件边界和两次 SIGTERM 优雅停止；最终镜像不含 Node、Go、编译器、源码、测试素材或 libvips 开发文件。
- `govulncheck` 实际调用链为 0；`golang.org/x/image` 已从存在 WebP 解码漏洞的 0.38.0 升级至 0.43.0。React Router 公告仅影响未启用的 unstable RSC API，作为可达性例外记录并在每次发布前复查。
- [Verify run 30479364047](https://github.com/Willxup/imagesilo/actions/runs/30479364047) 在提交 `b866eebcd411a38fba2365786a1fa05ed4cc443d` 上通过 quality、原生 amd64/arm64 增强容器 smoke 和每架构并发 1、16 请求 benchmark。结束匿名内存为 amd64 86,945,792 B、arm64 101,023,744 B；约 91.4 MB 文件页缓存与 16 个输出总量吻合。
- `my-geelinx` 已在 1 CPU/1 GiB 的专用 builder 中完成原生 amd64 构建，并在 0.5 CPU/256 MiB、并发 1 下连续运行 600 秒、完成 118 轮串行校验：cgroup 峰值 35,938,304 B，goroutine 10→8，FD 10→10，优雅停止通过；所有证据位于 `/var/tmp/imagesilo-v0.1.0-rc.1-phase7-20260730`。
- Git 标签 `v0.1.0-rc.1` 的 push 自动触发 [Release image run 30505989284](https://github.com/Willxup/imagesilo/actions/runs/30505989284)，在提交 `976a4a495e415de042ecb6d9758222b4ba5daf0d` 上通过质量门、原生 amd64/arm64 构建与完整容器 smoke，并成功创建多架构 OCI manifest。
- 公开镜像 `ghcr.io/willxup/imagesilo:v0.1.0-rc.1` 的 manifest digest 为 `sha256:573233d00455ab6e8dad9d875ad0ede7f770436ce24bb05f6d21bc02fac053cc`；匿名 registry 请求返回 HTTP 200，`imagetools inspect` 同时确认 `linux/amd64` 与 `linux/arm64`。[GitHub Pre-release](https://github.com/Willxup/imagesilo/releases/tag/v0.1.0-rc.1) 已发布。
- 资源基线见 `performance-baseline.md`。

阶段 1 已完成。GitHub Actions [Verify run 30447890938](https://github.com/Willxup/imagesilo/actions/runs/30447890938) 在提交 `2211fc7e7b7ae34ad9fa36ecbe8b78fe09a22268` 上通过：`quality`、原生 `ubuntu-24.04` amd64 容器闭环和原生 `ubuntu-24.04-arm` arm64 容器闭环全部成功。

阶段 2 已完成。GitHub Actions [Verify run 30450944427](https://github.com/Willxup/imagesilo/actions/runs/30450944427) 在提交 `7367185118583a10e89e078a12fb583c768733bc` 上通过：`quality`、原生 amd64 与原生 arm64 容器闭环全部成功。

阶段 3 已完成。GitHub Actions [Verify run 30463581291](https://github.com/Willxup/imagesilo/actions/runs/30463581291) 在提交 `09734182ec09486fd947a9f6e6727017ad734863` 上通过：`quality`、原生 amd64/arm64 镜像、四格式 smoke、WebP 转换和处理安全门全部成功。后续 [Verify run 30464513095](https://github.com/Willxup/imagesilo/actions/runs/30464513095) 在提交 `e6d059a755a748646ebd329de31e90635023a603` 上再次通过，并确认两个原生架构只运行默认并发 1 的持续基准。

阶段 4 已完成。GitHub Actions [Verify run 30467177771](https://github.com/Willxup/imagesilo/actions/runs/30467177771) 在提交 `6d17dfb362a60c2be85cbad38ebbe26a099790f0` 上通过：`quality`、原生 amd64/arm64 10k/100k Delivery Index 基准、容器构建、完整 smoke 和默认并发 1 图片基准全部成功。

阶段 5 已完成。GitHub Actions [Verify run 30471726635](https://github.com/Willxup/imagesilo/actions/runs/30471726635) 在提交 `4fdb303604c19ffc8889110e21c2189c3fca3130` 上通过：`quality`（含 Vitest 与单 worker 桌面/手机 Playwright）、原生 amd64/arm64 Delivery Index 基准、容器闭环和默认并发 1 的 16 请求图片基准全部成功。

阶段 6 已完成。GitHub Actions [Verify run 30474967233](https://github.com/Willxup/imagesilo/actions/runs/30474967233) 在提交 `9c0238957241e1e4f540821323bd298b291ed46d` 上通过：`quality`、原生 amd64/arm64 清单导入与历史 URL 校验、重复别名零残留、缺失正式文件重启容错、容器闭环和默认并发 1 的 16 请求图片基准全部成功。

阶段 7 已完成。GitHub Actions [Release image run 30505989284](https://github.com/Willxup/imagesilo/actions/runs/30505989284) 由 `v0.1.0-rc.1` tag push 自动触发并全部成功；公开 GHCR 多架构镜像和 [GitHub Pre-release](https://github.com/Willxup/imagesilo/releases/tag/v0.1.0-rc.1) 均已验证。

`.github/workflows/verify.yml` 和 `scripts/container-smoke.sh` 已成为后续提交的固定阶段门。脚本在本机和 GitHub 均确认成功/失败结束后不遗留临时容器或 named volume。

## 阶段门

1. 阶段 0：基础工程、依赖锁定、构建和空骨架测试。
2. 阶段 1：登录、JPEG 上传、列表、公开交付和重启恢复的最小纵向闭环。
3. 阶段 2：完整认证、API Token 和图片可见性。
4. 阶段 3：四种图片格式、libvips、缩略图和解码安全门。
5. 阶段 4：历史别名和零 SQL 交付。
6. 阶段 5：完整管理 API 与 React 界面。
7. 阶段 6：单张导入、巡检和缓存重建。
8. 阶段 7：安全、性能、双架构镜像和发布候选。
9. 阶段 8：依赖用户提供的旧系统数据副本、目标服务器与反向代理窗口，执行生产迁移和回滚演练。

前一阶段未满足退出条件时，不开始后一阶段。
