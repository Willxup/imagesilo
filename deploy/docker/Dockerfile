# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=node:26.5.0-bookworm-slim@sha256:2d49d876e96237d76de412761cf05dbfe5aee325cc4406a4d41d5824c5bb8beb
ARG GO_IMAGE=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818

FROM ${NODE_IMAGE} AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY api/ /src/api/
RUN npm run generate:api && npm run build

FROM ${GO_IMAGE} AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-build /src/web/dist ./internal/webui/dist
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/imagesilo ./cmd/imagesilo

FROM ${RUNTIME_IMAGE} AS runtime
RUN groupadd --gid 10001 imagesilo \
    && useradd --uid 10001 --gid 10001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin imagesilo \
    && install -d -o 10001 -g 10001 -m 0750 /data/db /data/images /data/cache/thumbnails /data/tmp

COPY --from=go-build /out/imagesilo /usr/local/bin/imagesilo

USER 10001:10001
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/imagesilo"]
CMD ["serve"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/usr/local/bin/imagesilo", "healthcheck"]
