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
| `ClaimedRequestKeys()` | 返回该协议入站解析实际认领的顶层 JSON 键集合（反射请求 DTO 的 json tag 派生，非手写清单） | 各协议子包（`openai/claimed_keys.go`、`responses/claimed_keys.go`、`service/anthropic/request_dto.go`） | openai / openai-res / anthropic（anthropic 的 DTO 定义在真正做解析的 `service/anthropic`，`transform/anthropic/adapter.go` 只转发） |
| `ClaimedJSONKeys` / `TopLevelJSONKeys` / `UnknownTopLevelKeys` | 「入站 raw 顶层键 − DTO 已认领键」的计算原语，纯函数无状态；求差**先精确、再忽略大小写**（与 encoding/json 字段匹配同口径，见下） | `service/transform/shared/json_keys.go` | 同文件 |
| `UnknownRequestKeys(style, rawBody)` + `ErrClaimedKeysUnsupported` | 按**入站** style 分派到对应认领键集合再求差，是上述原语对外的唯一可消费入口；未注册的 style 返回 `ErrClaimedKeysUnsupported` 而非空集合 | `service/transform/unknown_request_keys.go` | 同文件（`claimedRequestKeysByStyle` 注册 openai / openai-res / anthropic） |

## 5. 特殊约定

- 转换矩阵以统一模型为中枢，避免 N×M 直接互转
- 同格式路径可短路；流式与非流式分路径
- **入站请求 DTO 的字段容器统一在 `shared/optional_fields.go`（宽容 + 三态）**：`Optional[T]` / `OptionalNumber[T]` / `OptionalStringSeq` / `OptionalStringMap` / `OptionalInt64Map` / `RawArray` 的 `UnmarshalJSON` **一律不返回错误**（类型不匹配等同「没传」，`Set=false`），数值**必须**用 `OptionalNumber` 而非 `Optional[int]`。**完整理由见该文件头注释**（此处不复制，两份必然漂移），语义由 `optional_fields_test.go` 钉成可执行断言。openai 与 anthropic 入站共用同一份，**禁止**在协议子包内再复制一套。副作用：顶层键匹配走 `encoding/json` 的「先精确、未命中再 `EqualFold`」，故三协议入站顶层键名**均大小写不敏感**（`{"Max_Tokens":100}` 可解析），与 `UnknownTopLevelKeys` 的认领判定口径一致
- **流式 usage 时序（OpenAI 上游）**：`stream_options.include_usage` 的 usage 在 `finish_reason` **之后**单独发一个 `choices:[]` 的尾部 chunk，这是 OpenAI 规范行为而非个别上游怪癖。因此 `streaming/openai_to_responses.go` 不在看到 `finish_reason` 时立即发 `response.completed`，而是把事件缓存进 `state.pendingCompleted`、把各 chunk 的 usage 收进 `state.pendingUsage`（`captureOpenAIUsage`，`len>0` 才覆盖，故 finish 包的 `usage:null` 不会清掉真值）。终态在 usage 尾包**到达时**即交付（`flushPendingOpenAIToResponsesCompleted`），`[DONE]` 与流末 finalize 退化为幂等兜底——三入口清空 `pendingCompleted` 保证只发一次。usage 尾包到达就 flush（而非等 `[DONE]`）是活性保障：避免上游发完 usage 后挂住连接时客户端空等，也避免第二跳 anthropic 拿到未关闭的 content_block。空 `choices` 分支**不得**在收走 usage 前提前 return。`buildResponsesUsage` 透传尾包里的 `prompt_tokens_details.cached_tokens` / `completion_tokens_details.reasoning_tokens` 真值，不写死 0
- **`realtimeStreamState.finalize`**：为上述延后发送引入的流末收尾钩子，由路由 handler 在确有延后内容时设置，`transformStreamBodyRealtime` 在 scanner 结束时调用一次；上游读取出错（`scanner.Err()`）时也会尽力调用一次以交付终态（下游 pipe 此时仍可写；`flushEvent` 出错分支则不调，因那是写下游失败）。其它 realtime 路由无延后需求，`finalize` 保持 nil
- **Anthropic 出站的 usage 权威位置是 `message_delta.usage`**：`message_start.usage` 在 OpenAI 上游场景只能兜底 `1/1`——OpenAI 到流末才给 `prompt_tokens`，而 `message_start` 必须立即发。这是协议桥接的固有限制而非缺陷；`service/chat` 的 anthropic processer 同样只认 `message_delta.usage`
- **Anthropic 入站的 usage 拆在两个事件里**：`message_start.message.usage` 给 input 侧（`input_tokens` / `cache_read_input_tokens` / `cache_creation_input_tokens`），`message_delta.usage` 给 `output_tokens`；旧版上游的 `message_delta` 不重复带 input 侧。`streaming/anthropic_to_responses.go` 用 `state.anthropicStartUsage` 跨事件缓存 message_start 的 usage，在 message_delta 合并，否则转换后 input 侧 token 全为 0
- **转换层向落库侧旁路上游原始 usage（`models.TransformSideChannel`）**：转换输出受目标协议表达能力限制（Anthropic 无 reasoning_tokens 槽位、`message_start` 时序早于真实 prompt_tokens），从下游流反解必然有损，因此在**读上游那一跳**就地捕获。
  - 流式：`streaming/upstream_usage.go` 只管「哪一跳、什么时候把上游 usage 交出去」（`captureUpstreamUsageMap` / `captureUpstreamUsageResponses`）；**归一本身委托 `models.UsageFromMap`**，本包不再自带解析。不按「方向」铺开，避免 N×M 份重复解析。多跳（provider → openai-res → client）**只有第一跳**持有侧信道，第二跳传 nil，否则捕获到的是中间格式而非上游真值。
  - 非流式：`transform_provider_response.go` 在 `providerAdapter.ParseResponse` 后直接 `SetUpstreamUsage`——解析上游响应即得到，无 goroutine、无时序问题。
  - 侧信道语义是「最后一次**有效**观测胜出」：**允许多次调用**，但后写的快照必须是先写快照的**超集**，否则会把已观测到的字段覆盖成 0。Anthropic 正是分两次交——`message_start` 先交 input 侧（上游若在此后异常结束，这是唯一一次机会），`message_delta` 再交含 input 侧的合并快照。之所以不做逐字段合并：合并需要区分「上游报了 0」与「上游没报」，map 解析后两者不可分，把超集责任交给最了解协议时序的转换器更可靠。全零快照的丢弃统一收口在 `models.TransformSideChannel.SetUpstreamUsage`（判据 `models.Usage.HasTokens`），各捕获点无需自行判空。落库侧消费见 [logs-metrics.md](logs-metrics.md) 的 `UsageSource`。
