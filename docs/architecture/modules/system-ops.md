# system-ops 模块

> 横切接入与运维的**文档合集**（非单一业务实体）。全局架构见 [README.md](../README.md)；交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 横切：bootstrap + ops-io + test + 共享工具 |
| 目录位置 | 见下文子边界；勿将无关业务规则继续堆入本「合集」 |
| 主要职责 | 启动与路由挂接、鉴权、导入导出/DB 运维、连通测试、HTTP 响应与绑定辅助 |

## 2. 职责与边界

### 子边界（文档内拆分，代码目录未强制拆顶层包）

| 子边界 | 目录/入口 | 负责什么 |
|--------|-----------|----------|
| **bootstrap** | `main.go`、`middleware/`、`handler/register*.go`、`repos.go` | 进程启动与优雅关闭、鉴权、`RegisterAll`、默认 Repo、embed WebUI、admin bootstrap |
| **auth** | `service/auth/`、`handler/auth/`、`middleware/auth.go` | 密码哈希、JWT 签发/解析、登录/me/改密码、JWT 中间件 + /v1 调用 key 鉴权（api_keys 多 key，key 管理域见 api-keys 模块） |
| **ops-io** | `handler/importexport/`、`handler/database/` | 配置导入导出、DB stats/vacuum/export |
| **test** | `handler/testapi/`、`handler/testsupport/` | 供应商连通/工具/structured/thinking/vision 测试（四类关联级测试各有非流式与流式两种模式）与测试辅助 |
| **shared-http** | `httpresp/`、`handler/httpx/` | 管理端 JSON 信封；500 收口（原始 err 不外泄、编号内嵌、slog 留痕，见 §4 契约表）；分页/绑定辅助（**渐进采用**） |
| **shared-lib** | `common/`（含 `common/bgtask/`）、`consts/` | 无业务编排的工具与常量；`bgtask` 为后台 goroutine 登记/排空器 |

- **不负责什么**：chat 选路、虚拟模型策略、协议转换、各业务 CRUD 规则
- **对外暴露**：`RegisterAll`、`Deps`、`middleware.AuthJWT`、`middleware.AuthAPIKey`、`service/auth.{HashPassword,VerifyPassword,Sign,Parse}`、`httpresp.*`、`httpx.*`、`bgtask.{Go,SetDefault,Default}`、import/export/database/test REST
- **依赖谁**：聚合各 handler 子包与 service 启动接口；`middleware` 依赖 `repository.ApiKeyRepo` + `UserRepo`、`handler/auth` 依赖 `repository.UserRepo`（DIP）；`service/auth` 只依赖 bcrypt/jwt 标准库，无循环 import

## 3. 内部结构

