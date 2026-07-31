<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./assets/brand/imagesilo-logo-dark.svg" />
    <source media="(prefers-color-scheme: light)" srcset="./assets/brand/imagesilo-logo-card.png" />
    <img src="./assets/brand/imagesilo-logo-card.png" alt="ImageSilo" width="560" />
  </picture>
</p>

<p align="center">
  <a href="./README.md"><strong>English</strong></a> ｜ <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">ImageSilo</h1>

<p align="center">A lightweight, self-hosted home for your images.</p>

<p align="center">
  <a href="https://github.com/Willxup/imagesilo/releases"><img src="https://img.shields.io/github/v/release/Willxup/imagesilo?include_prereleases&amp;style=flat-square" alt="Latest release" /></a>
  <a href="https://github.com/Willxup/imagesilo/actions/workflows/verify.yml"><img src="https://img.shields.io/github/actions/workflow/status/Willxup/imagesilo/verify.yml?branch=main&amp;style=flat-square&amp;label=CI" alt="CI status" /></a>
  <a href="https://github.com/Willxup/imagesilo/pkgs/container/imagesilo"><img src="https://img.shields.io/badge/Docker-GHCR-2496ED?style=flat-square&amp;logo=docker&amp;logoColor=white" alt="Docker image on GHCR" /></a>
  <img src="https://img.shields.io/badge/Linux-amd64%20%7C%20arm64-FCC624?style=flat-square&amp;logo=linux&amp;logoColor=black" alt="Linux amd64 and arm64" />
</p>

ImageSilo is a Docker-first image host built as one Go process with SQLite and local file storage. It provides authenticated uploads, stable public URLs, historical aliases, scoped API tokens, image processing, and a responsive React administration interface without Redis, an external database, or a background job service.

> The current release is `v0.1.0-rc.3`. Every successful release also updates the mutable `latest` tag. Production deployments should still pin an immutable version tag or digest.

## Features

- Store JPEG, PNG, WebP, and GIF images with strict decoding and a 16 MP safety limit
- Preserve original bytes or explicitly compress and convert static images to WebP
- Deliver public images, historical aliases, and path-preserving migration files with Range, conditional request, and HEAD support
- Keep the delivery hot path in memory, with no SQLite lookup per image request
- Manage public/private visibility, administrator sessions, CSRF protection, login rate limits, and scoped API tokens
- Upload, search, filter, inspect, batch-update, and permanently delete images from the built-in web interface
- Import legacy URLs, verify bytes by SHA-256, inspect storage, and rebuild in-memory indexes
- Run as a non-root, multi-architecture container with image processing concurrency set to `1` by default

## Quick Start

Docker is the supported production runtime. The following starts a local evaluation instance with a named volume:

```bash
export IMAGESILO_IMAGE=ghcr.io/willxup/imagesilo:v0.1.0-rc.3

docker pull "$IMAGESILO_IMAGE"
docker volume create imagesilo-data

docker run --detach \
  --name imagesilo \
  --restart unless-stopped \
  --publish 127.0.0.1:8080:8080 \
  --env IMAGESILO_COOKIE_SECURE=false \
  --env IMAGESILO_PROCESSING_CONCURRENCY=1 \
  --volume imagesilo-data:/data \
  "$IMAGESILO_IMAGE"
```

On the first launch, run `docker logs -f imagesilo` and copy the `bootstrap_token` printed by ImageSilo. Open `http://127.0.0.1:8080/admin/setup`, enter that token, and create the administrator account. The token exists only in memory, is replaced after an uninitialized restart, and is erased after setup succeeds. The `imagesilo admin create` command remains available for scripted deployments.

For production, terminate HTTPS at a reverse proxy, keep `IMAGESILO_COOKIE_SECURE=true`, and back up the complete `/data` directory while writes are stopped. See the [deployment guide](./docs/deployment.md) before exposing the service.

To preserve an existing image URL tree without creating one alias at a time, mount it read-only at `/data/migrations`. A file mounted as `/data/migrations/i/2022/04/example.jpg` is immediately available at `/i/2022/04/example.jpg`. Managed historical aliases take precedence when both sources contain the same path; only JPEG, PNG, WebP, and GIF files are exposed.

## Design Boundaries

| Area | ImageSilo V1 |
| --- | --- |
| Runtime | One Go process in one container |
| Metadata | SQLite in `/data/db` |
| Images | Local files in `/data/images` |
| Path-preserving migrations | Read-only image tree in `/data/migrations` |
| Cache | Local thumbnails in `/data/cache` |
| Processing | libvips, concurrency `1` by default |
| Delivery | File streaming with ETag revalidation; unlimited by default (`0`), optionally capped |
| Platforms | `linux/amd64`, `linux/arm64` |
| Production storage | Docker named volume or local bind mount |

NFS and SMB are not supported. Do not run two ImageSilo containers against the same writable `/data` directory.

## Local Development

### Prerequisites

- Go 1.26.5
- Node.js 26.5.0
- npm

### Build and Verify

```bash
npm --prefix web ci
make check
make build
```

Create an administrator and run the local binary:

```bash
IMAGESILO_DATA_DIR="$PWD/data" \
  ./bin/imagesilo admin create --email admin@example.com

IMAGESILO_DATA_DIR="$PWD/data" \
IMAGESILO_COOKIE_SECURE=false \
  ./bin/imagesilo serve
```

Use `/healthz` for liveness and `/readyz` for SQLite readiness. Run `make e2e` for the single-worker browser workflow.

## Documentation

| Topic | Guide |
| --- | --- |
| Docker deployment, backup, upgrade, and rollback | [Deployment](./docs/deployment.md) |
| API tokens, curl, PicGo, and ShareX | [API token usage](./docs/api-token-usage.md) |
| Legacy image and URL migration | [Imports](./docs/imports.md) |
| Inspection, cleanup, and index rebuilds | [Operations](./docs/operations.md) |
| Architecture and data model | [Architecture](./docs/architecture.md) · [Data model](./docs/data-model.md) |
| Security and lightweight resource evidence | [Security audit](./docs/security-audit.md) · [Performance baseline](./docs/performance-baseline.md) |
| Release images and acceptance checks | [Release guide](./docs/release.md) |
| HTTP API contract | [OpenAPI](./api/openapi.yaml) |
| Bundled third-party licenses | [Third-party notices](./THIRD_PARTY_NOTICES.md) |

## Project Status

Stages 0–7 are complete, including native amd64/arm64 verification and the public `v0.1.0-rc.3` release candidate. Production migration remains a separate, environment-specific step. See [development status](./docs/development-status.md) for the current evidence and remaining work.
