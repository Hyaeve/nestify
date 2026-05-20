FROM node:20-alpine AS frontend-builder
WORKDIR /workspace/frontend
COPY frontend/package.json ./
RUN npm install
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
RUN addgroup -S nestify && adduser -S nestify -G nestify
COPY --from=backend-builder /out/nestify /app/nestify
COPY --from=frontend-builder /workspace/frontend/dist /app/web
COPY config/config.example.yaml /config/config.example.yaml
RUN mkdir -p /data/runtime /data/staging /logs && chown -R nestify:nestify /app /data /logs /config
USER nestify
EXPOSE 8080
ENV NESTIFY_WEB_DIR=/app/web
ENV NESTIFY_DB_PATH=/data/app.db
CMD ["/app/nestify"]

