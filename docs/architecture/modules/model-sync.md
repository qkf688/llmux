# model-sync 模块

> 本文件描述上游模型同步域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 从供应商同步模型列表、过滤、日志与同步后动作 |
| 目录位置 | `handler/modelsync/`；`service/modelsync/`；`service/model_sync.go`；`repository/model_sync_log.go`；实体 `ModelSyncLog` |
| 主要职责 | 按分组拉取上游模型、全量填充 `KeyGroup.Models` 白名单、过滤规则、自动同步、触发自动关联/清理 |

## 2. 职责与边界

- **负责什么**：`SyncProviderModels` / `SyncAllProviders` / `StartAutoSync`；过滤与清理配置；同步日志；`ActionHooks` 注入点
- **不负责什么**：供应商 CRUD；直接实现关联表修改（经 Hook 委托 `service/autoassoc`）；协议转换
- **对外暴露**：`NewModelSyncService`、`Sync*`、`GetRecentAddedModels`、`ActionHooks`；管理端同步与日志 API
- **依赖谁**：`models`、`providers`（`Models`）、`service/channel`（Assemble / 凭据 / BuildConfig）、`repository`（KeyGroup 等）；门面注入 autoassoc；`main` 启动自动同步

## 3. 内部结构

```
handler/modelsync/                 # 触发同步、日志、stats（薄 handler，编排下沉 service）
service/model_sync.go              # 门面 + ActionHooks 装配到 autoassoc + GetSyncStats
service/modelsync/                 # Service 核心、sync_group、filter、cleanup、config、actions、stats
repository/model_sync_log.go
models.ModelSyncLog
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `modelsync.Service` | 同步编排 + 统计聚合 | `service/modelsync/` | 同包 |
| `Service.GetSyncStats` | 汇总供应商数/各状态计数/上次下次同步时间 | `service/modelsync/stats.go` | 经 `repository` 聚合，不直连 DB |
| `ActionHooks` | 同步后自动关联/清理 | `service/modelsync/` | `NewModelSyncService` 注入 `GetAutoAssocService` |
| `GetProviderModels` | 组织级模型目录：各分组白名单并集 ∪ `custom_models` | service 门面/modelsync | 读 KeyGroup + Config（非上游 HTTP） |
| `ModelSyncLogRepo.AggregateProviderStats` | 按 provider 取最新一次同步状态并分桶计数 | `repository/model_sync_log.go` | GORM；含旧行空 status 兼容归一化 |
| `ProviderRepo.EnabledModelEndpointIDs` / `CountEnabledModelEndpoint` | 启用模型端点的供应商 ID/数量（NULL 视为启用） | `repository/provider.go` | GORM |

## 5. 特殊约定

- **同步粒度 = 分组**：每个 KeyGroup 取活跃凭据调 `Models()`，全量写入该组 `Models` CSV；请求数 ≈ 分组数，与凭据数无关
- **空白名单**：空 = 未同步/不限制（选路不限；不向组织级目录贡献模型 ID）
- **不再写 `Config.upstream_models`**：同步真相在分组白名单；组织级目录由 `GetProviderModels` 聚合
- 过滤规则（如仅 `:free`）为配置驱动，适配不同公益站场景；按分组拉回后套用
- 同步后副作用不内嵌在 Service 硬依赖 handler 包，统一走 `ActionHooks`
- Hook 实现为 `service/autoassoc` 单一入口；**禁止** modelsync 再实现一套关联/清理循环
- 统计口径：`providers_with_updates/unchanged/errors` 是「每个 provider 最新一次同步状态」的计数，**不是**日志行数；同一 provider 的历史行只算最新那条
- 旧同步日志无 `Status` 字段（空串）时，按 `AddedCount/RemovedCount` 反推（有增删=success，否则 unchanged），归一化集中在 `repository.normalizeSyncStatus`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日收敛 autoassoc 回写。*
