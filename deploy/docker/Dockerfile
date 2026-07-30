# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=node:26.5.0-bookworm-slim@sha256:2d49d876e96237d76de412761cf05dbfe5aee325cc4406a4d41d5824c5bb8beb
ARG GO_IMAGE=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818
ARG VIPS_VERSION=8.18.4
ARG VIPS_SHA256=2677bad6c422617fd1172d359c16af34e736965d042c214203a87187d26ff037

FROM ${NODE_IMAGE} AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY api/ /src/api/
COPY assets/ /src/assets/
RUN npm run generate:api && npm run build

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
RUN apt-get update \
    && apt-get install --yes --no-install-recommends \
        libexpat1 libglib2.0-0 libjpeg62-turbo libpng16-16 libwebp7 libwebpdemux2 libwebpmux3 zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 imagesilo \
    && useradd --uid 10001 --gid 10001 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin imagesilo \
    && install -d -o 10001 -g 10001 -m 0750 /data/db /data/images /data/migrations /data/cache/thumbnails /data/tmp

COPY --from=vips-runtime /opt/vips/lib /opt/vips/lib
COPY --from=go-build /out/imagesilo /usr/local/bin/imagesilo

ENV LD_LIBRARY_PATH=/opt/vips/lib

USER 10001:10001
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/imagesilo"]
CMD ["serve"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/usr/local/bin/imagesilo", "healthcheck"]
