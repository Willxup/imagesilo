# 自动化验证

## 快速校验

`.github/workflows/verify.yml` 是 PR、`main` push 和手动运行使用的快速门禁，并使用固定 commit SHA 的官方 Actions：

- `actions/checkout` 7.0.1
- `actions/setup-go` 7.0.0
- `actions/setup-node` 7.0.0
- `docker/login-action` 4.6.0（仅 Release）

以上 JavaScript Actions 均声明使用 Node.js 24 runtime（由 Action 自身提供，不受项目 setup-node 配置控制）。项目工具链由 `actions/setup-node` 选择 Node.js `24.x`、`actions/setup-go` 选择 Go `1.26.x`，均启用 `check-latest: true`，每次检查该系列的最新版本。本地不使用精确版本文件：Go 最低要求由 `go.mod` 指定为 1.26.10，Node.js 最低要求由 `web/package.json` 指定为 >=24.22.1，`web/.npmrc` 启用 `engine-strict=true`，安装时拒绝不满足要求的版本。

唯一的 `quality` job 执行 `make check`，覆盖 Go、React、前端构建、OpenAPI 生成一致性、Lint、类型检查和单元/集成测试。界面或浏览器行为变更按需使用现有浏览器人工或通过 computer-use 工具验收，见[浏览器验收清单](./browser-acceptance.md)；CI 不安装专用浏览器，也不执行浏览器验收。该工作流不构建容器镜像、不运行容器 smoke，也不接触 GHCR；`quality` check 名称保持不变。

## Release smoke

`.github/workflows/release.yml` 只接受属于 `origin/main` 的 `v*` Tag。Release 重新运行质量门后，分别在 GitHub 官方 `ubuntu-24.04`（amd64）和 `ubuntu-24.04-arm`（arm64）原生 runner 上执行：

1. 执行原生 Delivery Index benchmark。
2. 使用 `docker build --pull` 构建目标架构镜像，刷新 Go、Node.js 和 Debian Bookworm 滚动基础镜像。已发布的不可变镜像保持不变；安全更新必须通过新的发布版本构建并部署。
3. 通过 `--password-stdin` 创建临时管理员，密码不进入进程参数。
4. 登录并取得 Session Cookie。
5. 流式上传确定性 JPEG。
6. 核对公开 URL 的 SHA-256、非 root 用户和健康检查。
7. 停止容器、使用同一 named volume 重启并再次核对 URL。
8. 对同一镜像执行并发 `1` 的原生图片处理 benchmark。
9. 无论成功或失败都删除临时容器、volume、Cookie 和响应文件。
10. 只有该架构全部 smoke/benchmark 成功后才推送对应的不可变平台镜像；只有两个架构都成功后才创建版本与 `latest` manifest。

本地可使用相同脚本：

```bash
IMAGE=imagesilo:smoke \
PLATFORM=linux/arm64 \
PORT=18080 \
SMOKE_SUFFIX=local-arm64 \
bash scripts/container-smoke.sh
```

首次原生双架构运行证据为 [Verify run 30447890938](https://github.com/Willxup/imagesilo/actions/runs/30447890938)：质量门、amd64 容器闭环和 arm64 容器闭环全部成功。该记录保留为历史证据；当前开发提交只运行快速校验，双架构容器 smoke 已收敛到 Release Tag 流程。
