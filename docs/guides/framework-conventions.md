# 框架专属约定

> 按需加载文档。操作规范总入口见 [AGENTS.md](../../AGENTS.md)。
> 本文收录 Go/Gin、React/Vite、协议代理三块框架特有的踩坑约定——多数条目源自真实事故，改这些区域前先读对应小节。

## 1. Go / Gin

- 路由：`RegisterAll` 建 `/v1` 与 `/api`；`/v1` 挂 `middleware.AuthAPIKey(apiKeyRepo, userRepo)`，`/api` 登录路由免鉴权，其余挂 `middleware.AuthJWT(secret, repo)`。
- 引擎装配（`main.go` 的 `newRouter(deps, trustedProxies)`，**不用** `gin.Default()`）：`SetTrustedProxies(trustedProxies)`（**信任边界**，Stage 20，2026-09-24：值来自 `TRUSTED_PROXIES` env 经 `parseTrustedProxies` 解析——只有网段内代理的 `X-Forwarded-For` 会被采信为 ClientIP，未设/空 = **全关**（直连默认安全，伪造 XFF 不进日志 IP）；非法值装配 fail fast 不静默回退。⚠️ gin 的**默认**（从不调用 `SetTrustedProxies`）是 `0.0.0.0/0`+`::/0` 信任全部代理，必须显式调用才收窄；`SetTrustedProxies(nil)` 与 `[]string{}` 在 gin v1.10 同为全关（`prepareTrustedCIDRs` 对 nil 返回空集、`isTrustedProxy` 对空集判 false），解析层恒返回非 nil 显式表达「未配置 = 全关」。装配先于 `startBackgroundServices`，配置错误时无后台任务可安全退出）→ `RequestID()`（请求编号，Stage 3，见 §5）→ `AccessLog()`（等价原 gin Logger + rid，path 走 %q 防换行注入；整行 ≤4 KiB，超限截 `%q` **之后**的 path 段、status/耗时/rid 恒存活——Stage 16，见 §4）→ `Recovery()`（Stage 3 自建，替代 `gin.Recovery`：panic 型 500 同走 httpresp 收口，见 §5）→ gzip。**禁止**把 `RequestID` 挪到鉴权/路由之后——401/404 也要有号可对。装配序由两条测试锁定（Stage 18）：`main_router_assembly_test.go` 的 `TestGateRouterMiddlewareOrder`（四个 `router.Use` 的**次序** + 「全部 Use 先于 `handler.RegisterAll`」——gin 为已注册路由快照 handler 链，Use 写在 RegisterAll 之后对那些路由根本不生效）与 `TestRouterAssembly_RequestIDStamped`（**后果**：401/404/500 响应均带 `X-Request-Id`，访问行与 panic 留痕同号）。中间件增删或重排属装配契约变更，须同步这两处判据；行为断言测不出「先登记谁」（gin 访问行 formatter 在 `c.Next()` 之后才读请求头，后置的 RequestID 也已回写完毕），故次序必须由源码门锁。信任边界另由 `main.go` 的 `trusted_proxies_test.go` 锁定（伪造 XFF 无效 + nil=全关 + 配置网段生效 + 非法值 fail fast 四用例）。
- 子包注册函数只接收 `gin.IRoutes`（或项目既有签名），**不**在子包内 `Group` 出与全局不一致的鉴权边界。
- 后台任务在 `main` 启动：`HealthChecker.Start`、`ModelSyncService.StartAutoSync`，两者的 ctx 均派生自 `signal.NotifyContext`（取消即退出信号）。
- **禁止**用裸 `go f(context.Background())` 启动会写库的后台 goroutine；**必须**经 `common/bgtask` 的 `bgtask.Go` 登记，否则进程关闭时落库/权重衰减会被硬切。请求内且已用 `sync.WaitGroup` 等待完成的 goroutine 不在此列（由 `srv.Shutdown` 等在途请求覆盖）。
- 优雅关闭序在 `main.shutdown`：`srv.Shutdown`（停收新请求 + 等在途）→ `credwrite.Stop`（Flush 凭据写队列 + 停收，10s 超时）→ `bgtask.Default().Shutdown`（排空后台写库）→ `models.Close()`。每步各有超时上限且失败不提前返回；**禁止**去掉 `srv.Shutdown` 的 timeout——SSE 回写无 deadline，不设上限会让关闭无限挂起。**禁止**把 `credwrite.Stop` 挪到 `bgtask.Shutdown` 之后——队列 worker 经 bgtask 登记但退出由 `Stop` 的 requestStop 驱动（与 healthcheck 同构，不响应 ctx 取消），顺序反了 bgtask 会空等到超时、关库前队列也未必排空。ticker 服务的停止由信号 ctx 取消驱动，与「等在途请求」**并行**发生，不在这四步之内。
- 关闭的**唯一入口**是取消 `main` 的可取消 ctx（由信号 ctx 派生）：信号与 `ListenAndServe` 启动失败共用此路径。**禁止**用 `os.Exit` 旁路 `shutdown`——会硬切已登记的后台写库任务。注意 `signal.NotifyContext` 返回的 `stop()` 只停信号投递、**不**取消 ctx，故必须自己包一层 `context.WithCancel`。
- 容器/编排部署**必须**把宽限期设到大于关闭预算（`serverShutdownTimeout` + 2×`bgtaskDrainTimeout`，当前 30s+10s+10s）：compose 用 `stop_grace_period`、k8s 用 `terminationGracePeriodSeconds`；否则进程在排空完成前被 SIGKILL，本机制形同虚设。
- 长驻循环的取消源**只能有一个**：其自身由信号 ctx 派生的 ctx。`bgtask` 只负责「等它停干净」，不提供第二个取消源。
- 前端产物：`//go:embed webui/dist`；改前端后未 rebuild 则二进制仍是旧 UI。
- 并发：虚拟模型轮询等共享计数**必须**用 `sync.Mutex`（或等价）保护，与现实现一致。

