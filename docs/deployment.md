# Docker 部署、升级、备份与回滚

ImageSilo 的生产边界始终是一个容器、一个 Go 进程、一个本地 `/data`。不需要 Redis、外部数据库、任务队列或第二个应用服务。图片处理并发默认且推荐保持为 `1`。

## 首次部署

每次成功发布都会同时更新完整版本标签和可变的 `latest`。生产环境仍建议固定使用完整版本标签或 digest；`latest` 更适合希望自动跟随最新发布的测试环境。示例中的标签仅作占位：

```bash
export IMAGESILO_IMAGE=ghcr.io/willxup/imagesilo:v1.0.0-rc.1
docker pull "$IMAGESILO_IMAGE"
docker volume create imagesilo-data
docker run --detach \
  --name imagesilo \
  --restart unless-stopped \
  --publish 127.0.0.1:8080:8080 \
  --env IMAGESILO_PROCESSING_CONCURRENCY=1 \
	--env IMAGESILO_DELIVERY_CONCURRENCY=0 \
	--env IMAGESILO_TRUST_PROXY_HEADERS=true \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE"

docker logs -f imagesilo
```

仅在尚无管理员时，启动日志会输出一次 `bootstrap_token`。打开 `/admin/setup` 填入该 token 后设置管理员账号和初始策略；token 只保存于当前进程内存，初始化成功后立即清除，未初始化重启则重新生成。无人值守部署可改用 `imagesilo admin create --password-stdin`，创建管理员后 Web 初始化会自动关闭。

确认 `/healthz`、`/readyz`、管理员登录和一张微型测试图片后，再把反向代理指向 `127.0.0.1:8080`。TLS、请求体上限和公网限速应由现有反向代理承担；ImageSilo 自身仍保持单进程。

推荐拓扑是 Nginx Proxy Manager → `127.0.0.1:8080`。ImageSilo 默认信任 Nginx 的客户端地址头，并按单个有效 `X-Real-IP`、`X-Forwarded-For` 最右侧有效地址、TCP 对端地址的顺序用于登录 IP 限速。因此后端端口不得绕过 NPM 直接暴露公网；确需直连部署时显式设置 `IMAGESILO_TRUST_PROXY_HEADERS=false`。NPM 可以长期缓存带哈希的前端静态资源，但不要给可见性和内容都可能变化的图片 URL 设置长 TTL；公开图片由 `ETag`/`304` 复验，私密图片使用 `private, no-store`。

`IMAGESILO_DELIVERY_CONCURRENCY` 默认 `0`，表示不限制同时读取数量，由 Nginx、操作系统和文件系统负责连接与页缓存调度。配置为 `1`—`4096` 时，应用会对标准 URL、历史别名、迁移目录和缩略图统一限流；满载时立即返回 `503` 与 `Retry-After`。图片正文不进入 Go 全图缓存，读取依靠内存元数据索引、文件流、ETag 和操作系统页缓存，保持轻量。

## bind mount

宿主机目录必须位于本地文件系统，不支持 NFS 或 SMB。容器固定以数字 UID/GID `10001:10001` 运行：

```bash
sudo install -d -o 10001 -g 10001 -m 0750 \
  /srv/imagesilo/data/db \
  /srv/imagesilo/data/images \
  /srv/imagesilo/data/migrations \
  /srv/imagesilo/data/cache/thumbnails \
  /srv/imagesilo/data/tmp
```

将 `--volume imagesilo-data:/data` 替换为 `--volume /srv/imagesilo/data:/data`。权限错误会在启动阶段直接失败，不会退回 root 运行。

需要保留旧图床目录结构时，可以再把旧图片根目录只读挂载到 `/data/migrations`，例如 `--volume /srv/legacy-images:/data/migrations:ro`。挂载目录中的相对路径会直接成为公开 URL，后台迁移管理仍可浏览和复制直链。

只有需要从后台永久删除“专用迁移副本”时，才同时使用可写挂载、确保 UID/GID `10001:10001` 拥有删除权限，并增加 `--env IMAGESILO_MIGRATION_MUTATIONS=true`。该开关默认 `false`；仅改为可写挂载不会自动开放删除。不要对旧图床唯一原件启用此能力，具体边界见[导入指南](./imports.md)。

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
