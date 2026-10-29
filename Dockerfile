# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOOS
ARG GOARCH

ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /src

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    os="${GOOS:-$TARGETOS}"; \
    arch="${GOARCH:-$TARGETARCH}"; \
    extension=""; \
    if [ "$os" = "windows" ]; then \
        extension=".exe"; \
    fi; \
    CGO_ENABLED=0 \
    GOOS="$os" \
    GOARCH="$arch" \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o "/out/protoc-gen-frankenphp-go-grpc${extension}" \
        ./cmd/protoc-gen-frankenphp-go-grpc


FROM scratch AS binary

COPY --from=builder /out/ /
