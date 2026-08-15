# protocol-transform 模块

> 本文件描述协议转换模块的职责、边界、对外暴露、依赖、接口契约。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 外部协议 ↔ 内部统一模型的双向转换（含流式） |
| 目录位置 | `service/transform/`；`service/anthropic/`；`service/responses/`；`service/transform_facade.go`；`models/unified/` |
| 主要职责 | OpenAI Chat / OpenAI Responses / Anthropic 与 `UnifiedRequest`/`UnifiedResponse` 互转 |

## 2. 职责与边界

- **负责什么**：请求/响应格式转换；流式 SSE 转换；适配器注册；实时流路由；统一模型校验/修复辅助
- **不负责什么**：供应商选择、HTTP 调用、重试、日志持久化、管理端 CRUD
- **对外暴露**：`FormatAdapter`、`TransformerManager`、包级 `Transform*ToUnified` / `TransformProviderResponse` 等（经 `transform_facade`）
- **依赖谁**：`models` / `models/unified`、`common`/`common/maputil`；内部依赖 `service/anthropic`、`service/responses`

## 3. 内部结构

```
service/
├── transform_facade.go
├── transform/              # 转换引擎与适配器
│   ├── adapters_register.go
│   ├── openai/ | anthropic/ | responses/ | streaming/ | shared/
├── anthropic/              # Anthropic 编解码细节
└── responses/              # Responses API 编解码细节
models/
├── unified.go              # 统一类型门面/别名
└── unified/                # UnifiedRequest/Response 等
```

各子目录职责：
- `transform/`：引擎入口、适配器注册、流式路由
- `anthropic/` / `responses/`：协议特有编解码
- `models/unified/`：canonical 中间表示

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `FormatAdapter` | 外部格式 ↔ 统一模型 | `service/transform/` | openai / anthropic / openai-res 适配器 |
| `TransformerManager` | 请求/响应转换编排 | `service/transform/` | `service/transform` |
| `RegisterAdapter` | 注册协议适配器 | `service/transform/` | `adapters_register.go` |
| `RegisterRealtimeRoute` | 注册实时流协议组合 | `service/transform/streaming/` | 各路由实现 |
| `UnifiedRequest` / `UnifiedResponse` | 协议中枢类型；`UnifiedMessage.RedactedThinkingData` 独立保存 Anthropic `redacted_thinking.data` 不透明密文，不与 reasoning 文本混用 | `models/unified/` 与 `models/unified.go`（响应等部分类型仍在门面文件） | 被转换器读写 |
| `ThinkingClampConfig` | 思考档位钳制配置（白名单 + autoFallback + unknownStrategy），由 chat 主路径传入 `ProcessRequest` | `service/transform/transformer.go` | `clampUnifiedReasoning`（transform 路径）+ `clampPassthroughReasoning`（passthrough 路径） |

## 5. 特殊约定

- 转换矩阵以统一模型为中枢，避免 N×M 直接互转
- 同格式路径可短路；流式与非流式分路径
- **流式 usage 时序（OpenAI 上游）**：`stream_options.include_usage` 的 usage 在 `finish_reason` **之后**单独发一个 `choices:[]` 的尾部 chunk，这是 OpenAI 规范行为而非个别上游怪癖。因此 `streaming/openai_to_responses.go` 不在看到 `finish_reason` 时立即发 `response.completed`，而是把事件缓存进 `state.pendingCompleted`、把各 chunk 的 usage 收进 `state.pendingUsage`（`captureOpenAIUsage`，`len>0` 才覆盖，故 finish 包的 `usage:null` 不会清掉真值）。终态在 usage 尾包**到达时**即交付（`flushPendingOpenAIToResponsesCompleted`），`[DONE]` 与流末 finalize 退化为幂等兜底——三入口清空 `pendingCompleted` 保证只发一次。usage 尾包到达就 flush（而非等 `[DONE]`）是活性保障：避免上游发完 usage 后挂住连接时客户端空等，也避免第二跳 anthropic 拿到未关闭的 content_block。空 `choices` 分支**不得**在收走 usage 前提前 return。`buildResponsesUsage` 透传尾包里的 `prompt_tokens_details.cached_tokens` / `completion_tokens_details.reasoning_tokens` 真值，不写死 0
- **`realtimeStreamState.finalize`**：为上述延后发送引入的流末收尾钩子，由路由 handler 在确有延后内容时设置，`transformStreamBodyRealtime` 在 scanner 结束时调用一次；上游读取出错（`scanner.Err()`）时也会尽力调用一次以交付终态（下游 pipe 此时仍可写；`flushEvent` 出错分支则不调，因那是写下游失败）。其它 realtime 路由无延后需求，`finalize` 保持 nil
- **Anthropic 出站的 usage 权威位置是 `message_delta.usage`**：`message_start.usage` 在 OpenAI 上游场景只能兜底 `1/1`——OpenAI 到流末才给 `prompt_tokens`，而 `message_start` 必须立即发。这是协议桥接的固有限制而非缺陷；`service/chat` 的 anthropic processer 同样只认 `message_delta.usage`
- **Anthropic 入站的 usage 拆在两个事件里**：`message_start.message.usage` 给 input 侧（`input_tokens` / `cache_read_input_tokens` / `cache_creation_input_tokens`），`message_delta.usage` 给 `output_tokens`；旧版上游的 `message_delta` 不重复带 input 侧。`streaming/anthropic_to_responses.go` 用 `state.anthropicStartUsage` 跨事件缓存 message_start 的 usage，在 message_delta 合并，否则转换后 input 侧 token 全为 0
- **转换层向落库侧旁路上游原始 usage（`models.TransformSideChannel`）**：转换输出受目标协议表达能力限制（Anthropic 无 reasoning_tokens 槽位、`message_start` 时序早于真实 prompt_tokens），从下游流反解必然有损，因此在**读上游那一跳**就地捕获。
  - 流式：`streaming/upstream_usage.go` 按上游协议各一个映射函数（openai / openai-res / anthropic 三种），由对应的第一跳 handler 调用；不按「方向」铺开，避免 N×M 份重复解析。多跳（provider → openai-res → client）**只有第一跳**持有侧信道，第二跳传 nil，否则捕获到的是中间格式而非上游真值。
  - 非流式：`transform_provider_response.go` 在 `providerAdapter.ParseResponse` 后直接 `SetUpstreamUsage`——解析上游响应即得到，无 goroutine、无时序问题。
  - 侧信道语义是「最后一次观测胜出」，故调用方必须传**已合并的完整快照**（Anthropic 尤其：分两次分别只带 input / output 侧会互相覆盖）。落库侧消费见 [logs-metrics.md](logs-metrics.md) 的 `UsageSource`。
