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
- **usage 归集有三条来源，优先级由 `service/chat/chat_record.go` 的 `resolveUsageSource` 决定，来源记在 `ChatLog.UsageSource` 列**（取值常量见 `models/model.go` 的 `UsageSource*`）：
  1. `upstream`——跨协议场景下由 `models.TransformSideChannel` 旁路交出的**上游原始 usage**。转换层在读上游那一跳就地捕获（流式在 `service/transform/streaming/upstream_usage.go`，非流式在 `service/transform/transform_provider_response.go` 解析 `ParseResponse` 结果后），不经目标协议裁剪，最可信。多跳转换（provider → openai-res → client）只有**第一跳**持有侧信道，第二跳传 nil，避免把中间格式当成上游真值。
  2. `passthrough`——`style == provider.Type`，无转换层，processer 读的就是上游响应本身。
  3. `downstream`——走了转换但旁路没拿到 usage，退化为从**转换后**的下游流反解。此路径会打 `slog.Warn`，因为它可能因目标协议缺字段而失真。
  
  三条都拿不到有效 token 时记 `missing` 并告警——token 恒为 0 会静默影响计费与配额，不能沉在 DB 里。usage 修正发生在 `chatstats.Record*` **之前**，故统计与日志读到的是同一份数字。
- **上游 usage 字段容错**：`streaming/upstream_usage.go` 的 `usageFromUpstreamMap` 用有序候选路径表（`promptTokenPaths` / `cachedTokenPaths` / `reasoningTokenPaths` 等）归一各家写法——含 `completion_tokens_details.reasoning_tokens`、`output_tokens_details.reasoning_tokens`、usage 顶层 `reasoning_tokens`、`prompt_cache_hit_tokens`、Anthropic 的 `cache_read_input_tokens`。新增一种非标准写法只加一行候选，不改控制流（OCP）。**全零快照的丢弃收口在 `models.TransformSideChannel.SetUpstreamUsage`**（判据是 `models.Usage.HasTokens`，只看 Total/Prompt/Completion），流式与非流式共用同一道门禁——避免把「没解析出来」记成「上游明确报 0」而压掉 `missing` 告警。`resolveUsageSource` 判定有效性用的是同一个 `HasTokens`，两侧口径不会漂移
- `total_tokens` 的落库口径统一为 `prompt + completion`（不把 Anthropic 的 cache_read / cache_creation 额外计入），转换层与 `resolveUsageSource` 两侧一致
- 各 style processer 认的字段不同：openai 取根级 `usage`（不解析 `choices`，故天然免疫 OpenAI 的 `choices:[]` usage 尾包）、anthropic 取 `message_delta.usage`、openai-res 取 `response.completed` 内的 usage。这些只在 `downstream` / `passthrough` 路径生效


---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
