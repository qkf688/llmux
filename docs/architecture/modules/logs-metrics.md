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
| `chatLogResponse` / `chatIOResponse` | `/api/logs*` 与 `/api/logs/:id/chat-io` 的对外响应契约（全 snake_case），与 `models.ChatLog`/`ChatIO` 解耦；含两个读时计算字段 `unclaimed_request_fields` / `mismatched_request_fields`（见下）；S3-3 起含四个调度明细键 `endpoint_protocol`/`endpoint_url`/`key_group_name`/`credential_note`（映射 `ChatLog` 同字段，前端 logs 展示层已按同名键消费）；#4-3 起含两个密钥归因键 `api_key_id`/`api_key_name`（映射 `ChatLog` 同字段，见下） | `handler/logs/dto.go` | 同文件的 `buildChatLogResponse` / `buildChatIOResponse` / `computeRequestFieldDiagnostics`（`computeUnclaimedRequestFields` / `computeMismatchedRequestFields` 各传一组检测函数 + 哨兵） |
| `ChatLogRepo` / `ChatIORepo` | 日志与 IO 持久化访问 | `repository/chat_log.go` | GORM |
| `StatsRepo` | 六张 Stats* 表的写入（`AddTimeBased`/`IncModelCalls`/`IncRealModelCalls`/`AddProviderStats`）与读侧聚合（`GetTotal`/`SumDailiesSince`/`List*`） | `repository/stats.go` | GORM（`clause.OnConflict` upsert） |
| `RecordLog` / 落库编排 | 请求结束后 processer + 落库；processer 失败分支是**响应已开写后**所有后处理失败的唯一收口（写 `Status=error` + 一条留痕），并据入参 `ServingModelWithProviderID` 与失败分类决定是否清除该对话的粘性钉定（见 chat-gateway §5「提交后失败清钉」） | `service/chat` | 同包 |
| `chatstats.Record*` | 请求/token/供应商统计累加 | `service/chatstats/` | 同包 |
| `EnforceRetentionByOldestID` | 按策略裁剪旧日志 | `models/retention.go` | 同文件 |

## 5. 特殊约定