## 2. React / Vite 管理端

- 服务端状态：TanStack Query；客户端状态：Zustand（`stores/core/updater` 统一更新模式）。
- 表单：react-hook-form + zod；UI：Radix + Tailwind + CVA。
- 路径别名：`@/` → `src/`；开发代理：`/api` → `http://localhost:7070`。
- 页面**不得**绕过 `lib/api` 核心客户端自造鉴权 header 逻辑（除非扩展 client 本身）；**不得**自行解析响应体/信封（失败文案的唯一来源是 `client.ts`，见 `modules/webui.md` §5）。
- API 信封约定：`{ code, message, data }`；「成不成功」需同时看真实 HTTP 状态码与信封 `code`；失败文案取信封 `message`（500 补响应头 `X-Request-Id` 报障编号）；401 = 登录过期 → 清会话 + 跳 `/login`（接口自身凭据无效者走 `apiRequestAnonymous` 并让后端改用非 401 分类）。
- TypeScript 严格模式；**禁止**无注释的 `any`；确需放宽用 `unknown` 并在使用处收窄。
- dialog 尺寸**只能**用 `DialogContent` / `AlertDialogContent` 的 `size` 预设（`components/ui/dialog.tsx` 的 CVA 是唯一数据源），页面侧**禁止**再写 `max-w-*` / `max-h-*` / `h-[...]` 等外框尺寸类；`padding` / `gap` / `rounded` 等非尺寸调节不在此列。组件**内部**变长区（列表、日志、表格列宽）的 `max-h-*` / `w-[...]` 是合法内部约束，保留。理由：29 处各写各的尺寸拼装曾导致全站尺寸漂移，收口到单档后调参成本从 N 处降到 1 处。可机械复核：`rg 'DialogContent (size|className)' webui/src` 里 className 不应含外框尺寸类。
- dialog 的 Header 与 Footer 之间的内容**必须**包一层 `DialogBody`（全站唯一的滚动容器，自带 `min-h-0 flex-1 overflow-y-auto`）；**禁止**让 `DialogContent` 整框滚动或页面自己手写 `flex-1 overflow-y-auto`。固定高度档（`lg`/`xl`）不包 body 会让内容溢出外框，上限档（`sm`/`md`/`menu`/`sheet`）不包 body 则 `max-h` 只是一条没人执行的声明。「固定搜索栏 + 内部滚动列表」这类分区形态，`DialogBody` 套在**真正滚动的那一层**即可，不必是 `DialogContent` 的直接子级。`AlertDialogContent` 例外：它无 body 抽象、内容稳定，走整框滚动（理由见该文件注释）。

