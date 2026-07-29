# Docker 部署、升级、备份与回滚

ImageSilo 的生产边界始终是一个容器、一个 Go 进程、一个本地 `/data`。不需要 Redis、外部数据库、任务队列或第二个应用服务。图片处理并发默认且推荐保持为 `1`。

## 首次部署

固定使用完整版本标签，不要在生产环境跟随可变的 `latest`。示例中的标签仅作占位：

```bash
export IMAGESILO_IMAGE=ghcr.io/willxup/imagesilo:v1.0.0-rc.1
docker pull "$IMAGESILO_IMAGE"
docker volume create imagesilo-data
printf '%s\n' "$IMAGESILO_ADMIN_PASSWORD" | docker run --rm --interactive \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE" admin create --email admin@example.com --password-stdin
docker run --detach \
  --name imagesilo \
  --restart unless-stopped \
  --publish 127.0.0.1:8080:8080 \
  --env IMAGESILO_PROCESSING_CONCURRENCY=1 \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE"
```

确认 `/healthz`、`/readyz`、管理员登录和一张微型测试图片后，再把反向代理指向 `127.0.0.1:8080`。TLS、请求体上限和公网限速应由现有反向代理承担；ImageSilo 自身仍保持单进程。

## bind mount

宿主机目录必须位于本地文件系统，不支持 NFS 或 SMB。容器固定以数字 UID/GID `10001:10001` 运行：

```bash
sudo install -d -o 10001 -g 10001 -m 0750 \
  /srv/imagesilo/data/db \
  /srv/imagesilo/data/images \
  /srv/imagesilo/data/cache/thumbnails \
  /srv/imagesilo/data/tmp
```

将 `--volume imagesilo-data:/data` 替换为 `--volume /srv/imagesilo/data:/data`。权限错误会在启动阶段直接失败，不会退回 root 运行。

## 一致备份

ImageSilo 不在应用进程中实现备份。备份必须在停止写入后覆盖完整 `/data`：

1. 停止反向代理写流量。
2. `docker stop --time 20 imagesilo`，确认退出码为 `0`。
3. 复制 named volume 对应目录，或对 bind mount 所在本地文件系统创建快照。
4. 同时记录当前不可变镜像标签和 digest。
5. 启动原容器，复核 readiness、登录、标准 URL 和历史别名。

不要只复制 SQLite 主文件；WAL、图片文件和数据库必须来自同一个停止写入时点。

## 升级

1. 先完成完整 `/data` 备份，并保留旧镜像标签和 digest。
2. 拉取新版本镜像，在独立临时端口和独立数据副本上完成 smoke test。
3. 停止正式容器，使用新镜像和原 `/data` 启动。
4. 验证 `/readyz`、管理员登录、公开/私密图片、上传和历史别名。
5. 观察结构化日志、RSS、goroutine、文件描述符和磁盘占用。

## 回滚

数据库迁移是向前执行的，因此不能假设旧二进制一定能读取升级后的数据库。可靠回滚是：

1. 停止新容器。
2. 恢复升级前的完整 `/data` 备份或快照。
3. 使用记录过 digest 的旧镜像启动。
4. 复核 readiness、图片字节哈希和历史别名。

不要让新旧两个容器同时写同一个 `/data`。