- **上游 usage 归一是全仓单一入口 `models.UsageFromMap`**（`models/usage_normalize.go`）：字段容错走有序候选路径表（`usageFieldCandidates`），不是 per-provider if-else，新增一种非标准写法只加一行候选。已覆盖 usage 顶层 `reasoning_tokens`、`prompt_cache_hit_tokens`、Anthropic 的 `cache_read_input_tokens`、两侧 `audio_tokens` 等。四个调用方共用同一份：流式侧信道（`streaming/upstream_usage.go`）、非流 anthropic 入站（`service/anthropic/response_inbound.go`）、`service/chat/process.go` 的三个 processer（合并为单一 `parseUsageJSON`）。**归一落在 `models` 而非 `common`/transform**：产出即 `models.Usage`，且 `service/chat` 已 import `service/transform`（反向复用会成环），共享点只能在两边都能依赖的最底层。候选表排序规则是**嵌套标准位置 > 协议原生专有字段 > 顶层泛化兼容字段**（kimi 一类混合返回里 `input_tokens` 与 `prompt_tokens` 并存时原生胜出）。**边界**：这里管的是「raw upstream usage map → `models.Usage`」；`service/transform/openai/response.go` 与 `service/responses/response_parse.go` 里的 usage 映射输入是已解码的协议 DTO（→ unified），不走本入口，不属同一件事。给 `models.Usage` 加新明细字段时**必须**同步候选表与 `UsageFromMap`——漏一个字段该项统计恒为 0 且无人察觉
- **`ResponsesUsage` 的 details 键名必须是复数**（`input_tokens_details` / `output_tokens_details`）：这是 OpenAI Responses 的线格式，streaming 编码侧与 `models.UsageFromMap` 的 `output_tokens_details.reasoning_tokens` 候选路径都依赖这个写法。曾误写单数 `input_token_details`，导致解码恒 nil（读侧成死代码）且编码输出不合规
- **出站 usage 与入站同口径：独立尾包**：`streaming/responses_to_openai.go` 给 openai 客户端出站时，`finish_reason` 那包**不带** usage，usage 单独发一个 `choices:[]` 的尾部 chunk（仍带 `id`/`object`/`created`/`model`——是完整 chunk 而非裸 usage 对象），随后才是 `[DONE]`。与上方入站侧的规范认定对称：出站若塞进 finish 包，同一 codebase 就对同一协议事实持两套相反假设。usage 对象由 `openAIUsageFromModel(usageFromResponses(u))` 构建（见下条）。`captureUpstreamUsageResponses` 仍在该分支内恰好调用一次——落库值来自侧信道，与出站 chunk 形状无关。注：`service/chat/process.go` 的 openai processer `streamUsageExtract` 逐 chunk 扫 usage、不假设位置，故 downstream 兜底路径不受影响。`anthropic → openai` 走 anthropic→responses→openai 双跳，出站形状同此
- **出站 usage 装配是 streaming 内单一入口 `usage_wire.go`**：三个写出点（`anthropic_to_responses.go` / `openai_to_responses.go` 的 `response.completed`、`responses_to_openai.go` 的 usage 尾包）都走 `usageWireFromModel(u models.Usage, k usageWireKeys)`。两条协议线的差异**只有键名**（Responses 线 `input_tokens`/`output_tokens`/`*_tokens_details`，OpenAI Chat 线 `prompt_tokens`/`completion_tokens`/`prompt|completion_tokens_details`），故用键名表参数化 + 两个薄封装 `responsesUsageFromModel` / `openAIUsageFromModel`，新增协议线只加一份键名数据（OCP）。统一后的契约：total 一律过 `models.ResolveTotalTokens`（幂等，调用点不必自判）；**零值 details 不写出**，判据只看 detail 本身 `>0`，不附加父级 `promptTokens>0`/`completionTokens>0` 前置条件——与入站归一 `models.UsageFromMap` 同口径，否则 `cached>0 && prompt==0` 这类异常上游下两侧发散；**audio 明细有意不写出**（客户端契约未含，落库侧仍记录），要补须同时定下各线键名。**空值语义留在调用点**（各写出点「没有 usage」的判据不同）。强类型 `responses.ResponsesUsage → models.Usage` 由同文件的 `usageFromResponses` 承担，出站写出与侧信道 `captureUpstreamUsageResponses` 共用，不再各摊一次 map



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
- **budget-only 钳制**：客户端只给 budget、不给 effort（responses 入站 `reasoning.max_tokens` 与 effort 解耦，是唯一可达来源）时，budget 仍受白名单约束——上限取**白名单最高档**对应的 budget（`models.HighestEffortInWhitelist` + `ReasoningEffortToThinkingBudget`），超限钳到上限；白名单不含任何正向 6 档 → 剥离 thinking；白名单空则按 `unknownStrategy` 分流（passthrough 不约束、clamp_to_default 以 autoFallback 的 budget 为上限）。上限口径**不**把 budget 反推成 effort（反推有损：513..19999 全塌到 low），只钳数值 / 剥离，**不写回** effort（避免凭空补客户端没给的字段）。两条路径共用 `transform.BudgetLimitForClamp`。
- **分路径钳制**：
  - transform 路径：`ProcessRequest(ctx, raw, clamp)` 在 ToUnified 后、FromUnified 前对 `unified.ReasoningEffort` 钳制（`clampUnifiedReasoning`）；effort 为 nil 时转入 `clampUnifiedBudgetOnly`
  - passthrough 路径：`clampPassthroughReasoning` 对 raw body 按 style 钳制 effort 字段（OpenAI `reasoning_effort`、Anthropic `output_config.effort`、Responses `reasoning.effort`）+ budget 字段（Anthropic `thinking.budget_tokens`、Responses `reasoning.max_tokens`）；effort 字段缺席时转入 `clampPassthroughBudgetOnly`。**大小写归一化**：两条路径都在钳制前对 effort 做小写归一化（transform 经 `NormalizeReasoningEffort`，passthrough 经 `strings.ToLower`），避免客户端传 "HIGH" 时白名单命中失败。passthrough 路径在归一化后若值在白名单内但原始大小写不规范，会写回归一化小写值。**剥离**统一走 `stripPassthroughThinking`：删 effort + budget 字段后还要删 `passthroughThinkingContainers` 列出的容器（Anthropic 的 `thinking` 只有 type + budget_tokens，只删 budget 会残留 `{"type":"enabled"}`，上游会 400）。
