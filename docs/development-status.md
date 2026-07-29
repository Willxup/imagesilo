# 开发状态与准备度

更新日期：2026-07-29

## 结论

项目可以开始开发。产品范围、数据事实来源、热路径约束、失败补偿顺序、阶段门和最终验收标准已经闭环，不存在 Go、React、SQLite 或本地文件存储层面的不可实现项。

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
- 资源基线见 `performance-baseline.md`。

阶段 1 已完成。GitHub Actions [Verify run 30447890938](https://github.com/Willxup/imagesilo/actions/runs/30447890938) 在提交 `2211fc7e7b7ae34ad9fa36ecbe8b78fe09a22268` 上通过：`quality`、原生 `ubuntu-24.04` amd64 容器闭环和原生 `ubuntu-24.04-arm` arm64 容器闭环全部成功。

阶段 2 已完成。GitHub Actions [Verify run 30450944427](https://github.com/Willxup/imagesilo/actions/runs/30450944427) 在提交 `7367185118583a10e89e078a12fb583c768733bc` 上通过：`quality`、原生 amd64 与原生 arm64 容器闭环全部成功。

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