- **上游 usage 字段容错走有序候选路径表**（`streaming/upstream_usage.go` 的 `usageFieldCandidates`），不是 per-provider if-else：新增一种非标准写法只加一行候选。已覆盖 usage 顶层 `reasoning_tokens`、`prompt_cache_hit_tokens`、Anthropic 的 `cache_read_input_tokens` 等
- **`ResponsesUsage` 的 details 键名必须是复数**（`input_tokens_details` / `output_tokens_details`）：这是 OpenAI Responses 的线格式，也是 streaming 编码侧与 `service/chat/process.go` 的 `OpenAIResUsage` 解码侧一直在用的写法。曾误写单数 `input_token_details`，导致解码恒 nil（读侧成死代码）且编码输出不合规
- **出站 usage 时序不对称（已知债务）**：`streaming/responses_to_openai.go` 给 openai 客户端出站时把 usage 塞进带 `finish_reason` 的同一个 chunk，与入站侧刚判定「不可假设」的形状相反。对官方 SDK 无害（两种都能解析），属一致性债务，见 next-do



- **扩展最小改动集（现状）**：新外部格式通常同时需要 `RegisterAdapter`、流式 `RegisterRealtimeRoute`（若涉及 SSE）、chat 侧 `Beforer`/`Processer`、`register_v1` 路由与 `consts.Style*`；Realtime 矩阵当前未覆盖全部协议组合，部分路径走 pivot/遗留逻辑
- **Responses `function_call_output.output` 多模态**：`ResponsesItem.Output` 为 `interface{}`——纯文本 tool result 输出 `string`（老上游兼容），含图片块输出 `input_text`/`input_image` 数组（OpenAI Responses 协议规范）。编解码 helper 在 `service/responses/tool_content_codec.go`，入站解析复用 `parsePartsToUnifiedContent`（纯文本→string、含图片→块数组，与 Anthropic 路径行为一致）
- 专题细节可参考历史图示（本地 `local/架构文档/格式转换架构图.md`，未入库）

### 思考档位归一化与钳制（Stage B 扩档）

- **归一化（`NormalizeReasoningEffort`）**：只做字符串小写归一化，不做模型能力钳制。8 档 `[minimal, low, medium, high, xhigh, max, none, auto]` 原样小写透传；未知档位也原样透传（不再回退默认值）。钳制职责分离到 `ClampReasoningEffort`。
- **钳制（`ClampReasoningEffort`）**：纯函数（`models/model.go`），根据白名单 + `autoFallback` + `unknownStrategy` 执行就近钳制。规则：
  - 档位在白名单内 → 透传
  - 档位不在白名单 → 就近钳制到白名单内最接近的档位（6 档有序，按索引距离取最近；`none`/`auto` 特殊档不在有序序列中，只在白名单显式包含时透传）
  - `auto` 不在白名单 → 回退到 `autoFallback`（`SettingKeyReasoningEffortDefaultValue`）
  - `none` 不在白名单 → 返回空串（调用方剥离 thinking 字段）
  - 未知档位 + `unknownStrategy=clamp_to_default` → 回退到 `autoFallback`；`passthrough` → 原样透传
- **budget 互转（`ReasoningEffortToThinkingBudget` / `ThinkingBudgetToReasoningEffort`）**：6 档双向映射，single source of truth 在 `service/anthropic/helpers.go`。保留 Octopus 原值（low→1000, medium→20000, high→50000），新增 minimal→512, xhigh→80000, max→128000。
- **budget 联动（方案 E）**：effort 被钳制时，budget 按钳制后 effort 对应的 budget 值作上限——超上限则钳到上限 + warn；低于上限不动；effort 未钳制则 budget 不动。防止用户用高 budget 绕过 effort 白名单。
- **分路径钳制**：
  - transform 路径：`ProcessRequest(ctx, raw, clamp)` 在 ToUnified 后、FromUnified 前对 `unified.ReasoningEffort` 钳制（`clampUnifiedReasoning`）
  - passthrough 路径：`clampPassthroughReasoning` 对 raw body 按 style 钳制 effort 字段（OpenAI `reasoning_effort`、Anthropic `output_config.effort`、Responses 双路径 `reasoning.effort` + `metadata.reasoning_effort`）+ budget 字段（Anthropic `thinking.budget_tokens`、Responses `reasoning.max_tokens`）。**大小写归一化**：两条路径都在钳制前对 effort 做小写归一化（transform 经 `NormalizeReasoningEffort`，passthrough 经 `strings.ToLower`），避免客户端传 "HIGH" 时白名单命中失败。passthrough 路径在归一化后若值在白名单内但原始大小写不规范，会写回归一化小写值。

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