- **Responses 的 `metadata` 不是 effort 通道**：`metadata` 是客户端自由 KV 标签（OpenAI 限 16 键、上游不解读其中的控制语义），出站 emit、入站解析、passthrough 钳制三处都**只**认官方 `reasoning.effort`；`metadata` 内即便出现同名 `reasoning_effort` 键也按普通标签原样透传。llmux 曾额外写/读 `metadata["reasoning_effort"]`，因无人消费且白占客户端命名空间一格而移除。
- **协议级 budget < max_tokens 收敛（独立于白名单钳制）**：Anthropic 要求 `thinking.budget_tokens` 严格小于 `max_tokens`（后者是思考+回答总额）。该收敛不属于档位白名单，落在 chat 层的 `reconcileThinkingBudgetWithMaxTokens` 且在 `clampMaxTokens` **之后**执行（`clampMaxTokens` 正是非法组合的制造者）；两条路径共用同一实现。规则与阈值见 `chat-gateway.md`。**注意**另一条独立触发源：`service/anthropic/request_outbound.go` 在 `max_tokens` 缺失时硬填 8192，仅此默认值就足以让 effort ≥ medium（budget ≥ 20000）落进非法区间，与 `MaxTokensLimit` 是否配置无关。

---

### 入站未知字段检测（键集合原语）