- **对外响应必须经 `handler/logs/dto.go` 的 DTO，禁止裸序列化 `models.ChatLog`/`ChatIO`**。理由是历史教训而非洁癖：`ChatIO` 曾直接 `httpresp.Success(c, chatIO)` 返回，GORM 模型的 Go 字段名就是 JSON key，改一个字段名（`LogId`→`LogID`）即破坏前端契约；`ChatLog` 曾用 `map[string]any` 手拼，漏写 `completion_tokens_details` 导致 reasoning_tokens 落库有值但 API 永不返回、前端类型声明成为谎言。结构体 DTO 让编译器兜住字段齐全，模型层字段改名不再波及 API
- API 字段一律 snake_case（含 `id` / `created_at` / `chat_io` 等，见 [conventions.md](../conventions.md)）；`raw` 六字段用 `*string`+`omitempty`——空字符串是合法值，必须与「include_raw=false 未返回」区分
- **头快照落库即脱敏 + 存量清洗（Stage 16 立项，Stage 26 扩两列，2026-09-27）**：请求与响应头快照均在写侧 marshal 前过敏感头剥除，判据单源 `common.MaskHeadersWithDenyList` / `MaskHeadersWithDenyListJSON`（`common/headerpassthrough.go`，deny-list = `Authorization`/`Cookie`/`x-api-key`/`Proxy-Authorization`/`Set-Cookie` canonical 五键）——敏感头**键保留、值整值 `****`**（保留「带没带这个头」的排障信息），非敏感头原样。**请求列**接线点唯一 = `service/chat/chat_attempt_log.go` 的 `captureRequestLogSnapshot`（第 6 参 `customHeaderKeys` 动态并入本次请求实际配置的自配头名——关联级 `customer_headers` + key 级自定义头键名并集，`customCredentialHeaderKeys` 提取；Stage 26 收口原「自配头名盲区」），全仓 RequestHeaders 只有这一个写入源。**响应列**接线点唯一 = `service/chat/upstream_body_limit.go` 的 `responseHeadersJSON`（成功路径 `updateRequestAndResponseLog` 与失败出口共用；Stage 26 落地前响应头明文落库）。**存量清洗**：启动装配时 `ChatLogRepo.SanitizeLegacyLogSnapshots`（`repository/chat_log.go`，两列统一入口；`main.go` 在 BootstrapAdmin 之后、`startBackgroundServices` 之前调用并传入 `collectConfiguredHeaderKeys` 现存自配头名全集，位置由 `main_startup_sanitize_gate_test.go` 机械门锁定）——游标分批（500/批）、幂等（已清洗行零改写）、坏 JSON 行 `slog.Warn` 跳过不阻断启动、只改值不删行、行级去重（同一行两列都命中算 1）；**实际改写行后追加 `VACUUM`**（物理抹除，Stage 16 risk-review C1 实测支撑；回归 = `repository/chat_log_sanitize_stage26_bytes_test.go` 双列字节扫描）。**清洗局限（文档化）**：请求列 extraKeys 以现存配置为界，已删配置的历史自配头名洗不到；响应列静态名单可洗全。契约测试：写侧 `service/chat/chat_attempt_log_sanitize_test.go` + `chat_request_headers_custom_mask_test.go` + `chat_response_headers_mask_test.go`、清洗 `repository/chat_log_sanitize_test.go` + `chat_log_sanitize_stage26_test.go`
- **调度明细四字段（`endpoint_protocol` / `endpoint_url` / `key_group_name` / `credential_note`）在 `service/chat` 建行时从选路结果 `Selection` 填充一次**（`executeSingleProviderAttempt` 的 logEntry）：失败出口全部走 `struct Updates`（零值跳过），不会用空串覆盖这些字段。`credential_note` 的取值规则是「Note 非空用 Note，否则 `KeyHash[:8]` 加 `#` 前缀」（见 chat 包 `credentialLabel`）——组内多 key 排查需要在日志里区分命中哪条 key，KeyHash 是不解密即可获得的稳定脱敏标识。`enrichChatLogs` 的格式转换判定同样消费 `EndpointProtocol`（与 chat 链路透传判定取数点同源），字段为空（S3-3 之前存量行）时判无转换
- **密钥归因两列（`api_key_id` / `api_key_name`）是调用方 key 的写入时快照**（多 api-key 方案 §4，#4-3）：传递链 `middleware.CurrentCallerKey(c)` → `handler/v1` 的 `newReqMeta` 翻译成 `models.ReqMeta.ApiKeyID/ApiKeyName` → **全部建行点**统一填充；失败出口是 Update 非建行，快照随行自带。`api_key_id` 为 `*uint`：null = 无归因（内部路径/存量行），DTO **禁 omitempty**（键恒在，前端可区分「无归因」与「后端漏返」）。名称是快照不是外键——key 改名/删除历史日志不回写（契约测试 `TestChatLog_AttributionSnapshot_ImmuneToKeyRenameAndDelete` 钉死，加 FK/JOIN/级联即红）；按 key 筛选与 api_key_id 索引均未做（方案 §4 定案，有需求再列）
- **请求编号列（`request_id`，Stage 17）**：请求 rid 的写入时快照，传递链 `middleware.RequestID`（值已带来源前缀：自生成 `gw:`+12hex / 沿用 `in:`+原值，见 framework-conventions §5「请求编号」）→ `handler/v1` 的 `newReqMeta`（gin context → `models.ReqMeta.RequestID`）→ **全部建行点**统一填充（`chat_attempt.go` 的 logEntry + `chat_log_storage.go` 的 preProviderFailureLog；retryLog 通道行由 logEntry 派生自动携带）；失败出口是 Update 非建行，struct Updates 零值跳过保证快照不被空串覆盖。DTO `request_id` 键恒在、**不用 omitempty**（string 无三态语义；存量行/内部路径空串 = 无编号，键在值空同 `api_key_id` 口径）；契约测试 `handler/logs/request_id_contract_test.go` 断响应体键在与值形（列表+详情双路径）
- **失败出口/建行写库一律不陪葬请求 ctx（Stage 17，审查回批后全写点收口）**：chat_logs 全部写库点经同一口径——派生表达式单源 `chatLogDetachedCtx`（`service/chat/chat_log_storage.go`，`context.WithoutCancel` + `chatLogWriteTimeout` 5s）。收口覆盖：① 失败出口回写 `updateChatLogByID`（`chat_attempt_log.go`，`Client.Do` 错误/转换失败/非通过状态码/体积闸超限/读体错误/成功路径 raw 回写 6 调用点 + **审查回批并入的 `chat_record.go` processer 错误分支**，此前该处直调靠调用方恰好传 `context.Background()` 保平安、无防线）；② 建行 `SaveChatLog`；③ 后处理路径 `persistChatLog`/`createChatIO`/`maybeClearRawOnSuccess`（`chat_record_persist.go` 内部派生，err 传播语义不变——该路径失败要中止后处理，不走 wrapper 的「仅 slog」口径）。**为什么**：超时/客户端断连正是请求 ctx 已死、最需要留痕的时刻，陪葬会让行停在建行占位的 `success`（建行即写 success 占位、成功出口不显式写终态）——日志页/指标把超时计为成功，`maybeClearRawOnSuccess` 还会按行状态分流清理 raw。同口径延伸：**失败分支的设置读取**经 `withDetachedLogCtx`（`chat_attempt.go` 的 raw 开关/上限读取、`chat_attempt_error.go` 的错误体截断上限）——死 ctx 下 GORM 读设置退默认值，管理员配置恰在失败场景静默失效（审查回批 W2）。`updateChatLogByID` 对 `logID==0`（`disable_all_logs`）显式短路；写库姿势保持 struct Updates（零值跳过是调度明细/归因快照穿透的设计依赖）。回归：`upstream_body_limit_test.go` 的 deadline 用例断行终态 `Status=error` + rid 快照（改前实测红）+ `chat_log_write_detach_test.go`（cancelled ctx 两写点 + id=0 用例——后者锁「不误伤其他行」，短路分支本身不可由该用例区分）。**已知残余窗口**：detached 写自身失败/5s 内未完成时（SQLite 锁竞争、磁盘故障）行仍停 success 占位，观测面仅 `slog.Error`（带 log_id）——写库基础设施坏掉时无完美解，属明示权衡。**已知未收口邻接面**：同失败分支的 `applyProviderFailureAdjustments`（权重/优先级）与 `resetCredentialAuthFailCount` 仍用请求 ctx 直写（credwrite 主链已走 Background 队列不受影响）——已登记 find-issues，批次未定
- **「未选出供应商即失败」的四类出口走同一拼装 `savePreProviderFailureLog`**（`service/chat/chat_log_storage.go`）：虚拟选模失败、模型不存在、虚拟跳过某真实模型（`saveVirtualSkipLog`）、真实模型无启用供应商。（另有第五个建行点——`executeSingleProviderAttempt` 的 logEntry，它是**已选出供应商**的尝试行，字段口径不同，不走本拼装。）字段口径（`preProviderFailureLog`）只定义在该结构体一处，补字段时不会被漏改；写库失败只记 slog、不改写调用方拿到的错误。**可判别特征**：这四类的 `ProviderName` 恒为空（`ProviderModel` 仅虚拟跳过那类填真实模型名）——列表页「提供商」列为空即代表「请求没打到上游」。**真实模型无启用供应商**那条的 `Error` 在严格能力匹配开启时会附一句提示（关联可能被 tool_call / structured_output / image 需求筛掉），关档时不附
- **超时与调用方撤走的终态归因可区分（Stage 33）**：网关总超时到点后 chat_logs 终态写 `Status=error`、`Error` 带 deadline 错误文本（如 `context deadline exceeded`），与「调用方放弃」（`context canceled`）在行上可分辨；留痕级别判定 `service/chat.isClientDisconnectErr` **显式排除超时类**（`DeadlineExceeded` / `net.Error.Timeout()`），超时不再被断连降级口径吞成 Warn 同形留痕（约定见 framework-conventions §5）。回归：`service/chat/chat_timeout_attribution_test.go`
- **「窗口掐断」与「上游自己断的」在 `Error` 文本上可分（Stage 38，2026-10-03）**：客户端原文只有一句 `... net/http: timeout awaiting response headers`，看不出是我们自己的单次尝试窗口到点还是上游异常，故 `service/chat/chat_attempt.go` 的 `Do` 失败出口先过 `describeUpstreamWindowTimeout`——命中「`net.Error.Timeout()` + 请求 ctx 未到点 + 原文含 net/http 等响应头超时标记」三条时，`Error` 前缀「等待上游响应头超过 N 秒（流式等头窗口，网关主动掐断）」或「等待上游响应头超过 N 秒（非流式整段预算窗口：上游通常要生成完整回答才发响应头；网关主动掐断）」，原始错误经 `%w` 保留在同一条文本里（链不断，`errors.Is/As` 判据照常）。类别与秒数取自调用方实际施加的那个窗口（`upstreamWindow` 值 + 类别同源带出），文案不自行判断入站形态。三类不贴：上游自己断连、连接/握手调优常量超时（dial 30s / TLS 10s）、请求 ctx 已到点（总超时族）；窗口未知（值为 0）时也不贴，避免贴错类别。**文案只声称机制做到的事**：窗口由 transport 的 `ResponseHeaderTimeout` 施加、覆盖到「收到响应头」为止，故不写「等整段回答」——上游先发头再慢写 body 的形态不含该标记，不会被贴类别（该形态只受总超时约束）。回归：`service/chat/chat_stream_window_split_test.go`（端到端两分支 + 判据边界表）
- **三条刻意不入库的失败出口（Stage 7 定案的边界，非缺陷）**：入站请求体超限（413）、预处理失败（400，含 JSON 坏掉与缺 model）、**真实模型解析路径**上查供应商时数据库自身出错（500）。前两者连「请求哪个模型」都未解析出，与「谁用哪把 key 打了哪些模型」的归因线无关；后者是系统故障，服务端日志已有留痕。审计「密钥-模型」维度不需要它们；也不为它们建行，以免为「无法归因的行」增加筛选噪声。注意**虚拟路径**同一处的 DB 错误不属此列——它走 `saveVirtualSkipLog` 建行（`balanceChatVirtual` 的查询失败分支），原因见该处
- **`unclaimed_request_fields` 是读时计算的诊断字段，不落库**：指出客户端原始请求体里有哪些顶层键本网关根本没解析（转换后会静默消失），供开发/运维据此去补 transform。入参是 `ChatLog.Style` + `ChatLog.RawRequestBody`，调 `transform.UnknownRequestKeys`（见 [protocol-transform.md](protocol-transform.md)）。
  - **不落库的理由**：DTO 会随 transform 演进，落库的结论会僵化成「按当时 DTO 算的旧答案」；读时算永远反映最新代码，且日志详情是低频页面，反射 + JSON parse 的开销可忽略。
  - **与 raw 组同一个 `includeRaw` 门控**：它的输入就是 `RawRequestBody`，`include_raw=false` 时该列压根没从库里读出来（`ChatLogRepo.List` 的 Omit），此时算出来的只会是假的 `raw_not_recorded`。
  - **用 `status` 枚举而非「nil / 空数组」表达结果**：`ok`（`fields` 即结论，可能为空数组）/ `raw_not_recorded`（开关关着或 `errors_only` 在成功时清空了 raw）/ `style_unsupported`（该入站 style 未注册认领键集合；三个生产 style 现已全部注册，故此态**当前无生产可达路径**，保留是为新 style 落地期——前端对应文案同理，清理死代码时别删）/ `parse_error`（raw 不是 JSON 对象，`detail` 给原因）。「查不出来」有三种彼此需要区分的原因，挤进一个可空数组的话前端只能猜，最坏是把「查不了」显示成「已检查、无问题」——假阴性比没有这个功能更糟。
  - **语义边界**：结果是「网关 DTO 不认领的键」，不等于「转换过程中丢了这些键」——passthrough 路径（`style == provider.Type`）请求不进 transform，这些键实际原样透传。且只看顶层，`messages[].xxx` 不在范围内。前端措辞须说清这两点
