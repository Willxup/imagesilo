ARG NODE_IMAGE=node:24.19.0-bookworm-slim@sha256:3638d9a6fe4030bd716be989438248074489337ba3275657f93595428be4fc03
ARG GO_IMAGE=golang:1.26.5-bookworm@sha256:6c5605ab3a9a9fb3c4eafe5b3d63cdbf3881caf113262b67862547b54a9db599
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818
ARG VIPS_VERSION=8.18.4
ARG VIPS_SHA256=2677bad6c422617fd1172d359c16af34e736965d042c214203a87187d26ff037
ARG VERSION=dev
ARG BUILD_REVISION=unknown
ARG BUILD_SOURCE=https://github.com/Willxup/imagesilo

FROM ${NODE_IMAGE} AS web-build
ARG VERSION
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY api/ /src/api/
COPY assets/ /src/assets/
RUN npm run generate:api && VITE_APP_VERSION="${VERSION}" npm run build

FROM ${GO_IMAGE} AS vips-build
ARG VIPS_VERSION
ARG VIPS_SHA256
RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
        build-essential ca-certificates curl libexpat1-dev libglib2.0-dev \
        libjpeg62-turbo-dev libpng-dev libwebp-dev meson ninja-build pkg-config xz-utils zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /tmp/libvips
RUN curl --location --fail --silent --show-error \
        --output vips.tar.xz \
        "https://github.com/libvips/libvips/releases/download/v${VIPS_VERSION}/vips-${VIPS_VERSION}.tar.xz" \
    && echo "${VIPS_SHA256}  vips.tar.xz" | sha256sum --check --strict \
    && tar --extract --file vips.tar.xz --strip-components=1 \
    && meson setup build \
        --prefix=/opt/vips \
        --libdir=lib \
        --buildtype=release \
        -Ddeprecated=false -Dexamples=false -Dcplusplus=false -Ddocs=false \
        -Dmodules=disabled -Dintrospection=disabled -Dvapi=false \
        -Dcfitsio=disabled -Dcgif=disabled -Dexif=disabled -Dfftw=disabled \
        -Dfontconfig=disabled -Darchive=disabled -Dheif=disabled -Dheif-module=disabled \
        -Dimagequant=disabled -Djpeg=enabled -Duhdr=disabled -Djpeg-xl=disabled \
        -Djpeg-xl-module=disabled -Dlcms=disabled -Dmagick=disabled -Dmagick-module=disabled \
        -Dmatio=disabled -Dnifti=disabled -Dopenexr=disabled -Dopenjpeg=disabled \
        -Dopenslide=disabled -Dopenslide-module=disabled -Dhighway=disabled -Dorc=disabled \
        -Dpangocairo=disabled -Dpdfium=disabled -Dpng=enabled -Dpoppler=disabled \
        -Dpoppler-module=disabled -Dquantizr=disabled -Draw=disabled -Drsvg=disabled \
        -Dspng=disabled -Dtiff=disabled -Dwebp=enabled -Dzlib=enabled \
        -Dnsgif=true -Dppm=false -Danalyze=false -Dradiance=false \
    && meson compile -C build \
    && meson install -C build

FROM vips-build AS go-build
ENV PKG_CONFIG_PATH=/opt/vips/lib/pkgconfig
ENV CGO_CFLAGS=-I/opt/vips/include
ENV CGO_LDFLAGS=-L/opt/vips/lib
ENV LD_LIBRARY_PATH=/opt/vips/lib
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-build /src/web/dist ./internal/webui/dist
RUN CGO_ENABLED=1 go test -tags vips ./internal/platform/processor ./internal/image ./internal/httpapi \
    && CGO_ENABLED=1 go build -tags vips -trimpath -ldflags='-s -w' -o /out/imagesilo ./cmd/imagesilo

FROM vips-build AS vips-runtime
RUN rm -rf /opt/vips/bin /opt/vips/include /opt/vips/lib/pkgconfig /opt/vips/share \
    && find /opt/vips/lib -type f \( -name '*.a' -o -name '*.la' \) -delete \
    && find /opt/vips/lib -type f -name '*.so.*' -exec strip --strip-unneeded {} +

FROM ${RUNTIME_IMAGE} AS runtime
ARG BUILD_REVISION
ARG BUILD_SOURCE
RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
        libexpat1 libglib2.0-0 libjpeg62-turbo libpng16-16 libwebp7 libwebpdemux2 libwebpmux3 zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 imagesilo \
    && useradd --uid 10001 --gid 10001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin imagesilo \
    && install -d -o 10001 -g 10001 -m 0750 /data/db /data/images /data/migrations /data/cache/thumbnails /data/tmp

COPY --from=vips-runtime /opt/vips/lib /opt/vips/lib
COPY --from=go-build /out/imagesilo /usr/local/bin/imagesilo
COPY THIRD_PARTY_NOTICES.md /usr/share/doc/imagesilo/THIRD_PARTY_NOTICES.md

LABEL org.opencontainers.image.source="$BUILD_SOURCE" \
      org.opencontainers.image.revision="$BUILD_REVISION"

ENV LD_LIBRARY_PATH=/opt/vips/lib \
    IMAGESILO_LISTEN_ADDRESS=:8080 \
    IMAGESILO_DELIVERY_CONCURRENCY=0 \
    IMAGESILO_TRUST_PROXY_HEADERS=true

USER 10001:10001
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/imagesilo"]
CMD ["serve"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/usr/local/bin/imagesilo", "healthcheck"]