```
# bootstrap
main.go / middleware/ / handler/register*.go / repos.go / models/（gorm_logger.go：SQL 日志 slog 桥接与脱敏闸，见 §4 契约表）
# auth
service/auth/ / handler/auth/ / middleware/auth.go
# ops-io
handler/importexport/ / handler/database/
# test
handler/testapi/ / handler/testsupport/ / service/credprobe/（completion_probe.go 非流式探针 + stream_probe.go 流式探针）
# shared
httpresp/ / handler/httpx/ / common/ (dataurl, headerpassthrough, sseframe, maputil/, bgtask/) / consts/
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `handler.RegisterAll` | 注册全部业务路由（新域需改此函数挂接） | `handler/register.go` | 调用各子包 `Register*` |
| `handler.Deps` | 注入 JWTSecret 等启动依赖 | `handler/register.go` | `main` 构造 |
| `middleware.AuthJWT` | 管理后台 JWT 鉴权（注入 currentUser 到 context） | `middleware/auth.go` | 同文件 |
| `middleware.AuthAPIKey` | /v1 代理 API 的调用 key 鉴权：按 `credentialcrypto.HashKey(明文)` SQL 点查 `api_keys.api_key_hash`（无内存索引），`enabled=false`→401，按 `OwnerID` 注入 owner 用户（`ContextKeyUser`）与 `CallerKey{ID,Name}` 快照（`ContextKeyCallerKey`，#4-3 归因/#6 头注入共用）；兼容 `Authorization: Bearer` 与 `x-api-key` | `middleware/auth.go` + `callerkey.go` | `handler/register_v1.go` 挂接 |
| `service/auth.Sign/Parse` | JWT 签发与解析 | `service/auth/jwt.go` | 同包 |
| `service/auth.HashPassword/VerifyPassword` | bcrypt 密码哈希与校验 | `service/auth/password.go` | 同包 |
| `service/auth.BootstrapAdmin` | 首启动创建 admin 账号 | `service/auth/bootstrap.go` | `main` 调用 |
| `repository.SetDefault` / `Default` | 默认仓储绑定 | `repository/repository.go` | `main` + handler `repos()` |
| `bgtask.Go` / `SetDefault` / `Default` / `(*Manager).Shutdown` | 登记 fire-and-forget 后台 goroutine，关闭时统一排空（有超时上限） | `common/bgtask/bgtask.go` | `main` 注入 Manager；`service/chat`、`service/healthcheck`、`service/modelsync`、`handler/*` 调 `bgtask.Go` |
| `models.Close` | 关闭底层 `*sql.DB`，**须在** bgtask 排空之后调用 | `models/init.go` | `main.shutdown` |
| `models.gormConfig` / `gormSlogLogger` | 生产 `gorm.Open` 配置**单源**：gorm SQL 日志经适配器进 slog（日志门面统一，Stage 1）；`ParameterizedQueries`（失败/慢语句只见占位符 SQL，不见 api_keys/credentials 的密文 hex 与 hash 值）与 `IgnoreRecordNotFoundError`（/v1 鉴权点查 miss 不留痕，防外部刷日志）两闸恒开，Warn 级 + 200ms 慢阈值与 gorm 默认同参。**闸只护常规回调路径**：`db.Scan()`/`Raw(...).Scan()` finisher 在 gorm 内部经 `logger.Recorder`（`RecorderParamsFilter` 默认透传）旁路 ParamsFilter，拿到的是已内插值的 SQL——含敏感绑定值的查询**禁用** Scan finisher，一律走 `Find`/`First` | `models/gorm_logger.go` | `models.Init`（`models/init.go`）；行为契约测试 `models/gorm_logger_test.go` |
| `httpresp.Success/Error*` | 统一管理端响应 | `httpresp/` | 同包 |
| `httpresp.InternalServerError(c, err)` / `InternalErrorMessage(c, message, err)` | HTTP 500 收口（Stage 3）：两者都先 `slog`（`request_id`/`method`/`path`/原始 err；静态出口的 message 也进日志；客户端断连类降 Warn）再回响应。前者对外恒发 `internal server error [req=<编号>]`（原始 err 不外泄），后者仅回**已审查静态文案**（`message` 禁止拼错误文本，无 err 传 `nil`）。范围仅本机 500（含 panic——`middleware.Recovery` 亦走此出口）；4xx 原文、`/v1` 上游状态码透传不在此列。机械执法 `gate_500_test.go`，约定见 `framework-conventions.md §5` | `httpresp/response.go` | 全项目 handler `err` 分支；契约测试 `httpresp/internal_error_test.go` |
| `middleware.RequestID` / `middleware.AccessLog` / `middleware.Recovery` | 请求编号（Stage 3）：最外层为每请求定 `X-Request-Id`（入站规矩则沿用，否则生成 12 位 hex），写 context + 响应头 + 回写入站头；`AccessLog` 让访问行带同一 rid（path 走 %q，防 `%0A` 伪造整行）；`Recovery` 替代 gin 版——panic 栈进 slog（带 rid）、未提交响应走 httpresp 收口出口，`ErrAbortHandler` 原样再抛。上游**响应**自带 `X-Request-Id` 由 `v1.writeHeader` 改名 `X-Upstream-Request-Id`，网关命名空间唯一 | `middleware/request_id.go`、`middleware/access_log.go`、`middleware/recovery.go` | `main.go` 装配（gzip 之前）；`httpresp` 经 `consts.ContextKeyRequestID` 取号 |
| `testapi.ResolveAssociationLink` / `ResolveProviderLink` | 连通性测试的**唯一**上游链路口径：主协议端点（`SelectPrimaryEndpoint`）→ 候选分组（关联级按白名单 `GroupCandidatesForModel`；供应商级不过滤，显式 `group_id` 时锁组）→ 组内首个可用凭据（`channel.ProbeGroup`，掩码只进 `slog`）。装配失败即 401 类缺陷（#19），六个测试端点（`GET /api/test/:id`、`POST /api/providers/:id/test`、`/test/react/:id`、`/test/structured-output/:id`、`/test/thinking/:id`、`/test/vision/:id`）**禁止**直用 `Provider.Config` 造客户端 | `handler/testapi/models.go` | `handler/testapi/{provider_handlers,react_handler,structured_output_handler,thinking,vision}.go`（`/test/:id` 与 structured/thinking/vision 的装配/探针/模式分叉/失败回包公共流在 `completion_flow.go`；`POST /providers/:id/test` 因链路入参与回包不同仍直用探针） |
| `testapi.BuildTestHeaders` | 诊断测试出站头构造点，`with_header` 语义＝**白名单透传**（默认拒绝），判据在 `common.PassthroughAllowedHeader` / `common.MergePassthroughHeaders`（与生产转发同源，见 chat-gateway 模块）：放行 `X-*` 自定义头（`X-Api-Key` 除外，它可能是入站网关凭据）、`User-Agent`、`OpenAI-Beta`/`Anthropic-Beta`/`Anthropic-Version`；剥除 `Authorization`/`Cookie`（管理端会话材料）、`Referer`/`Origin`（内部地址）、`X-Forwarded-*`/`X-Real-Ip`/`X-Client-Ip`/`X-True-Ip`/`X-Remote-Addr`/`X-Original-Uri`（反代写入的客户端 IP / 内部路由）、`Accept-Encoding`/`Connection`/`Proxy-*`/`Host`/`Sec-*`（传输层/浏览器内部头——自设 `Accept-Encoding` 会禁用 Go Transport 透明解压，gzip 上游会被探针误判死）。旧实现整包 `Clone()` 入站头会把管理员 JWT 发给第三方（Anthropic `x-api-key` 分支不覆写 `Authorization`）。`customer_headers` 为管理员显式通道，**不受白名单限制**且后写覆盖透传结果。契约测试 `TestBuildTestHeaders_PassThroughWhitelist` 断言键级外泄面 | `handler/testapi/models.go` | 三处调用（`completion_flow.go`/`provider_handlers.go`/`react_handler.go`）→ `providers/{openai_base,anthropic}.go` 的 `req.Header = header`；前端关联表单 `association-form-dialog.tsx` 的说明文案 |
| `testapi` 测试结果形状 | 链路装配成功之后的一切结论（含上游非 2xx）都用 HTTP 200 + 信封 `code:200` + `data:{passed, error?, error_type?, mode, group_id, group_name}`；解析**之前**的问题（ID 非数字、显式 `group_id` 不属于该供应商、记录不存在）才是 400/404 信封。`group_id`/`group_name` **恒在**（未选定链路 = `0` / `""`），前端按 `(model, group)` 把 ✓/✕ 归位到分组徽标（C2 形态），所以失败不能走非 200 信封——`apiRequest` 遇 `code!=200` 只抛 message、丢掉 data。`mode`（`"stream"`/`"non_stream"`）**在四个关联级端点上恒在**且成功失败两侧都写（链路装配**之前**的失败按 `respond.go` 既有形状回，那时还没有可报告的执行）：「流式失败」本身就是要给用户看的事实；供应商级 `POST /providers/:id/test` 无模式轴、不写 `mode`。`stream`（`{frames, first_frame_ms, duration_ms, terminated_by}`）诊断块**仅在流式且已进入分帧阶段时存在**（探针在 Content-Type 门或连接阶段就失败时该键缺席，而不是回一个空对象——空 `terminated_by` 会被误读成「上游没发终止帧」这一真实缺陷）。`/test/react/:id` 是 SSE 流（`fetchEventSource` 消费），只复用 `respondRequestError` 判调用方错误，其余结论走 `error`/`success` 事件。**探针阶段的 `error_type`/失败文案/细节由 `credprobe` 两个探针统一产出**，端点只负责套信封（`BuildDetailedError` / `summary: detail`）；`error_type` 值域单一来源 = `common`（前端 `webui/src/lib/errors.ts` 的 `ERROR_TYPES` 镜像该值域） | `handler/testapi/respond.go` + `completion_flow.go` 的 `probeOutcome.modeFields` | `webui/src/lib/api/modules/catalog/{providers,model-providers}.ts` 的 `*TestResult` 与 `StreamTestStats` |
| `common.ErrorTypeFromStatus` | 诊断测试 `error_type` 的**唯一**状态码映射：`{network,auth,provider,timeout,validation,unknown}` 常量 + 状态码→类型 switch（401/403=auth、404/≥500=provider、其余 4xx=validation、其他=unknown）；`network`/`timeout` 无状态码可判，由 `credprobe` 两个探针在 Do 与读 body（流式含读流）两个错误分支按 `errors.Is(DeadlineExceeded)` 区分（滴流超时归 timeout，不归 network）——否则同一慢上游在非流式弹窗归 timeout、流式弹窗归 network。下沉自 `handler/testapi`（旧 `getErrorTypeFromStatus` 薄委托已删除），因 #24-B 若另写一份必与之漂移 | `common/errtype.go` | `service/credprobe/{completion_probe,stream_probe}.go`、`handler/testapi/react_anthropic.go`；前端 `webui/src/lib/errors.ts` |
| `common.ExtractUpstreamErrorDetail` / `UpstreamErrorDetail` | 上游错误体细节提取**唯一**来源：沿 JSON 信封的 `error`/`message` 字段**递归**追最内层人读文本（含「`message` 本身又是 JSON 信封」的网关套娃，如 opencode go；层数预算 5），追不到回退原文，统一截断 512B；`UpstreamErrorDetail` 附加 `HTTP 状态码: N` 行。旧 `testapi.parseProviderErrorDetail`（不截断）与 `pools.truncateDetail`（不解析）双轨已收敛于此 | `common/upstream_detail.go` | `credprobe.{RunCompletionProbe,RunStreamProbe}`、`handler/testapi/react_anthropic.go` |
| `credprobe.RunCompletionProbe` | 「一次真实最小 completion 探针」编排的**唯一**来源（BuildReq→带 deadline 的 Do→网络/超时归类→读 body→非 2xx 归类→细节提取；通过=任意 2xx；`Timeout≤0` 兜底 `DefaultCompletionTimeout`，须传常量口径避免放大 client 缓存）。ctx deadline 刻意早于 transport 超时（+`CompletionTransportBuffer`），慢上游（含 headers 后 body 滴流）稳定归 `timeout` 而非误归 `network`。管理端**新增非流式 completion 类测试端点必须调本函数，禁止再抄编排**；client 装配与探针体仍归各调用方（链路语义不同） | `service/credprobe/completion_probe.go` | `handler/testapi/{provider_handlers,completion_flow}.go`、`handler/pools/credential_probe.go` |
| `credprobe.RunStreamProbe` | 「一次真实**流式** completion 探针」编排的**唯一**来源，与 `RunCompletionProbe` 并列（两者共用 `CompletionTransportBuffer` 超时分级与状态码归类口径，**同一失败阶段不再各写一份措辞**）：注入 `"stream": true`（测试体因此不必为流式复制一份常量）→ `Accept: text/event-stream` → Do → 非 2xx 走**同一套** `ErrorTypeFromStatus`/`UpstreamErrorDetail` 归类 → **Content-Type 硬门**（判据单源 `common.IsEventStreamContentType`，非 event-stream 即归 `ProbeSummaryStreamUnsupported` + `validation`，这是「上游吃掉 stream 参数」的显式检出）→ SSE 分帧（判据单源 `common.ScanSSEFrames`，`common/sseframe.go`；探针 / 网关转发 / chat 日志三处读流走同一实现，「测试判过的流」与「转发链路实际处理的流」结构上不可能分叉。载荷 trim 为 SSE 规范式：只去前导一个空格，不去行尾空白）。预算（单行 1 MiB / 整条流 4 MiB / 帧数 1 万，超限 `ProbeSummaryStreamTooLarge`，管理端点不得被失控上游打爆内存）。**双轨口径**：`Frames` 无损（判定唯一输入）/ `RawOutput` 有损回显（头 224 KiB + 尾 32 KiB 保留、中段截断打标），调用方**禁止**用 `RawOutput` 反推判定。探针不判断内容对不对（空流也算 Passed=true），「有没有内容 / 有没有终止事件」属端点领域判定 | `service/credprobe/stream_probe.go` | `handler/testapi/completion_flow.go` |
| `testapi.foldStream` / `validateStreamFold` | 流式模式的**协议语义层**（纯函数）：把 SSE 帧折叠回**非流式响应体形状**，使 `extractThinking` / `extractVisionText` / `extractStructuredOutputJSON` 原封复用——**通过标准只有一份**是设计约束，为流式另写一套判定必然与它漂移。三协议策略：OpenAI Chat 累加 `choices[].delta.{content,reasoning_content,reasoning}` 与 `tool_calls[].function.arguments`（**流里出现哪个推理键就写回哪个键**，归一会掩盖网关差异）；Anthropic 按 `content_block.index` 建块保序，`input_json_delta` 拼完以 `json.RawMessage` **原字节**写回 `tool_use.input`（先 unmarshal 再 marshal 会让 map 键按字母序重排，判定文本就对不上上游原文）；Responses **直接取终止帧 `response.completed` 的 `response` 原文**（它本身就是完整非流式响应体，省掉一整条 delta 映射分支）。事件名允许从 `event:` 行回落到载荷 `type`（部分中转不发 `event:` 行）。严格门：流内 error 帧 / `response.failed` → 报上游故障（不当成「模型未返回内容」）；缺终止事件 → `errStreamNotTerminated`；无任何内容增量 → `errStreamNoContentDelta` | `handler/testapi/stream_shape.go` | `handler/testapi/completion_flow.go`（三门在此统一执行，四个端点零重复） |
| 测试的**模式轴**（`?stream=1`） | 流式与非流式是同一能力测试的**两种执行模式**，不是两种测试：四个关联级端点（`/test/:id`、`/test/structured-output/:id`、`/test/thinking/:id`、`/test/vision/:id`）同一 GET 靠 query 参数分模式，**不新增路由、不新增 `providers.Metadata` 字段**。新增同类测试仍走 `runCompletionTestFlow` 一份编排；**禁止**为流式复制第二套判定或端点。`/test/react/:id` 与 `POST /providers/:id/test` 不提供模式（前者本就恒流式，后者批量逐条开流会把耗时乘以 N） | `handler/testapi/{completion_flow,stream_shape}.go` | `webui/src/routes/model-providers/components/dialogs/test-dialog.tsx`（模式开关）+ `lib/api` 的 `testModelProviderXxx(id, mode)` |

## 5. 特殊约定

- **禁止**在 `main.go` 增加业务路由明细；新 API 域改 `RegisterAll` + 子包 `Register*`
- 鉴权：`/api/*` 用 `middleware.AuthJWT`（登录路由 `auth.RegisterLogin` 在 AuthJWT 之前挂，免鉴权）；`/v1/*` 用 `middleware.AuthAPIKey`（api_keys 表多 key hash 点查，兼容 `Authorization: Bearer` 与 `x-api-key`；key 生命周期管理见 api-keys 模块）。`service/auth` 是纯工具层，不依赖 repository/models
- `database`/`importexport` 允许 PRAGMA/VACUUM/`c.File`/事务 SQL 等白名单直连
- `httpx`：**存在** Strict/Loose 分页与 `BindJSON`，但 handler 仍大量手写 `ShouldBindJSON`——属渐进落地，非已全量统一
- `common` 不再依赖 Gin（响应已迁 `httpresp`），避免沦为业务垃圾桶
- **优雅关闭序**（`main.shutdown`）：`srv.Shutdown`（停收新请求 + 等在途，**必须带 timeout**——SSE 回写无 deadline）→ `credwrite.Stop`（Flush 凭据写队列 + 停收，10s 超时，**必须**在此处、`bgtask.Shutdown` 之前）→ `bgtask.Default().Shutdown`（排空后台写库 goroutine）→ `models.Close()`；顺序不可颠倒，排空前关库会让在途写库任务撞上「DB 已关闭」。注意 ticker 服务的停止由信号 ctx 取消驱动，与「等在途请求」**并行**发生，不在上述四步之内；排空在两者之后
- 关闭的**唯一入口**是取消 `main` 的可取消 ctx：信号与 `ListenAndServe` 启动失败共用此路径，**禁止**在任何分支用 `os.Exit` 旁路 `shutdown`（会硬切已登记的后台写库任务）
- 容器/编排部署**必须**把宽限期设到大于关闭预算（`serverShutdownTimeout` + 2×`bgtaskDrainTimeout`，当前 30s+10s+10s）：compose `stop_grace_period`、k8s `terminationGracePeriodSeconds`；否则进程在排空完成前被 SIGKILL，优雅关闭形同虚设
- **禁止**用裸 `go f(context.Background())` 起会写库的后台 goroutine；**必须**经 `bgtask.Go` 登记。请求内且已用 `sync.WaitGroup` 等待完成的 goroutine 例外（由 `srv.Shutdown` 覆盖）
- 连通测试**一次解析一次请求**：`buildLink` 跨候选组只补「本组无可用凭据」，上游返回失败**不**跨组重试——测试体是真实计费的 completion（`providers/meta_bodies.go`）且前端批量并发 3，放大成 N 倍花费不可接受；react 是 SSE 多轮，重跑会重复推事件。代价已知：组内只验首个可用凭据，排在后面的坏 key 由 credprobe / 号池详情按凭据测试覆盖
- **流式模式只在管理端单次点击下使用**：整条流要读完才能折叠判定，thinking 类可达数十秒，故超时沿用 60s 常量；**禁止**把它接进 `healthcheck` 或按凭据批量测试（探针耗时放大一个量级、探测语义也要重新定义）。上游失控狂吐由 `streamProbeMaxTotalBytes` 预算兜底，不靠调小超时（调小会把正常慢渠道误判成 timeout）
- 前端静态：`//go:embed webui/dist`，`NoRoute` 对非 `/api` `/v1` 的 GET 回 `index.html`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写子边界；2026-08-16 补充优雅关闭序与 `bgtask` 契约；2026-09-09 补充 testapi 诊断链路与测试结果形状（#19）；2026-09-12 completion 探针编排收敛至 `credprobe.RunCompletionProbe` 后回写契约；2026-09-13 新增 thinking 能力测试端点（`/test/thinking/:id`）；2026-09-14 新增图片识别能力测试端点（`/test/vision/:id`）；2026-09-14 关联级四端点新增**流式/非流式模式轴**（`credprobe.RunStreamProbe` + `testapi.foldStream` 折叠复用既有 extractor；`/test/:id` 并入 `completion_flow`，失败回包统一带 `raw_output`）；2026-09-14 `with_header` 由整包透传改**白名单透传**（`BuildTestHeaders`，止住管理端会话头外发第三方）；2026-09-15 白名单判据下沉 `common/headerpassthrough.go`，生产转发 `chatcore.BuildHeaders` 改用同一判据（热路径收口，两链路单一同源）；2026-09-16 SSE 分帧与 Content-Type 判据下沉 `common/sseframe.go`（探针/网关/chat 日志三处读流单实现，#3）。*
