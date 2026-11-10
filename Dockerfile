# syntax=docker/dockerfile:1

# ---- builder ----------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

# Cache modules first for faster incremental builds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled (modernc.org/sqlite is pure Go) → fully static binary.
# TARGETOS/TARGETARCH supplied by buildx (domyślnie linux/amd64 — zwykły komputer).
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime ----------------------------------------------------------------
# Root variant (uid 0) so the process can write the SQLite file into the named
# /data volume, which Docker initialises as root:root. Fine for a single-user
# homelab; switch to :nonroot only if you pre-chown the volume to uid 65532.
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Portal assets are baked into the image. Uploaded courses + the SQLite DB live
# in the /data volume, so they survive image rebuilds and updates.
COPY --from=builder /src/web ./web
COPY --from=builder /out/server ./server

ENV ADDR=:8080 \
    WEB_DIR=/app/web \
    DB_PATH=/data/courses.db \
    COURSES_DIR=/data/courses \
    MAX_UPLOAD_MB=50

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/server"]
