# 发布候选与受限资源验收

## 质量门

`make check e2e` 覆盖 Go、React、OpenAPI 生成一致性和单 worker 浏览器闭环。`Verify` 工作流随后在原生 amd64、原生 arm64 各构建一次镜像、执行容器 smoke，并只运行并发 `1`、总计 `16` 个请求的图片处理 benchmark。

容器 smoke 还会验证：

- 公开标准 URL 和别名的 Range、条件请求、HEAD 与相同 ETag。
- 固定 `10001:10001`、exec-form ENTRYPOINT 和内置健康检查。
- 只读 `/data` 能给出明确权限错误。
- 运行镜像不存在 Node、Go、编译器、源码、测试素材和 libvips 开发文件。
- 少量串行读、巡检和索引重建后，临时文件为零，goroutine 与文件描述符没有持续增长。
- SIGTERM 在超时内以退出码 `0` 完成。

## 多架构 OCI manifest

推送符合语义版本格式的 Git tag，或手动运行 `Release image` 并输入同类标签，例如 `v1.0.0-rc.1`。工作流在两个原生 runner 分别构建和 smoke，推送平台标签，最后创建同一版本的 OCI manifest；不会自动覆盖 `latest`。

发布后再次确认：

```bash
docker buildx imagetools inspect ghcr.io/willxup/imagesilo:v1.0.0-rc.1
```

输出必须同时包含 `linux/amd64` 和 `linux/arm64`。

## my-geelinx 受限验收

远程验收严格使用 `/var/tmp` 独立目录，默认只占 `0.5` CPU、`256m` 内存、`128` PID，处理并发固定为 `1`。脚本只要求 Docker、curl、Python 3、base64 和系统哈希工具，不要求安装 jq。默认连续运行 10 分钟，所有请求串行：

```bash
ssh my-geelinx
cd /var/tmp/imagesilo-acceptance-src
IMAGE=ghcr.io/willxup/imagesilo:v1.0.0-rc.1 \
WORK_DIR=/var/tmp/imagesilo-acceptance-v1.0.0-rc.1 \
CPU_LIMIT=0.5 MEMORY_LIMIT=256m DURATION_SECONDS=600 \
bash scripts/remote-release-acceptance.sh
```

脚本不会删除 `WORK_DIR`。结果、日志、测试图片、SQLite 和正式文件全部归拢在该目录；只会移除临时容器。验收记录包含 cgroup 当前/峰值内存、goroutine、文件描述符和实际串行轮次。

### v0.1.0-rc.1 验收记录

提交 `b866eebcd411a38fba2365786a1fa05ed4cc443d` 已在 `my-geelinx` 原生 amd64 上完成：

- 构建器限制为 CPU 0、1 CPU quota、1 GiB 内存；镜像内 vips 测试通过后删除 builder。
- 候选镜像大小 `94,845,421` 字节，固定 `10001:10001`、exec-form ENTRYPOINT 和内置健康检查。
- 运行限制为 0.5 CPU、256 MiB、128 PID、处理并发 1，连续 600 秒完成 118 轮串行校验。
- cgroup 当前内存 `14,135,296` 字节、峰值 `35,938,304` 字节；goroutine `10 → 8`、FD `10 → 10`。
- 启动 RSS `20,402,176` 字节；上传后概览 RSS `43,679,744` 字节，结束 RSS `26,570,752` 字节。
- SIGTERM 正常退出，测试容器、builder 和临时候选镜像已删除。
- 源代码、构建元数据、失败前置检查和最终验收结果保留在 `/var/tmp/imagesilo-v0.1.0-rc.1-phase7-20260730`，总计约 2.0 MiB。

Git 标签 `v0.1.0-rc.1` 当前仅存在于本地；GHCR 外部包发布必须在得到明确授权后执行，不得用其他分发方式绕过。
