# 开发状态与准备度

更新日期：2026-07-29

## 结论

项目可以开始开发。产品范围、数据事实来源、热路径约束、失败补偿顺序、阶段门和最终验收标准已经闭环，不存在 Go、React、SQLite 或本地文件存储层面的不可实现项。

仓库开始时为空，远程仓库为 `git@github.com:Willxup/imagesilo.git`，因此 Go module 使用 `github.com/Willxup/imagesilo`。

## 当前环境证据

- 本机为 macOS arm64，Go、Node、Clang 和 Docker buildx CLI 可用。
- 本机 Docker daemon 尚未启动，因此双架构容器构建与运行 smoke test 必须在 daemon 可用后执行。
- 本机未安装 libvips；阶段 3 的处理器以 Docker 中锁定的 libvips 8.18.4 为可复现构建边界，不把开发机全局库当作事实来源。
- arm64 本机只能证明 arm64 原生运行；amd64 可以先做 QEMU 最低验证，发布门仍要求 amd64 真机 smoke test。

## 已完成

- 阶段 0：依赖、工具链和 Docker 基础镜像 digest 已锁定；Go、React、OpenAPI、SQLite 迁移、Makefile 和 Compose 基础工程通过完整检查。
- 阶段 1 的代码与本机闭环：无回显管理员创建、Argon2id、Session 哈希内存索引、流式 JPEG 上传、原子落盘、失败补偿、图片列表、内存交付索引、Range/ETag、React 登录/上传/列表、重启恢复全部完成。
- arm64 原生容器闭环通过；amd64 OrbStack 模拟容器闭环通过。
- 资源基线见 `performance-baseline.md`。

阶段 1 仍缺发布基线要求的 amd64 真机容器 smoke test。没有可用的原生 amd64 Docker context 或 runner 时，不把模拟结果伪装成真机结果，也不越过阶段门进入阶段 2。

仓库已准备 `.github/workflows/verify.yml` 和 `scripts/container-smoke.sh`：一旦代码进入 GitHub，官方 `ubuntu-24.04` 与 `ubuntu-24.04-arm` runner 会执行同一套原生双架构闭环。当前 GitHub 仓库为公开空仓库，但本机 `gh` 令牌无效；未获得用户授权前不创建提交或推送。

该脚本已在本机 arm64 原生和 amd64 OrbStack 模拟环境分别通过，并验证失败/成功路径结束后均不遗留临时容器或 named volume。剩余差异只有执行节点是否为原生 amd64。

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
