# 阶段 1 性能与资源基线

测量日期：2026-07-29  
主机：macOS arm64  
工具链：Go 1.26.5，Node.js 构建版本 26.5.0  
服务：本机原生 `imagesilo serve`，SQLite WAL，本地 APFS 数据目录

## 固定样本

`tests/performance/jpeg_stdout.go` 生成 6000 × 6000、36 MP、JPEG quality 90 的确定性渐变图。上传字节为 2,416,159，源文件和正式文件 SHA-256 均为 `93415bb62272e26a322efb2eaa4c3ad8e72fe391fe82af80e05b17ce9fcdb1dd`。

## 结果

| 场景 | Go heap alloc | Go heap sys | Goroutine | 进程 RSS |
|---|---:|---:|---:|---:|
| 启动完成、加载 4 张图片和活跃 Session | 313,624 B | 7,995,392 B | 3 | 约 13.6 MiB（空数据进程测量） |
| 一次 36 MP JPEG 上传完成 | 54,387,656 B | 62,259,200 B | 7 | 67.7 MiB |

早期 64 MiB Argon2id 参数导致一次登录后 RSS 约 144 MiB。根据实测改为 Argon2id 19 MiB、2 次迭代、单线程，并把缺失账号使用的 dummy hash 预计算后，登录前 RSS 为 13.6 MiB，登录后为 34.5 MiB。

连续三次 36 MP 上传的 RSS 观测为约 86.9 MiB、138.8 MiB、138.9 MiB：Go 堆在第二次扩大后形成高水位，第三次没有继续线性增长。阶段 1 未观察到按请求持续增长的迹象；阶段 3 切换 libvips 后必须重新分别测量 Go heap、RSS 和原生内存，不能沿用本基线推断生产并发值。

## 容器验证

- `linux/arm64`：原生构建和运行；管理员登录、SQLite 写入、6 MP JPEG 上传、公开直链、SHA-256、停止与 named volume 重启恢复均通过。
- `linux/amd64`：本机 OrbStack 模拟闭环通过；GitHub 官方原生 `ubuntu-24.04` runner 的构建、登录、上传、直链、非 root、健康检查和重启恢复也已通过。
- `linux/arm64`：本机原生闭环通过；GitHub 官方原生 `ubuntu-24.04-arm` runner 的同一闭环也已通过。
- 证据为 GitHub Actions [Verify run 30447890938](https://github.com/Willxup/imagesilo/actions/runs/30447890938)，对应提交 `2211fc7e7b7ae34ad9fa36ecbe8b78fe09a22268`。
- `scripts/container-smoke.sh` 在本机和 GitHub 均确认没有遗留容器或 volume。

当前阶段不根据这些数字设置 Docker CPU/内存配额，也不调用强制 GC。图片解码并发值仍保持保守默认 1，等待阶段 3 在 amd64/arm64 真机上完成 1、2、4、8 并发 benchmark。
