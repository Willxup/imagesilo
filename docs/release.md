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

远程验收严格使用 `/var/tmp` 独立目录，默认只占 `0.5` CPU、`256m` 内存、`128` PID，处理并发固定为 `1`。默认连续运行 10 分钟，所有请求串行：

```bash
ssh my-geelinx
cd /var/tmp/imagesilo-acceptance-src
IMAGE=ghcr.io/willxup/imagesilo:v1.0.0-rc.1 \
WORK_DIR=/var/tmp/imagesilo-acceptance-v1.0.0-rc.1 \
CPU_LIMIT=0.5 MEMORY_LIMIT=256m DURATION_SECONDS=600 \
bash scripts/remote-release-acceptance.sh
```

脚本不会删除 `WORK_DIR`。结果、日志、测试图片、SQLite 和正式文件全部归拢在该目录；只会移除临时容器。验收记录包含 cgroup 当前/峰值内存、goroutine、文件描述符和实际串行轮次。
