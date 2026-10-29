FROM node:22-bookworm AS web
WORKDIR /repo/web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
COPY --from=web /repo/server/internal/static/dist ./internal/static/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/localcloud ./cmd/localcloud

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg postgresql-client \
        libreoffice-writer libreoffice-calc libreoffice-impress \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 1000 localcloud \
    && useradd --system --uid 1000 --gid localcloud --home-dir /data --shell /usr/sbin/nologin localcloud

WORKDIR /app
COPY --from=build /out/localcloud /app/localcloud

ENV LOCAL_CLOUD_DATA_DIR=/data
RUN mkdir -p /data && chown -R localcloud:localcloud /data

USER localcloud
EXPOSE 8090
ENTRYPOINT ["/app/localcloud"]