## 3. 协议与代理

- 对外：OpenAI/Anthropic 兼容 `/v1/*`；对内管理：`/api/*`。
- 认证：管理后台 `/api/*` 用 JWT（`Authorization: Bearer <JWT>`，登录走 `POST /api/auth/login`）；代理 `/v1/*` 用调用方 API key（`api_keys` 表多 key hash 点查，`Authorization: Bearer <key>` 或 `x-api-key: <key>`，Anthropic 兼容）。
- 流式：SSE；转换与 process 路径分流行/非流，**禁止**混用缓冲假设。
- 两层 LB：虚拟→真实（virtualmodel）+ 真实→供应商（chatcore/balancer）；改一层时**禁止**破坏另一层的候选过滤（如黑名单在候选池）。

## 4. 请求/响应体大小资源防御（Stage 2 质量门，2026-09-17）

读取上游响应或请求体的代码**禁止**无上限 `io.ReadAll(r.Body)`。合法入口白名单：

- 诊断 / 探针 / 连通测试路径：`common.ReadCappedBody`（1 MiB 单源；上游未通过判定（见 `common.IsPassingStatus`）的错误体同口径）。
- `Models()` 模型清单读取（Stage 27，2026-09-27）：`common.ReadModelsListBody`（`common.ModelsListReadLimitBytes` 10 MiB 单源），providers 两处 `Models()` 成功路径解码前必经；定档量级依据（每模型 ~100-200B、聚合商数百模型 ≈ 100 KiB）写在 common 注释；达上限视为截断按解码失败处置（错误文本含 `read cap` 与状态码）。刻意区别于诊断探针的 1 MiB——清单体量天然大一个数量级，沿用 1 MiB 是拍脑袋不是定档。
- 入站请求体：`consts.MaxInboundBodyBytes`（20 MiB 硬上限，代码常量刻意不做成设置——可被调小的防线不是防线）+ `io.LimitReader(body, max+1)` 多探 1 字节拒绝；`/v1` 超限回真实 413（SDK 不自动重试），管理端按该端点既有信封惯例（HTTP 200 + 信封 code，与同域导出端点一致；前端两条通道都会解析）。
- 上游非流式成功响应：`service/chat` 的 `limitUpstreamResponseBody` 在 attempt 内、响应头提交给客户端**之前**整读加限（设置项 `upstream_max_response_mib`，默认 32 MiB、下限 1 不可关闭），未超限回填为可重读内存字节。失败语义：超限=端点级异常 → 组织级转移**不冷却凭据**；读体错误（含 ctx 超时打断读体）与 `Client.Do` 失败**成对**归凭据级网络——超时分类只挂 Do 侧会让读体超时静默漏走（见全局 RULES「Go HTTP 超时」）。
- 原始采集落库：四文本字段（`RequestBody`/`RawRequestBody`/`RawResponseBody`/`Error`）共用单字段上限（设置项 `raw_log_field_max_kib`，默认 256 KiB），截断一律 `common.TruncateUTF8`（rune 边界 + `…(truncated)` 标记，按字节劈半进 JSON 必出 U+FFFD 乱码尾）。采集读头不吞尾（MultiReader 回填），旁路不得成为下游截断点。
- 回复类日志落库（Stage 23，2026-09-25）：`ResponseBody`（非流式 + 流式拼接）与 `ChatIO` 累积共用单条上限（设置项 `response_log_max_kib`，默认 2048 KiB、硬顶 16384），截断一律 `common.TruncateHeadTailUTF8`（头尾保留 + `common.TruncateHeadTail` 统一标记，标记字节计入预算）。流式累积侧另有 `service/chat/output_budget_accumulator` 预算闸（内存与落库双头同受约束）。
- 全项目截断标记**单源** `common/truncate.go`：尾切 `TruncatedSuffix`、头尾式 `TruncateHeadTail`；`common.SSETruncatedMarker` 文案委托后者。新增截断场景禁止另写标记文案。
- 访问日志单行（`middleware.formatAccessLine`）：上限 4 KiB（代码常量 `accessLogLineMaxBytes`，刻意不做设置——同上口径），超限截 `%q` **之后**的 path 段（先截后转义会被控制字符 `\xNN` 展开重新超限），status/耗时/rid 恒存活；截断一律 `common.TruncateUTF8`（Stage 16，2026-09-20）。机械复核：`middleware/access_log_test.go` 断整行字节数 ≤ 上限 + rune 边界回退 + 截断标记存在 + rid 存活。其余字段自身病态时 path 段预算压不下整行，退化为整行尾部截断：体量上界优先，rid/status 在载体字段已病态时被切（审查回批 W1，`TestFormatAccessLine_OverlongClientIPCapsWholeLine` 锁定）。原「伪造超长 XFF 的 ClientIP」病态来源已由 Stage 20（2026-09-24）信任边界收窄：`TRUSTED_PROXIES` 未设时 XFF 不进 ClientIP，本兜底保留作纵深防御（病态远端地址等残余面）。
- slog 侧 path attr（Stage 26，2026-09-27）：`middleware/recovery.go` panic 留痕行与 `httpresp/response.go` `logInternalError` 两处 path 写点过 `common.TruncateUTF8`，上限单源 `common.SlogPathMaxBytes`（4 KiB，`common/log_limits.go`，与访问行同族同值、同样不做设置）。病态长 URL 触发 panic/500 时不撑爆留痕行；回归 `httpresp/path_truncate_test.go` + `middleware/recovery_path_truncate_test.go`。
- 日志头快照脱敏（Stage 16 立项，Stage 26 扩两列）：请求列 `captureRequestLogSnapshot` 与响应列 `responseHeadersJSON` 落库前过 `common.MaskHeadersWithDenyList`（deny-list 五键 + 动态 extraKeys=现存自配头名全集）；新增敏感键在 `common/headerpassthrough.go` 加一行即双侧生效，禁止在 chat/log 侧复制名单。
- 流式 SSE 不整读、天然不在本防线内；帧级/累积侧信道截断策略另有约定（见 logs-metrics.md），提高上限需单独立项。流式**日志累积**的体积口径已由 Stage 23 收口（`output_log` 预算闸，见上一条），与「不整读转发流」不冲突——预算闸只约束日志副本，转发流仍逐 chunk。

