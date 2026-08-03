ARG BUILD_FROM=alpine:3.21

# Build stage always runs on the build host's platform; the Go
# cross-compile below targets BUILD_ARCH.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG BUILD_ARCH=amd64
ARG BUILD_VERSION=dev

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ .
RUN set -e; \
    case "${BUILD_ARCH}" in \
      aarch64) export GOARCH=arm64 ;; \
      armv7)   export GOARCH=arm GOARM=7 ;; \
      *)       export GOARCH=amd64 ;; \
    esac; \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags "-s -w -X main.version=${BUILD_VERSION}" \
      -o /out/bambu-notifier ./cmd/bambu-notifier

FROM ${BUILD_FROM}

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/bambu-notifier /usr/local/bin/bambu-notifier

ENTRYPOINT ["/usr/local/bin/bambu-notifier"]
CMD ["--config", "/data/options.json"]
