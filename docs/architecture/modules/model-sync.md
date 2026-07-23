# model-sync 模块

> 本文件描述上游模型同步域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 从供应商同步模型列表、过滤、日志与同步后动作 |
| 目录位置 | `handler/modelsync/`；`service/modelsync/`；`service/model_sync.go`、`model_sync_auto_actions.go`；`repository/model_sync_log.go`；实体 `ModelSyncLog` |
| 主要职责 | 拉取上游模型、计算增删、更新 `upstream_models`、过滤规则、自动同步、触发自动关联/清理 |

## 2. 职责与边界

- **负责什么**：`SyncProviderModels` / `SyncAllProviders` / `StartAutoSync`；过滤与清理配置；同步日志；`ActionHooks` 注入点
- **不负责什么**：供应商 CRUD；直接实现关联表修改（经 Hook 委托）；协议转换
- **对外暴露**：`NewModelSyncService`、`Sync*`、`GetRecentAddedModels`、`ActionHooks`；管理端同步与日志 API
- **依赖谁**：`models`、`providers`（`Models`）；门面注入 auto-associate/clean；`main` 启动自动同步

## 3. 内部结构

```
handler/modelsync/                 # 触发同步、日志、stats
service/model_sync.go              # 门面 + hooks 装配
service/model_sync_auto_actions.go # 同步后动作实现
service/modelsync/                 # Service 核心、filter、cleanup、config
repository/model_sync_log.go
models.ModelSyncLog
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `modelsync.Service` | 同步编排 | `service/modelsync/` | 同包 |
| `ActionHooks` | 同步后自动关联/清理 | `service/modelsync/` | `model_sync_auto_actions` 等注入 |
| `GetProviderModels` | 经 providers 拉上游列表 | service 门面/modelsync | providers 实现 |
| `ModelSyncLogRepo` | 同步日志 | `repository/model_sync_log.go` | GORM |

## 5. 特殊约定

- 过滤规则（如仅 `:free`）为配置驱动，适配不同公益站场景
- 同步后副作用不内嵌在 Service 硬依赖 associations 包，统一走 `ActionHooks`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
