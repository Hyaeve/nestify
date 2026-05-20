# Nestify

Nestify 是一个面向 NAS / 家庭服务器场景的可视化归档管理平台。

当前仓库处于首轮工程骨架阶段，已建立：

- Go 后端基础目录结构
- Vue 3 + Element Plus 前端基础目录结构
- Docker / Compose 编排骨架
- 配置示例与架构文档骨架
- 前后端最小连通链路骨架

## 目录结构

```text
.
├─ backend/                # Go 后端
├─ frontend/               # Vue 3 前端
├─ config/                 # 配置示例
├─ data/                   # 运行数据、SQLite、临时文件
├─ deploy/                 # 部署说明与示例
└─ docs/                   # 架构文档
```

## 当前阶段说明

本轮仅搭建工程骨架与最小入口，不包含：

- 实际归档规则执行逻辑
- 完整 API 业务实现
- 完整页面业务交互
- 数据库表与迁移实现

## 当前最小连通能力

- 后端提供健康检查接口：`GET /api/v1/health`
- 后端在生产模式下可托管前端静态文件
- 前端仪表盘会请求后端健康检查并显示连接状态

## 当前已落地的持久化基础

- 后端使用 SQLite 作为主存储
- 默认数据库路径：`../data/app.db`（容器内为 `/data/app.db`）
- 已增加单管理员登录骨架，默认初始化账号可由环境变量控制
- 已提供最小规则接口：
  - `GET /api/v1/rules`
  - `GET /api/v1/rules/:id`
  - `POST /api/v1/rules`
  - `GET /api/v1/settings`
- 前端规则页已接入真实规则列表接口

## 本地开发

### 启动后端

```bash
cd backend
go run ./cmd/server
```

默认监听 `:8080`。

可通过环境变量覆盖：

- `NESTIFY_HTTP_ADDR`
- `NESTIFY_WEB_DIR`
- `NESTIFY_DB_PATH`
- `NESTIFY_ADMIN_INITIAL_USERNAME`
- `NESTIFY_ADMIN_INITIAL_PASSWORD`
- `NESTIFY_BROWSE_ROOTS`

## 默认管理员初始化

首次启动且数据库中不存在管理员账号时，会自动创建一个管理员账号：

- 用户名默认值：`admin`
- 密码默认值：`nestify123`

建议在生产环境通过环境变量覆盖：

```bash
NESTIFY_ADMIN_INITIAL_USERNAME=your-admin
NESTIFY_ADMIN_INITIAL_PASSWORD=your-strong-password
```

## 路径浏览根目录

路径选择器默认会自动推断系统可浏览根目录：

- Windows 下自动推断可用盘符
- Linux / 容器下默认使用根目录 `/`

如果你希望限制前端只能浏览指定目录，可设置：

```bash
NESTIFY_BROWSE_ROOTS=/library;/archive;/config
```

多个根目录使用分号分隔。

### 启动前端

```bash
cd frontend
npm install
npm run dev
```

前端开发服务器默认监听 `5173`，并通过代理把 `/api` 请求转发到后端。

## 容器运行

```bash
docker compose up --build
```

容器模式下：

- 前端构建产物由后端从 `/app/web` 托管
- 默认访问地址为 `http://localhost:8080`

## GitHub 自动构建镜像

仓库已补充 GitHub Actions 工作流 [docker-image.yml](.github/workflows/docker-image.yml)，可在推送到 GitHub 后自动构建容器镜像。

### 触发方式

- 推送到 `main` / `master`
- 推送标签 `v*`，例如 `v0.1.0`
- 手动触发 GitHub Actions
- Pull Request 时会执行镜像构建验证，但不会推送镜像

### 镜像仓库

默认推送到 GitHub Container Registry：

```text
ghcr.io/<owner>/<repo>
```

例如仓库是 `yourname/nestify`，镜像名就是：

```text
ghcr.io/yourname/nestify
```

### 自动生成的标签

- 默认分支推送：`latest`
- 分支名标签：例如 `main`
- Git 标签：例如 `v0.1.0`
- 提交 SHA 标签

### 首次使用前需要确认

1. 将仓库推送到 GitHub
2. 在仓库设置中确认 Actions 有权限写入 Packages
3. 首次推送后，在 GitHub 的 Actions 与 Packages 页面确认镜像已生成

### 本地与仓库建议

- 构建上下文忽略规则见 [.dockerignore](.dockerignore)
- Windows 本地产物如 [backend/server.exe](backend/server.exe) 已加入 [.gitignore](.gitignore)，避免上传到仓库

