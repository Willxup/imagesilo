# ImageSilo

ImageSilo 是一个单进程、Docker 优先的自托管图床。V1 使用 Go、React、TypeScript、SQLite 和本地文件存储，目标是在保持公开 URL 稳定的同时，安全地完成图片上传、交付与管理。

项目当前按照 [`docs/development-status.md`](docs/development-status.md) 记录的阶段门推进，产品与架构基线来自 `ImageSilo-可执行开发计划.md` 1.9。

## 本地验证

需要 Go 1.26.5 和 Node.js 26.5.0。首次检出后执行：

```bash
npm --prefix web ci
make check
make build
```

本地运行时必须显式指定可写数据目录：

```bash
IMAGESILO_DATA_DIR="$PWD/data" IMAGESILO_COOKIE_SECURE=false ./bin/imagesilo serve
```

访问 `/healthz` 检查进程存活，访问 `/readyz` 检查 SQLite 是否就绪。

首次创建管理员时默认从真实终端无回显读取并确认密码：

```bash
IMAGESILO_DATA_DIR="$PWD/data" ./bin/imagesilo admin create --email admin@example.com
```

CI 只能显式使用 `--password-stdin`，密码不会出现在进程参数中：

```bash
printf '%s\n' "$CI_SMOKE_PASSWORD" | IMAGESILO_DATA_DIR="$PWD/data" ./bin/imagesilo admin create --email admin@example.com --password-stdin
```

管理后台入口为 `/admin/login`。登录后可以上传公开/私密图片、修改现有图片的可见性、设置默认可见性、修改管理员密码，以及创建和吊销具名 API Token。

API Token 的 curl、PicGo 与 ShareX 配置见 [`docs/api-token-usage.md`](docs/api-token-usage.md)。HTTP 契约的唯一事实来源为 [`api/openapi.yaml`](api/openapi.yaml)。

## 部署边界

V1 唯一支持的生产部署方式是 Docker Engine + Docker Compose。`/data` 必须使用 Docker 本地 named volume 或宿主机本地文件系统 bind mount；不支持 NFS 或 SMB。
