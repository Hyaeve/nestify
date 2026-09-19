FROM node:20-alpine AS frontend-builder
WORKDIR /workspace/frontend
COPY frontend/package.json ./
COPY frontend/package-lock.json ./
RUN npm ci --include=dev
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend-builder
WORKDIR /workspace/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nestify ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache tzdata
COPY --from=backend-builder /out/nestify /app/nestify
COPY --from=frontend-builder /workspace/frontend/dist /app/web
COPY config/config.example.yaml /config/config.example.yaml
RUN mkdir -p /data/runtime /data/staging /log /config
ENV TZ=Asia/Shanghai
EXPOSE 8080
ENV NESTIFY_WEB_DIR=/app/web
ENV NESTIFY_DB_PATH=/data/app.db
# 运行日志（run_history）单独一个文件，挂到 /log 与主数据分开。
ENV NESTIFY_LOG_DB_PATH=/log/logs.db
CMD ["/app/nestify"]

