# 自动化验证

`.github/workflows/verify.yml` 使用固定 commit SHA 的官方 Actions：

- `actions/checkout` 7.0.1
- `actions/setup-go` 7.0.0
- `actions/setup-node` 7.0.0

`quality` job 执行与本地一致的 `make check`。`container-smoke` 分别在 GitHub 官方 `ubuntu-24.04`（amd64）和 `ubuntu-24.04-arm`（arm64）原生 runner 上执行：

1. 构建目标架构镜像。
2. 通过 `--password-stdin` 创建临时管理员，密码不进入进程参数。
3. 登录并取得 Session Cookie。
4. 流式上传确定性 JPEG。
5. 核对公开 URL 的 SHA-256、非 root 用户和健康检查。
6. 停止容器、使用同一 named volume 重启并再次核对 URL。
7. 无论成功或失败都删除临时容器、volume、Cookie 和响应文件。

本地可使用相同脚本：

```bash
IMAGE=imagesilo:smoke \
PLATFORM=linux/arm64 \
PORT=18080 \
SMOKE_SUFFIX=local-arm64 \
bash scripts/container-smoke.sh
```

首次原生双架构运行证据为 [Verify run 30447890938](https://github.com/Willxup/imagesilo/actions/runs/30447890938)：质量门、amd64 容器闭环和 arm64 容器闭环全部成功。Git 推送通过仓库 SSH 远程完成；后续正常开发提交按相同工作流持续验证。