- **`mismatched_request_fields` 是第二个读时计算诊断字段，与上者同形不同因**：指出哪些键**被 DTO 认领了、但客户端把值类型写错**，被 `shared.Optional*` 容器当成「未传」静默丢弃（请求照常 200、参数不生效）。调 `transform.MismatchedRequestKeys`（原理与覆盖缺口见 [protocol-transform.md](protocol-transform.md)）。
  - **共用同一个四态 `status` 与同一段编排**：两者的 Go 类型是同一个 `requestFieldDiagnostics`，四态判定收在 `computeRequestFieldDiagnostics(log, detect, errUnsupported)`——两份拷贝会漂移，让同一弹窗内两块的「查不了」语义不一致。差异只有两个注入参数：检测函数与「不支持」哨兵。
  - **`style_unsupported` 在两块里语义相反**：未认领检测的该态是**能力缺口**（当前无生产可达路径）；类型不匹配检测的该态是**不适用**——openai-res 已注册认领检测但刻意不注册类型检测，因为它类型写错会直接 400、不存在静默丢弃。这是该态**唯一的生产可达路径**。前端两块的文案因此必须分开写（`mismatchedViewByStatus` 说「不适用」而非「尚未实现」）。
  - **语义边界**：结果是「值类型不符而被容器忽略的键」，不等于上游拒绝了该参数；只看顶层，且只覆盖有三态容器的字段（`json.RawMessage` / `RawArray` 字段无从判断，是已知缺口）。
