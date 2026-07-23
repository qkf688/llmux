# logs-metrics 模块

> 本文件描述请求日志、ChatIO 与仪表盘指标域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 请求可观测：日志查询清理、IO 详情、统计指标 |
| 目录位置 | `handler/logs/`；`handler/metrics/`；`service/chat` 内 `RecordLog`/落库编排；`service/chatstats/`（Stats 写入）；`repository/chat_log.go`；`models/retention.go`；实体 `ChatLog`、`ChatIO`、`Stats*` |
| 主要职责 | 管理端查询/删除/清理日志；ChatIO 详情；按天/小时/模型/供应商等指标；保留策略 |

## 2. 职责与边界

- **负责什么**：`ChatLog`/`ChatIO` 读删与清理；metrics 聚合查询；user-agents 等辅助查询；保留策略 `EnforceRetentionByOldestID`
- **不负责什么**：健康检查日志、模型同步日志（各自域）；发起 LLM 调用；模型/供应商配置
- **对外暴露**：`/api/logs*`、`/api/metrics*` 等 REST；`ChatLogRepo`/`ChatIORepo`
- **依赖谁**：`repository`（日志路径）；metrics 部分直读 `models.DB`；写入侧：日志落库编排在 `service/chat`，Stats 累加在 `service/chatstats`（由 chat 触发）

## 3. 内部结构

```
handler/logs/          # 列表、详情、chat-io、删除、enrich
handler/metrics/       # dashboard 计数与时序
service/chat/*record* / chat_log_storage.go  # 日志落库编排
service/chatstats/     # Stats* 写入（叶子包）
repository/chat_log.go
models.ChatLog / ChatIO / Stats*
models/retention.go
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ChatLogRepo` / `ChatIORepo` | 日志与 IO 持久化访问 | `repository/chat_log.go` | GORM |
| `RecordLog` / 落库编排 | 请求结束后 processer + 落库 | `service/chat` | 同包 |
| `chatstats.Record*` | 请求/token/供应商统计累加 | `service/chatstats/` | 同包 |
| `EnforceRetentionByOldestID` | 按策略裁剪旧日志 | `models/retention.go` | 同文件 |

## 5. 特殊约定

- `logs/enrich.go` 允许多表只读富化直连（repository 白名单例外之一）
- 健康/同步日志不归本模块，前端有独立路由 `health-check-logs` / `model-sync-logs`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
