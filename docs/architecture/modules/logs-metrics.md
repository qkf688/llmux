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
handler/logs/dto.go    # 对外响应契约（chatLogResponse / chatIOResponse）+ 映射
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
| `chatLogResponse` / `chatIOResponse` | `/api/logs*` 与 `/api/logs/:id/chat-io` 的对外响应契约（全 snake_case），与 `models.ChatLog`/`ChatIO` 解耦；含读时计算字段 `unclaimed_request_fields`（见下） | `handler/logs/dto.go` | 同文件的 `buildChatLogResponse` / `buildChatIOResponse` / `computeUnclaimedRequestFields` |
| `ChatLogRepo` / `ChatIORepo` | 日志与 IO 持久化访问 | `repository/chat_log.go` | GORM |
| `StatsRepo` | 六张 Stats* 表的写入（`AddTimeBased`/`IncModelCalls`/`IncRealModelCalls`/`AddProviderStats`）与读侧聚合（`GetTotal`/`SumDailiesSince`/`List*`） | `repository/stats.go` | GORM（`clause.OnConflict` upsert） |
| `RecordLog` / 落库编排 | 请求结束后 processer + 落库 | `service/chat` | 同包 |
| `chatstats.Record*` | 请求/token/供应商统计累加 | `service/chatstats/` | 同包 |
| `EnforceRetentionByOldestID` | 按策略裁剪旧日志 | `models/retention.go` | 同文件 |

## 5. 特殊约定

- **对外响应必须经 `handler/logs/dto.go` 的 DTO，禁止裸序列化 `models.ChatLog`/`ChatIO`**。理由是历史教训而非洁癖：`ChatIO` 曾直接 `httpresp.Success(c, chatIO)` 返回，GORM 模型的 Go 字段名就是 JSON key，改一个字段名（`LogId`→`LogID`）即破坏前端契约；`ChatLog` 曾用 `map[string]any` 手拼，漏写 `completion_tokens_details` 导致 reasoning_tokens 落库有值但 API 永不返回、前端类型声明成为谎言。结构体 DTO 让编译器兜住字段齐全，模型层字段改名不再波及 API
- API 字段一律 snake_case（含 `id` / `created_at` / `chat_io` 等，AGENTS.md 3.1）；`raw` 六字段用 `*string`+`omitempty`——空字符串是合法值，必须与「include_raw=false 未返回」区分
- **`unclaimed_request_fields` 是读时计算的诊断字段，不落库**：指出客户端原始请求体里有哪些顶层键本网关根本没解析（转换后会静默消失），供开发/运维据此去补 transform。入参是 `ChatLog.Style` + `ChatLog.RawRequestBody`，调 `transform.UnknownRequestKeys`（见 [protocol-transform.md](protocol-transform.md)）。
  - **不落库的理由**：DTO 会随 transform 演进，落库的结论会僵化成「按当时 DTO 算的旧答案」；读时算永远反映最新代码，且日志详情是低频页面，反射 + JSON parse 的开销可忽略。
  - **与 raw 组同一个 `includeRaw` 门控**：它的输入就是 `RawRequestBody`，`include_raw=false` 时该列压根没从库里读出来（`ChatLogRepo.List` 的 Omit），此时算出来的只会是假的 `raw_not_recorded`。
  - **用 `status` 枚举而非「nil / 空数组」表达结果**：`ok`（`fields` 即结论，可能为空数组）/ `raw_not_recorded`（开关关着或 `errors_only` 在成功时清空了 raw）/ `style_unsupported`（该入站 style 未注册认领键集合；三个生产 style 现已全部注册，故此态**当前无生产可达路径**，保留是为新 style 落地期——前端对应文案同理，清理死代码时别删）/ `parse_error`（raw 不是 JSON 对象，`detail` 给原因）。「查不出来」有三种彼此需要区分的原因，挤进一个可空数组的话前端只能猜，最坏是把「查不了」显示成「已检查、无问题」——假阴性比没有这个功能更糟。
  - **语义边界**：结果是「网关 DTO 不认领的键」，不等于「转换过程中丢了这些键」——passthrough 路径（`style == provider.Type`）请求不进 transform，这些键实际原样透传。且只看顶层，`messages[].xxx` 不在范围内。前端措辞须说清这两点
- `ChatLogRepo.List` 在 `!IncludeRaw` 时用 **Omit 排除 raw 大字段**，不用 Select 白名单：白名单的失效模式是静默的（新增字段忘记加进来 → 该字段恒为零值，调用方无从察觉，`usage_source` 就这样漏过一轮）
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
- **上游 usage 字段容错**：全仓单一归一入口 `models.UsageFromMap`（`models/usage_normalize.go`）用有序候选路径表（`promptTokenPaths` / `cachedTokenPaths` / `reasoningTokenPaths` 等）归一各家写法——含 `completion_tokens_details.reasoning_tokens`、`output_tokens_details.reasoning_tokens`、usage 顶层 `reasoning_tokens`、`prompt_cache_hit_tokens`、Anthropic 的 `cache_read_input_tokens`。新增一种非标准写法只加一行候选，不改控制流（OCP）。流式侧信道（`streaming/upstream_usage.go`）、非流 anthropic 入站（`service/anthropic/response_inbound.go`）与 `service/chat/process.go` 的三个 processer（合并为单一 `parseUsageJSON`）共用这一份；此前四处各写一套，退化成同一概念的四个残缺子集（openai-res processer 不读 reasoning、anthropic 非流入站不认 openai 兼容字段）。**全零快照的丢弃收口在 `models.TransformSideChannel.SetUpstreamUsage`**（判据是 `models.Usage.HasTokens`，只看 Total/Prompt/Completion），流式与非流式共用同一道门禁——避免把「没解析出来」记成「上游明确报 0」而压掉 `missing` 告警。`resolveUsageSource` 判定有效性用的是同一个 `HasTokens`，两侧口径不会漂移
- **`total_tokens` 的口径分两种，收口在 `models.ResolveTotalTokens`（纯函数）**：
  - **回退口径**——上游省略 `total_tokens` 时取 `prompt + completion`，不把 Anthropic 的 `cache_read` / `cache_creation` 额外计入，与 `models.Usage.HasTokens` 一致。
  - **上游优先**——上游显式给了 `total_tokens`（>0）时一律原样采用、不重算，即使该值含 cache token（贴近上游真值）。故 anthropic 形状 usage 的 total 不再被强制重算为 `prompt+completion`。

  侧信道 / processer 落库（经 `models.UsageFromMap`）与流式客户端写出点（三处统一走 `streaming/usage_wire.go` 的 `usageWireFromModel`：`responses_to_openai.go` 的 usage 尾包、`openai_to_responses.go` 与 `anthropic_to_responses.go` 的 `response.completed`）共用这一判据——此前写出点直写上游原值、缺失时写出 0（anthropic 出站更是硬算 `input+output`），与落库侧分叉，造成同一次请求「客户端看到的 total 与 DB 记录不一致」。**已知缺口**：非流入站的两处 DTO→unified 映射（`service/responses/response_parse.go`、`service/transform/openai/response.go`）仍直取上游原值未过该函数
- 各 style processer 认的字段不同：openai 取根级 `usage`（不解析 `choices`，故天然免疫 OpenAI 的 `choices:[]` usage 尾包）、anthropic 取 `message_delta.usage`、openai-res 取 `response.completed` 内的 usage。这些只在 `downstream` / `passthrough` 路径生效


---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