- `ChatLogRepo.List` 在 `!IncludeRaw` 时用 **Omit 排除 raw 大字段**，不用 Select 白名单：白名单的失效模式是静默的（新增字段忘记加进来 → 该字段恒为零值，调用方无从察觉，`usage_source` 就这样漏过一轮）
- `logs/enrich.go` 允许多表只读富化直连（repository 白名单例外之一）
- 统计表的日期列格式（`2006-01-02`）与「时间维度 = total/daily/hourly 三表」属存储细节，只在 `repository/stats.go` 内出现；调用方一律传 `time.Time`
- `StatsProviderTotal.AvgResponseTime` 存的是**累计**耗时，均值由 `handler/metrics` 除以 `TotalRequests` 得到；补零/除零保护等展示计算不下沉仓储
- 健康/同步日志不归本模块，前端有独立路由 `health-check-logs` / `model-sync-logs`
- `EnforceRetentionByOldestID` 的 `BeforeDelete` 回调与主表删除在同一事务内执行；回调收 `tx *gorm.DB`，必须用 tx 操作子表，返回 error 会回滚整个事务（主表不删）。ChatLog 的 `EnforceRetention` 用此机制保证 ChatIO 与 ChatLog 同生共死，不产生孤儿 ChatIO 行（db 体积单调增长的根因修复）
- `ChatLog.RawResponseBody` 的截断语义（Stage 23 统一后）：**流式转换路径**经 `service/transform/streaming` 的头尾双段累积器（头 512KB + 尾 512KB），超出部分丢弃中间段并插入统一头尾截断标记（`common.SSETruncatedMarker`，文案委托 `common.TruncateHeadTail` 单源）、同时打一条 `slog.Warn`；保留尾段是为了不丢 `finish_reason` / `usage` / `[DONE]` / `message_stop` 等收尾事件。**非流式路径**（`service/chat/chat_attempt_log.go` 的 `captureRawResponseBody`）只采集头部 `raw_log_field_max_kib`（默认 256KiB）并带 `…(truncated)` 尾切标记（`common.TruncateUTF8` 单源）；采集读头不吞尾——MultiReader 回填保证下游转换/直通拿到完整响应（Stage 2-D）。故同一字段可能来自两种策略，排查时以有无截断标记为准；另注意累积体不是字节级保真（多行 data 已被合成单行）
- **回复类日志的体积上限（Stage 23）**：`log.response_log_max_kib`（默认 2048 KiB、下限 16、硬顶 16384）盖三条路径——非流式 `ResponseBody`、流式 `ChatIO` 累积（`service/chat/output_budget_accumulator`，头段封顶 + 尾段滚动，内存与落库双头同受预算）、ChatIO 落库保险闸（`chat_record_persist.go` 的 `capOutputUnion`）。超限统一存「开头一段 + `common.TruncateHeadTail` 统一标记 + 结尾一段」，标记字节计入预算（总长 ≤ 上限）。与 `raw_log_field_max_kib` 的分工：后者管四类**采集**字段（采样口径、256KiB 级），本项管回复**整条**（「看全文」口径、调试开关场景，默认给足原文体量），故独立设键不合并。全项目一切截断标记以 `common/truncate.go` 为单源（`TruncatedSuffix` 尾切 / `TruncateHeadTail` 头尾式），存量行里的旧标记不回写
- **`ChatIO.Input` 请求侧原文的单条上限（Stage 26）**：`createChatIO`（`chat_record_persist.go`）落库前过 `truncateChatIOInput`——预算复用 `raw_log_field_max_kib`（请求侧是采样排障语义，不开新设置项，grilling 拍板），截断同一把刀 `common.TruncateHeadTailUTF8`（头尾保留 + 统一标记，标记字节计入预算）。此前 `in.Before.raw` 原文直落库，入站硬顶 20 MiB（`consts.MaxInboundBodyBytes`）下单行 ChatIO 可达 20 MiB。回归：`service/chat/chat_io_input_limit_test.go`
- **slog 侧 path attr 单值上限（Stage 26）**：`common.SlogPathMaxBytes`（4 KiB，`common/log_limits.go` 单源，与访问行 `accessLogLineMaxBytes` 同族同值）——`middleware/recovery.go` 的 panic 留痕行与 `httpresp/response.go` 的 `logInternalError` 两处 path 写点均过 `common.TruncateUTF8`。病态长 URL（仅受 `net/http MaxHeaderBytes` ~1 MiB 兜底）触发 panic/500 时不再把 MiB 级 path 写进日志行。回归：`httpresp/path_truncate_test.go` + `middleware/recovery_path_truncate_test.go`（两写点各一条直达用例）
- processer 出错时（`service/chat/chat_record.go` 错误分支）只写 `Status`/`Error` 就返回，**不消费流式累积体**，失败流的 `RawResponseBody` 目前为空
- **usage 归集有三条来源，优先级由 `service/chat/chat_record.go` 的 `resolveUsageSource` 决定，来源记在 `ChatLog.UsageSource` 列**（取值常量见 `models/model.go` 的 `UsageSource*`）：
  1. `upstream`——跨协议场景下由 `models.TransformSideChannel` 旁路交出的**上游原始 usage**。转换层在读上游那一跳就地捕获（流式在 `service/transform/streaming/upstream_usage.go`，非流式在 `service/transform/transform_provider_response.go` 解析 `ParseResponse` 结果后），不经目标协议裁剪，最可信。多跳转换（provider → openai-res → client）只有**第一跳**持有侧信道，第二跳传 nil，避免把中间格式当成上游真值。
  2. `passthrough`——`style == provider.Type`，无转换层，processer 读的就是上游响应本身。
  3. `downstream`——走了转换但旁路没拿到 usage，退化为从**转换后**的下游流反解。此路径会打 `slog.Warn`，因为它可能因目标协议缺字段而失真。

  三条都拿不到有效 token 时记 `missing` 并告警——token 恒为 0 会静默影响计费与配额，不能沉在 DB 里。usage 修正发生在 `chatstats.Record*` **之前**，故统计与日志读到的是同一份数字。