豁免**必须**在读取处注释说明「体积已被上游哪一道限定」——现存唯一豁免：`service/transform/transform_provider_response.go` 整读（已被 attempt 体积闸限定）。可机械复核：`rg 'io\.ReadAll' --type go -g '!*_test.go'` 的命中只允许是上述白名单入口或带豁免注释处。

## 5. HTTP 500 错误响应收口（Stage 3 质量门，2026-09-17）

HTTP 500（本机自己坏，**含 panic**）**禁止**把内部 `err` 原文回给调用方（SQLite/gorm 报错含表名/列名/约束名）；原始错误只进服务端日志。**禁止**绕过 httpresp 手搓 500（`c.JSON(500,…)`、`http.Error`、直用 `StatusInternalServerError` 常量）。合法出口只有两个，两者都先 `slog`（带 `request_id`/`method`/`path`/原始 `error` 与调用方业务上下文 `attrs`）再回响应；静态出口的 `message` 也一并进日志（同名分支靠它定位是哪一处）：

- `httpresp.InternalServerError(c, err error, attrs ...any)`：对外恒发通用文案 `internal server error [req=<编号>]`（编号内嵌，SDK 用户报障时拿得走），原始 err 只进日志。拼接式旧写法（`"前缀: "+err.Error()`）拆到这里或下面的静态入口（前缀留下、err 进日志）。
- `httpresp.InternalErrorMessage(c, message string, err error, attrs ...any)`：仅用于**已审查、不含内部信息**的静态提示（如 `"encryption not configured"`）。`message` **必须**是字符串字面量或纯业务标识符（`action`、`ie.Message`——其取值须可追回常量），**禁止**拼接任何错误文本（`err.Error()`/`fmt.*`/`+`）；`err` 为仅服务端可见的原始错误，配置守卫类无 err 时传 `nil`。
- `attrs`（Stage 18）：业务上下文（`credential_id` / `group_id` / `api_key_id` / `path` / `stack` 等）**必须**经这个变参并进**同一行**留痕；键值成对，**禁止**占用保留键 `request_id` / `method` / `path` / `message` / `error`。
- **级别约定（Stage 25 扩写）**：ERROR = 服务器故障，请求路径上一切客户端错误（4xx）留痕一律 ≤ `Warn`。两个子口径：① 客户端断连类（`context.Canceled`/坏管道/连接重置）——500 收口出口内部已按此降 `Warn`（见下 `isClientDisconnect`）；② 4xx 客户端错误的手写留痕（读体失败 / 体积超限 / JSON 解析失败等）**必须**用 `slog.Warn` 及以下——`httpresp.BadRequest` 等不留痕，那行手写留痕是该次失败的唯一记录（内容保真、只降级别），一个持续发坏请求的客户端不该能把按 ERROR 计数的告警刷满。执法：`gate_4xx_error_level_test.go`（同一块内 `slog.Error` 与 4xx 响应出口并存即红；后台任务 / 启动装配的 `slog.Error` 不调 httpresp，天然不在判据面）。**边界**：500 级收口出口之外的 `slog.Error`（含嵌套 if 分支内、后台任务）不在本条约束内，其合法性由「ERROR 唯一入口」与下述边界①②判定。
- **ERROR 唯一入口（Stage 18）**：一次 500 级失败只允许留**一条** ERROR，且只能出自上面两个出口（每调用写一行）。**禁止**在调用点再写 `slog.Error/Warn` 给同一个错误重复留痕——历史上这种「手写一行业务细节 + 收口再留一行错误」的双份留痕，让按 ERROR 行计数的告警把一次故障算成两次，且两行还得靠 rid 才对得上；细节请走 `attrs`。`middleware.Recovery` 的 panic 同口径：未提交响应走收口出口、栈进 `attrs`；已提交（SSE 半开流）响应已回写无从补收口，才由 Recovery 自留该次 panic 的唯一一条 ERROR。**边界**（刻意不算违规）：① 4xx 客户端错误的手写留痕——`httpresp.BadRequest` 等不留痕，那行是该次失败的唯一记录（级别按上行「级别约定」≤ `Warn`）；② 后台任务 / 启动装配的 `slog.Error`——无请求上下文，本就没有收口出口可走；③ 响应已提交后的流中断收口（handler/v1 `writeResponseBody`，2026-09-22；留痕归口 2026-09-23 审查回批）——响应已回写无收口出口（同 Recovery 已提交分支口径）：流式写协议内 error 事件收尾、非流式静默截断，两者**都不在 handler 留痕**；该失败的唯一一条留痕由日志管道收口 `service/chat.RecordLog` 写出（错误经管道落 chat_logs Status=error；断连类降 Warn、其余 ERROR，级别判定同上「级别约定」）。计数口径：`RecordLog` 的 processer 失败与持久化失败都只 return 不自记，唯一写出点是收口那一条（内层+外层双记是 2026-09-23 前的实况）；回归测试 `service/chat/chat_record_log_once_test.go` 与 `handler/v1/stream_error_test.go` 锁「恰一条 / handler 零自记」计数。