- **要解决什么**：一眼看出「客户端发来的请求里有哪些顶层键是本网关根本没解析的」——这些键在转换后会静默消失。
- **为什么不比对入站 body 与出站 body 的键差**：改名与丢失在输出侧不可区分（`unified.Stop` 出到 anthropic 叫 `stop_sequences`、出到 openai 叫 `stop`），靠手工白名单补这个信息缺口等于维护转换逻辑的第二份副本，必然漂移成误报。检测点前移到入口侧——「DTO 认领了哪些键」从 struct tag 反射派生，是事实而非副本，DTO 改了自动跟着改，零维护且不受出站改名影响。
- **分层**：`shared/json_keys.go` 提供不认识任何协议的通用原语（`ClaimedJSONKeys` 反射 struct tag、`TopLevelJSONKeys` 提 raw 顶层键、`UnknownTopLevelKeys` 求差）；各协议子包用 `ClaimedRequestKeys()` 传入自己的请求 DTO 零值；顶层 `unknown_request_keys.go` 用 `UnknownRequestKeys(style, rawBody)` 把两者接成可消费入口。依赖方向由 import 倒逼：shared ← 协议子包 ← 顶层 transform。
- **style 分派用独立可选注册表，不进 `FormatAdapter`（ISP）**：认领键是**入站**解析的性质，而 `FormatAdapter` 同时管出站；塞进接口会让「这个协议答不答得了」变成运行时约定而非编译期事实。独立注册表让「未注册 = 不支持」成为编译期事实，新协议补上请求 DTO 后只需加一行注册（OCP）。当前三个 style 全部已注册。
- **未命中时刻意不回退默认协议**：与 `getAdapterOrDefault` 的 fallback 形状相反。拿 openai 的键集合去查 anthropic 的 body 会把 `system` / `stop_sequences` 整片误报，正是 `TestUnknownTopLevelKeys_OutboundRenameIsNotUnknown` 钉死要防的退化。哨兵错误 `ErrClaimedKeysUnsupported` 而非空切片，是因为空切片与「确实没有未知字段」不可区分，消费方会把「查不了」显示成「已检查、无问题」。
- **三个 style 全覆盖，但 anthropic 的 DTO 只到顶层**：`service/anthropic/request_dto.go` 的 `anthropicRequest` 认领 13 个顶层键（model / stream / max_tokens / temperature / top_p / stop_sequences / metadata / system / messages / tools / thinking / output_config / tool_choice），嵌套结构停在 `json.RawMessage`、内部仍走既有 map 解析——检测只看顶层，嵌套 struct 化对它零贡献。**纪律**：`ClaimedRequestKeys` 反射的是 tag 而非「字段有没有被读」，往 DTO 加字段与把字段接进 `UnifiedRequest` 必须同一次改动，否则会把真实缺口伪装成已解决。anthropic 的 `top_k` / `service_tier` / `container` / `mcp_servers` 等当前确实未解析，被报出来是正确行为。
- **消费方必须用「入站 style」的键集合**：用出站协议的键集合去查会把改名字段误报成未知，退化回被否掉的旧方案。生产消费方是**日志详情接口的读时计算字段**（`unclaimed_request_fields`，见 [logs-metrics.md](logs-metrics.md)）：入参取 `ChatLog.Style` + `ChatLog.RawRequestBody`，不落库、不打日志、不进热路径。
- **passthrough 路径无转换丢失**：`style == providerType` 时（`chat_attempt_request.go`）请求不进 transform，原样透传。对这类日志跑检测的语义是「网关 DTO 不认领的键」，而非「转换丢失的键」，消费方展示时须区分措辞。
- 只看顶层：嵌套层（`messages[].xxx`）的键归属需要逐个子 DTO 的映射知识，不在原语职责内。
- **求差与 encoding/json 同口径（先精确、再忽略大小写）**：`json.Unmarshal` 对键的匹配是「精确优先、精确未命中再忽略大小写」，故 `{"Temperature":0.7}` 会被正常解析进 `temperature` 字段。而认领集合按精确 tag 名派生，若求差只比精确名，这类键会被报成未知——诊断一上线就假报。`UnknownTopLevelKeys` 用 `strings.EqualFold` 线性兜底折叠（借标准库折叠语义，不自造归一化），精确未命中再折叠匹配。低频诊断路径，键数量级下 O(n·m) 无意义。回归测试 `TestUnknownTopLevelKeys_MatchesEncodingJSONCaseFolding` 先用标准库证明变体确实被解析进同一字段，再断言其不进未知集合——标准库若改掉折叠回退，前置 DeepEqual 先失败，区分「前提变了」与「实现错了」。

