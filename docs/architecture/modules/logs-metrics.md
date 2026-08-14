# logs-metrics 模块

> 本文件描述请求日志、ChatIO 与仪表盘指标域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 请求可观测：日志查询清理、IO 详情、统计指标 |
| 目录位置 | `handler/logs/`；`handler/metrics/`；`service/chat` 内 `RecordLog`/落库编排；`service/chatstats/`（Stats 写入）；`repository/chat_log.go`、`repository/stats.go`；`models/retention.go`；实体 `ChatLog`、`ChatIO`、`Stats*` |
| 主要职责 | 管理端查询/删除/清理日志；ChatIO 详情；按天/小时/模型/供应商等指标；保留策略 |

## 2. 职责与边界

- **负责什么**：`ChatLog`/`ChatIO` 读删与清理；metrics 聚合查询；user-agents 等辅助查询；保留策略 `EnforceRetentionByOldestID`
- **不负责什么**：健康检查日志、模型同步日志（各自域）；发起 LLM 调用；模型/供应商配置
- **对外暴露**：`/api/logs*`、`/api/metrics*` 等 REST；`ChatLogRepo`/`ChatIORepo`
- **依赖谁**：`repository`（日志读写、保留清理，含 `ChatLogRepo.EnforceRetention`；统计读写经 `StatsRepo`）；`handler/metrics` 经 `repos().Stats` 只做展示计算，不再直读 `models.DB`；写入侧：日志落库编排在 `service/chat`（经 `repos()`），Stats 累加在 `service/chatstats`（经 `repos().Stats`）

## 3. 内部结构

```
handler/logs/          # 列表、详情、chat-io、删除、enrich
handler/metrics/       # dashboard 计数与时序
service/chat/*record* / chat_log_storage.go  # 日志落库编排
service/chatstats/     # Stats* 写入组合（叶子包，经 StatsRepo）
repository/chat_log.go
repository/stats.go    # 六张 Stats* 表的 upsert 与聚合读
models.ChatLog / ChatIO / Stats*
models/retention.go
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ChatLogRepo` / `ChatIORepo` | 日志与 IO 持久化访问 | `repository/chat_log.go` | GORM |
| `StatsRepo` | 六张 Stats* 表的写入（`AddTimeBased`/`IncModelCalls`/`IncRealModelCalls`/`AddProviderStats`）与读侧聚合（`GetTotal`/`SumDailiesSince`/`List*`） | `repository/stats.go` | GORM（`clause.OnConflict` upsert） |
| `RecordLog` / 落库编排 | 请求结束后 processer + 落库 | `service/chat` | 同包 |
| `chatstats.Record*` | 请求/token/供应商统计累加 | `service/chatstats/` | 同包 |
| `EnforceRetentionByOldestID` | 按策略裁剪旧日志 | `models/retention.go` | 同文件 |

## 5. 特殊约定

- `logs/enrich.go` 允许多表只读富化直连（repository 白名单例外之一）
- 统计表的日期列格式（`2006-01-02`）与「时间维度 = total/daily/hourly 三表」属存储细节，只在 `repository/stats.go` 内出现；调用方一律传 `time.Time`
- `StatsProviderTotal.AvgResponseTime` 存的是**累计**耗时，均值由 `handler/metrics` 除以 `TotalRequests` 得到；补零/除零保护等展示计算不下沉仓储
- 健康/同步日志不归本模块，前端有独立路由 `health-check-logs` / `model-sync-logs`
- `EnforceRetentionByOldestID` 的 `BeforeDelete` 回调与主表删除在同一事务内执行；回调收 `tx *gorm.DB`，必须用 tx 操作子表，返回 error 会回滚整个事务（主表不删）。ChatLog 的 `EnforceRetention` 用此机制保证 ChatIO 与 ChatLog 同生共死，不产生孤儿 ChatIO 行（db 体积单调增长的根因修复）
- `ChatLog.RawResponseBody` 的截断语义按来源分两种：**流式转换路径**经 `service/transform/streaming` 的头尾双段累积器（头 512KB + 尾 512KB），超出部分丢弃中间段并在文本中插入 `...[llmux truncated N bytes ...]...` 标记、同时打一条 `slog.Warn`；保留尾段是为了不丢 `finish_reason` / `usage` / `[DONE]` / `message_stop` 等收尾事件。**非流式路径**（`service/chat/chat_attempt_log.go` 的 `captureRawResponseBody`）全量记录、无上限。故同一字段可能来自两种策略，排查时以有无截断标记为准；另注意累积体不是字节级保真（多行 data 已被合成单行）
- processer 出错时（`service/chat/chat_record.go` 错误分支）只写 `Status`/`Error` 就返回，**不消费流式累积体**，失败流的 `RawResponseBody` 目前为空
- **流式 token 统计的提取来源是「转换后发给客户端的流」，不是上游原始流**：`handler/v1/chat.go` 用 `io.TeeReader` 从 `res.Body` 分流给 `RecordLog`，而跨协议场景下该 `res.Body` 已被 `service/transform` 替换为转换管道的输出（上游原始字节只旁路给 `rawAccumulator` 写 `RawResponseBody`，不参与 usage 提取）。因此 `ChatLog` 的 `prompt_tokens`/`completion_tokens`/`total_tokens`/`tps` 正确性依赖转换层是否把上游 usage 完整带过来——转换层丢 usage 会直接表现为这些字段全 0（`tps` 由 `TotalTokens` 派生，故同步为 0）。各 style processer 认的字段不同：openai 取根级 `usage`（不解析 `choices`，故天然免疫 OpenAI 的 `choices:[]` usage 尾包）、anthropic 取 `message_delta.usage`、openai-res 取 `response.completed` 内的 usage
- **`reasoning_tokens` 在 anthropic / openai-res style 下仍恒为 0**（已知缺口）：`service/chat/process.go` 这两个 style 的 `parseUsage` 手工构造 `models.Usage` 且从不赋值 `CompletionTokensDetails`。转换层侧 `streaming/openai_to_responses.go` 已透传上游真值（`buildResponsesUsage` 同时透传 `cached_tokens`，该列经 openai-res `parseUsage` 会落库），但 `streaming/anthropic_to_responses.go` 仍把 `reasoning_tokens` 硬编码为 0。仅 openai 直通路径（整体 `json.Unmarshal` 到 `models.Usage`）能落到上游真实值


---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