**范围边界**：只收本机 500。① 4xx 原文（校验/绑定错误，管理端表单报错提示依赖它）不改；② `/v1` 上游状态码透传（`handler/v1/chat.go` 的 `errors.As(statusCoder)` 分支，转发的是供应商错误，属协议内容）不在本防线；③ `/v1` 代理链路自带状态码的业务失败（配置态 404 / 重试穷尽 502，`service/chat/client_request_error.go` 构造）同走该分支按自带状态码返回——分类口径见 `chat-gateway.md` §5「失败状态码分类口径」。

**请求编号**：`middleware.RequestID()` 挂在所有路由最外层（`main.go` 于 gzip 之前），为每请求定一号，编号**自带来源前缀**（Stage 17，防外部自选值与自生成号共桶串号）：网关自生成 = `gw:` + 12 位 hex；沿用入站 `X-Request-Id` = `in:` + 原值（内核校验 `^[A-Za-z0-9_\-]{1,64}$` 不变，对齐调用方自家 trace）；入站值已含 `gw:`/`in:` 前缀判非法重生成（前缀是网关标记，外部不得自选，防双前缀与自生成号空间污染）。**前缀进值本体**而非展示点拼接：httpresp 文案 `[req=]`、slog `request_id` 字段、访问行 `rid=`、日志页回显全部消费同一个带前缀值（值透传，展示点零拼接），契约测试 `middleware/request_id_consistency_test.go` 锁一致性。编号写三处（context / 响应头 / 回写入站头，令 `middleware.AccessLog` 访问行与 `/v1` 上游请求透传都指向同一号；上游供应商看到的 `X-Request-Id` 带前缀——保留外部 trace 关联只加标记，Stage 17 口径）。上游**响应**自带的 `X-Request-Id` 在 `writeHeader` 改名 `X-Upstream-Request-Id` 回传——网关命名空间唯一，报障编号无双义。`middleware.Recovery` 替代 `gin.Recovery`：panic 型 500 未提交的响应走 `InternalServerError` 收口（客户端拿得到 `[req=]`，栈进其 `attrs`——一次 panic 只产一条 ERROR，见上「ERROR 唯一入口」），已提交半开流不追加 body、由 Recovery 自留该次 panic 的唯一一条栈留痕（带 rid）。

