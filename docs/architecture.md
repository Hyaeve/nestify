# Nestify 架构基线

## 技术栈

- 前端：Vue 3 + Element Plus + Pinia + Vue Router
- 后端：Go 单体服务
- 主存储：SQLite
- 导入导出：YAML
- 部署方式：单容器交付

## 子系统

1. Web 管理界面
2. API 与任务编排层
3. 文件处理执行层
4. SQLite / 运行目录 / YAML 导入导出

## 当前实现阶段

当前仅完成工程骨架，不包含数据库迁移、规则执行器和完整页面实现。

## 当前最小可运行链路

1. 后端提供 [`/api/v1/health`](backend/internal/httpapi/router.go:1) 健康检查接口
2. 前端仪表盘通过 [`fetch`](frontend/src/api/system.ts:1) 请求健康检查接口
3. 生产模式下后端托管 [`frontend/dist`](frontend/src/main.ts:1) 构建产物，并支持 SPA 路由回退

## 当前已落地的最小数据链路

1. SQLite 作为主持久化存储
2. 规则对象已具备最小数据库表结构与查询/创建能力
3. 系统设置已具备最小数据库表结构与读取能力
4. 前端规则页已从后端真实接口拉取列表数据