- **上游 usage 字段容错**：全仓单一归一入口 `models.UsageFromMap`（`models/usage_normalize.go`）用有序候选路径表（`promptTokenPaths` / `cachedTokenPaths` / `reasoningTokenPaths` 等）归一各家写法——含 `completion_tokens_details.reasoning_tokens`、`output_tokens_details.reasoning_tokens`、usage 顶层 `reasoning_tokens`、`prompt_cache_hit_tokens`、Anthropic 的 `cache_read_input_tokens`。新增一种非标准写法只加一行候选，不改控制流（OCP）。流式侧信道（`streaming/upstream_usage.go`）、非流 anthropic 入站（`service/anthropic/response_inbound.go`）与 `service/chat/process.go` 的三个 processer（合并为单一 `parseUsageJSON`）共用这一份；此前四处各写一套，退化成同一概念的四个残缺子集（openai-res processer 不读 reasoning、anthropic 非流入站不认 openai 兼容字段）。**全零快照的丢弃收口在 `models.TransformSideChannel.SetUpstreamUsage`**（判据是 `models.Usage.HasTokens`，只看 Total/Prompt/Completion），流式与非流式共用同一道门禁——避免把「没解析出来」记成「上游明确报 0」而压掉 `missing` 告警。`resolveUsageSource` 判定有效性用的是同一个 `HasTokens`，两侧口径不会漂移。**reasoning 拆分的存在性**另走 `CompletionTokensDetails.ReasoningTokensKnown`（`usageFieldPresent` 判「键存在」而非「值 > 0」；三处 DTO→unified 手工作法点以 details 指针非 nil 置位）：落库后前端可区分「上游没报拆分」（展示未知）与「上游明确报 0」（展示 0），避免把 ocg 一类网关不回传 `completion_tokens_details.reasoning_tokens` 误读成「模型没推理」
- **`total_tokens` 的口径分两种，收口在 `models.ResolveTotalTokens`（纯函数）**：
  - **回退口径**——上游省略 `total_tokens` 时取 `prompt + completion`，不把 Anthropic 的 `cache_read` / `cache_creation` 额外计入，与 `models.Usage.HasTokens` 一致。
  - **上游优先**——上游显式给了 `total_tokens`（>0）时一律原样采用、不重算，即使该值含 cache token（贴近上游真值）。故 anthropic 形状 usage 的 total 不再被强制重算为 `prompt+completion`。

  侧信道 / processer 落库（经 `models.UsageFromMap`）与流式客户端写出点（三处统一走 `streaming/usage_wire.go` 的 `usageWireFromModel`：`responses_to_openai.go` 的 usage 尾包、`openai_to_responses.go` 与 `anthropic_to_responses.go` 的 `response.completed`）共用这一判据——此前写出点直写上游原值、缺失时写出 0（anthropic 出站更是硬算 `input+output`），与落库侧分叉，造成同一次请求「客户端看到的 total 与 DB 记录不一致」。**已知缺口**：非流入站的两处 DTO→unified 映射（`service/responses/response_parse.go`、`service/transform/openai/response.go`）仍直取上游原值未过该函数
- 各 style processer 认的字段不同：openai 取根级 `usage`（不解析 `choices`，故天然免疫 OpenAI 的 `choices:[]` usage 尾包）、anthropic 取 `message_delta.usage`、openai-res 取 `response.completed` 内的 usage。这些只在 `downstream` / `passthrough` 路径生效


---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