执法：三条判据编进 `go test ./...`——`gate_500_test.go` 的「禁止绕过」「message 禁拼接」，加 `gate_500_dual_log_test.go` 的「ERROR 唯一入口」（AST 级：同一块内 `slog.Error/Warn` 与收口出口并存即红，附判据自测防漏判/误判）；人肉复核命令（与门同判据）：`rg -n 'httpresp\.InternalErrorMessage\([^,]+, [^,)]*(\+|\.Error\(\)|fmt\.)' --type go -g '!*_test.go'` 零命中。

## 6. 上游响应状态码通过口径（Stage 5 质量门，2026-09-17）

判「这次上游算不算成功」的**唯一入口是 `common.IsPassingStatus(statusCode int) bool`：通过集 = 200 与 201，仅此两码**。非测试代码**禁止**对上游状态码做判通过的裸比较（`res.StatusCode != http.StatusOK`、`statusCode == 200` 这类），无论写在哪个包。

- **为什么带上 201**：OpenAI 兼容网关有相当一部分照抄 OpenAI 某些接口的习惯回 201，它们工作正常、内容有效。旧的严格 `== 200` 把这类供应商判死；而健康检查这一处判失败的代价不是「诊断报错」，是**自动停用模型-供应商关联**（`handleCheckResult` 连续失败到阈值即停用，且停用原因不入库）——线上静默少一条可用线路，后台只显示「已停用」，看不出是状态码口径造成的。
- **为什么不放宽到整个 2xx**（审查回批，2026-09-17）：放宽的全部理由只有「有些网关回 201」。204/206 这类「成功但没有正文」从来不是聊天补全该有的回答；把它们判通过，**直通链路**（客户端与上游协议形状相同、不进转换层，因此没有任何解析步骤会失败）会把空 body 直接回给调用方，并走一整套成功副作用——权重/优先级**上调**、失败计数清零、健康检查 `autoEnable` 重新启用关联。即坏供应商被系统主动加权，比误停用更难发现。保持其失败语义 = 沿用统一前的处置（记 error + 组织级衰减 + 换下一家），不新增行为。
- **为什么落 `common` 而不是由 `credprobe` 导出**：同一条判定有 7 处需要（`service/healthcheck`、`service/credprobe` 两个探针、`service/chat` attempt、`providers` 两处 `Models()`、`handler/testapi` react），其中 providers 层**禁止** import service 子包（依赖方向），跨 2 域以上的纯逻辑必须下沉 `common`（AGENTS §4.4）。
- **边界（刻意不管的两件事）**：① `IsPassingStatus` 只判状态码、**不判内容**——上游回 200/201 但内容并非合法响应（被重定向到登录页、一页 HTML 错误）不在此函数拦截；各调用点对内容的要求本就不同（聊天要能解析成回复、模型列表要清单、流式测试要真在吐字），捆进同一个判定会让改聊天校验连带牵动模型列表。**更新（2026-09-22）**：chat 转发链路的内容有效性由**提交门**补判（见下）；**更新（2026-09-23，Stage 19）**：健康检查成功分支已叠**内容形状门**（见 §7），探针侧仍只判状态码（探针答「key 活不活」，刻意不判领域内容）。解码失败路径**必须把上游状态码带进错误文本**（如 `decode model list (HTTP 200): ...`），否则同步日志只剩 `EOF` 无从定位。② 按**具体失败码**分类的写法合法（401/403 归鉴权、429 归限流、5xx 归供应商，如 `common.ErrorTypeFromStatus`）——那是「失败之后怎么处置」的另一个决定，不在「算不算通过」这条线上。
- **内容有效性判据（提交门，2026-09-22）**：状态码通过之外，chat 转发链路在**写给客户端之前**多一道内容门——`common/ssecontent.go` 是「什么内容可提交」的唯一判据（wire 无关错误形状：顶层 `error` 键非 null / `type:"error"` / `event: error` 行；非法 JSON、空体、非对象拒收；`[DONE]` 免检），`service/chat/chat_commit_gate.go` 在 attempt 内执行（流式预读第一非空帧 + 预读字节回放，非流式校验已物化 body），失败回重试循环换下家、客户端零字节。预读预算（总量 4 MiB / 单行 1 MiB）超限与畸形巨行按**不可提交**处置（组织级换下家、不冷却凭据——垃圾洪水不该罚 key；2026-09-23 审查回批修正分类，此前误归凭据级网络出口、会把组内 key 逐条冷却）。**已知边界**：① 首帧判据只看第一非空帧，第二帧起的错误不触发重试（提交后语义，由 handler 断流收口）；② 非流式请求上游回 SSE 判「给错格式」不可提交、换下家（2026-09-28 收口，此前为不校验的残余盲区）；③ 流式请求上游回非 SSE（JSON/HTML/空体）判失败换下家——忽略 `stream:true` 直接回 JSON 的供应商会被判不可提交；④ 判据只识错误形状——`data: {"foo":1}` 这类「合法 JSON 但语义垃圾」与单帧 `[DONE]` 空流会判可提交（直通链路无解析步骤兜底，由客户端自行发现）；⑤ 断流 error 事件形状（`handler/v1/stream_error.go`）未经各家真机 SDK 验证，现有断言只锁字节级形状契约。
- **执法**：`gate_upstream_status_test.go` 编进 `go test ./...`，判据不止「直接比较」一种写法——`switch`/`case` 列成功码、`StatusCode/100`、`HasPrefix(res.Status,"2")`、跨行写法同样命中；扫描按整文件折叠匹配（逐行喂会放过跨行写法）。放行**只有行级标记** `//gate:upstream-status-allow <理由>`，理由为空判红，标记了却不再违规也判红（防注释沉积）；**不做文件级豁免**——整文件免检等于给一处合法例外买下一整座文件的后门。判据本身有样例测试（`TestGateUpstreamPassingStatusJudgment`/`...Marker`）钉住不漏判、不误判、不自我豁免。所有机械门共用 `gateSourceFiles` 遍历，带数量下限与**锚点文件**断言：2026-09-17 发现 `gate_500_test.go` 因把遍历根目录 `.` 当隐藏目录跳掉而从未扫到任何文件——门是绿的、约束不存在，这类缺陷必须有正向哨兵，不能靠人看。
- **新增判定站点怎么办**：不要新写一份，直接调 `common.IsPassingStatus`；确需与 200/201 不同的例外（某协议规定非 200 即错）**先在本节写明理由**，再在违规行加行级标记。

