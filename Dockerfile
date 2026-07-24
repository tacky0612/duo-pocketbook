# ローカル検証(Docker Compose)用イメージ。
# フロントエンド(React+Vite)とバックエンド(Go)をビルドし、単一コンテナで配信する。

# --- フロントエンドビルド ---
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# npm run build の docs:api（redocly）が ../api/openapi.yaml を参照するため配置する
COPY api/openapi.yaml /api/openapi.yaml
RUN npm run build

# --- バックエンドビルド ---
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# --- 実行 ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=frontend /app/dist /app/frontend/dist
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=2s --retries=30 \
  CMD wget -q --spider http://localhost:8080/health || exit 1
CMD ["/app/server"]