---

### 跨格式转换字段覆盖矩阵（golden）

- **要解决什么**：一眼看出「统一模型每个字段在各协议出站时 emit 成什么 / 被哪个 style 吃掉」，且转换器改动时自动亮出 diff。与上一节的入站未知字段检测互补——那个查「客户端发来但网关没认领的键」，这个查「网关认领了但出站丢掉的字段」。
- **两个矩阵，各自 fixture × style**（style 列表均为 `openai` / `openai-res` / `anthropic`）：
  - 请求侧 `TestFromUnifiedCoverage_Golden`（`from_unified_coverage_test.go`）→ `testdata/golden/from_unified/{fixture}/{style}.json`，fixture 为 `full` / `embedding` / `reasoning_effort_only`
  - 响应侧 `TestFormatResponseCoverage_Golden`（`format_response_coverage_test.go`）→ `testdata/golden/format_response/{fixture}/{style}.json`，fixture 为 `full` / `error_only` / `multi_choice`
- **刷新方式**：`go test ./service/transform/ -run TestFromUnifiedCoverage_Golden -update-golden`（响应侧换测试名）。golden 由 `assertGoldenJSON` 规范化后写入（2 空格缩进 + 键字典序），**禁止**手改。
- **直调 adapter 而非走 ProcessRequest / TransformProviderResponse**：`FromUnified` 与 `FormatResponse` 都是纯函数（无 ctx、不读设置），测的是纯字段映射终态。串联入口会把钳制（`clampUnifiedReasoning`）或 `ParseResponse` 的字段丢失混进来，让「字段去向」不可读——尤其响应侧，`openai.ParseResponse` 根本不填 `Logprobs` 与四个 reasoning 字段，走串联入口结构性地覆盖不到这些路径。
- **与 `TestGolden_{Request,Response}Conversions` 不重叠**：那两个是 `from × to` 跨协议矩阵（输入是协议原始 body 文件，排除同 style），本矩阵是 `fixture × style`（输入是 Go 满配 fixture，含同 style 出站）。前者验端到端，后者验单向映射完整性。
- **错误也进 golden**：`FromUnified` / `FormatResponse` 返错时包成 `{"__from_unified_error__": ...}` / `{"__format_response_error__": ...}` 冻结，而非 `Fatalf`。某些路径的当前行为就是返错（如 embedding 出站未接线），这本身是覆盖矩阵要记录的现状；接线后 golden 从 error 变真实 body，diff 自证。**不**引入「哪个 style 期望 error」的期望表——那是 adapter 逻辑的第二份副本，必然漂移。
- **styles 刻意硬编码而非 range 注册表**：新协议注册后应人工审一遍出站形状再纳入基线，自动全跑会静默接受未审输出。
- **防腐化守卫**：`TestFullUnifiedRequest_CoversAllSerializableFields` / `TestFullUnifiedResponse_CoversAllSerializableFields` 要求满配 fixture 的字段全员非零，豁免项写进 `exempt` 表且**反向锁定为零值**（豁免字段被填值同样报错，说明豁免理由已失效）。新增字段忘了填 fixture 时守卫失败，否则新字段会静默游离在 golden 覆盖外。
  - 两者的**遍历深度刻意不同**：请求侧只查顶层（`UnifiedRequest` 41 个顶层字段，单层已是实质约束）；响应侧**递归下降**（`UnifiedResponse` 只有 9 个顶层字段，实质内容全在 `Choices[].Message.ToolCalls` / `Usage.*Details` / `Logprobs` 三层，单层 IsZero 只要 `Choices` 非 nil 就通过，等于没有守卫）。递归报错按字段路径定位，如 `Choices[].Logprobs.Content[].TopLogprobs[].Token`。
  - 递归的叶子边界是 `interface{}`（`UnifiedMessage.Content`）：其背后是 `text` / `image_url` / `input_audio` 互斥的 sum type，单个 fixture 值无法同时填满所有变体，多模态出站映射另由 `TestTransformProviderResponse_OpenAIToAnthropic_MapsMultimodalImage` 等专项测试覆盖。
- **不覆盖流式**：流式响应（`streaming.TransformResponseRealtime`）从不构造 `UnifiedResponse`、从不调 `FormatResponse`，是独立状态机 + 4 条注册路由，与本矩阵零共享。改一侧不会自动同步另一侧，这是已存在的漂移风险面。

---