## 7. 诊断类时间预算单源 + 检测类内容形状门（Stage 19 质量门，2026-09-23）

**时间预算单源**：诊断族（`common` 整域 / `service/healthcheck` / `service/credprobe` / `providers` / `handler/testapi` / `handler/pools`）的非测试代码**禁止**裸写时间字面量（`time.Second/…` 单位表达式、`time.Duration(`/`time.ParseDuration(` 转换，**含 const/var 声明行**）——诊断类时间预算一律登记进 `common/upstream_timeout.go`（值 + 为什么是这个值，这是**唯一**合法住所，写到别处同样判红），调用点只引名字（`common.Diagnostic*`）。为什么：健康检查与两处 `Models()` 曾各写一份 `30*time.Second`，单独改一处就会出现「同一条链路探针说超时、健康检查说成功」的互相矛盾结论（find-issues 2026-09-17）。生产聊天超时不在此列（来自设置项 `SettingKeyRequestHeaderTimeout`，语义独立）。

- 放行只有行级标记 `//gate:diag-timeout-allow <非空理由>`：理由为空判红、标记了不再违规同样判红（防注释沉积）；不做文件级豁免。现存例外两类：`providers/cache.go` 的 Transport 连接池调优（生产/诊断共用，非诊断请求超时参数）、`service/healthcheck/settings.go` 的分钟值单位换算。
- 执法：`gate_diag_timeout_test.go` 编进 `go test ./...`——文本行级判据（剥 `//` 注释后匹配；含 `time.ParseDuration(`，无单位裸数字与跨行写法是已声明的接受性漏判）+ `TestGateDiagTimeoutJudgment`（mustCatch/mustPass 判据自测，含伪标记 `allowX` 不放行）+ `TestGateDiagTimeoutMarker`（标记三态：缺理由 / 失效 / 放行；标记前缀必须以行尾或空白收尾、只认代码行尾注释）+ 常量哨兵（判 `^\s*Name\s*=` **声明形状**而非文本提及，防「删声明留注释」绕门）+ `gateSourceFiles` 遍历（数量下限 + 六族锚点文件；与目录同名的门面文件如 `service/healthcheck.go` 同样在扫描面内）。

**检测类内容形状门**：状态码通过（`common.IsPassingStatus`）之后，健康检查成功分支再判「内容是不是所测协议的合法响应」。判据单源 `transform.ValidateResponseShape(format, body)` 四条口径：① 响应体达 `common.UpstreamDiagnosticReadLimitBytes`（1 MiB）判「形状不可验」而非判健康（判据随函数走，调用点不再各自判超限；LimitReader 区分不了「被截断」与「恰好等长」，达上限即视为不可验）；② 空回复判过（门只挡形状不挡语义，对齐 `RunStreamProbe` 空流 Passed）；③ 非空体须过 `common.JSONContentError`（与 chat 提交门**共用同一份**错误形状判断）——**200 + HTML 登录页 / 垃圾页 / 错误信封在这条挡下**；④ 必须能被所测 wire format 的解析入口解析出响应对象——反代回的「合法 JSON 对象但字段类型全不对」在这条挡下（协议形状含字段类型）。响应形状口径与请求体同源（`chooseTestProbe` ← `providers.WireFormatOf`）；刻意不判 Content-Type 头（网关乱写头常见，判内容误判率更低）。失败走既有失败链（error 日志 → 连败 → 衰减/停用），错误文本**带上游状态码**（§6 解码失败口径）。**与 chat 提交门的口径差（刻意）**：直通链路不解析、能吞下字段怪异的响应，本门判不健康——关联同时服务协议转换链，在那条链上这类响应会解析失败；失败后果是连败到阈值自动停用，错误文本可归因、autoEnable 可自愈。**残余边界（与提交门边界④同口径）**：`{"code":401,"message":"…"}` 这类「合法 JSON 对象但语义是错误页」两家都不挡。探针（credprobe / testapi 点测）刻意**不**加形状门：它们答的是「key 活不活 / 链路通不通」（`ProbeBody` 注释：只求状态码通过判定），不是「这条关联能不能服务聊天」。读体与时间两道预算：读体走 `common.ReadCappedBody` 且读错误显式判失败（**读不到 ≠ 空回复**）；时间派生 `common.DiagnosticCompletionTimeout` 整体 ctx（client 整体超时恒 0，滴流不止会卡死整轮巡检）。回归测试：`service/transform/shape_test.go` + `service/healthcheck/upstream_status_test.go`（形状门 / 超限边界 / 读错误 / 201+空体 / 请求响应同源）。
