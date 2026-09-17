# 框架专属约定

> 按需加载文档。操作规范总入口见 [AGENTS.md](../../AGENTS.md)。
> 本文收录 Go/Gin、React/Vite、协议代理三块框架特有的踩坑约定——多数条目源自真实事故，改这些区域前先读对应小节。

## 1. Go / Gin

- 路由：`RegisterAll` 建 `/v1` 与 `/api`；`/v1` 挂 `middleware.AuthAPIKey(apiKeyRepo, userRepo)`，`/api` 登录路由免鉴权，其余挂 `middleware.AuthJWT(secret, repo)`。
- 引擎装配（`main.go`，**不用** `gin.Default()`）：`RequestID()`（请求编号，Stage 3，见 §5）→ `AccessLog()`（等价原 gin Logger + rid，path 走 %q 防换行注入）→ `Recovery()`（Stage 3 自建，替代 `gin.Recovery`：panic 型 500 同走 httpresp 收口，见 §5）→ gzip。**禁止**把 `RequestID` 挪到鉴权/路由之后——401/404 也要有号可对。
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
- 页面**不得**绕过 `lib/api` 核心客户端自造鉴权 header 逻辑（除非扩展 client 本身）。
- API 信封约定：`{ code, message, data }`，`code !== 200` 抛错；401 跳转 `/login`。
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

- 诊断 / 探针 / 连通测试路径：`common.ReadCappedBody`（1 MiB 单源；上游未通过（非 2xx）的错误体同口径）。
- 入站请求体：`consts.MaxInboundBodyBytes`（20 MiB 硬上限，代码常量刻意不做成设置——可被调小的防线不是防线）+ `io.LimitReader(body, max+1)` 多探 1 字节拒绝；`/v1` 超限回真实 413（SDK 不自动重试），管理端按该端点信封惯例（前端对非 2xx 不解析 body）。
- 上游非流式成功响应：`service/chat` 的 `limitUpstreamResponseBody` 在 attempt 内、响应头提交给客户端**之前**整读加限（设置项 `upstream_max_response_mib`，默认 32 MiB、下限 1 不可关闭），未超限回填为可重读内存字节。失败语义：超限=端点级异常 → 组织级转移**不冷却凭据**；读体错误（含 ctx 超时打断读体）与 `Client.Do` 失败**成对**归凭据级网络——超时分类只挂 Do 侧会让读体超时静默漏走（见全局 RULES「Go HTTP 超时」）。
- 原始采集落库：四文本字段（`RequestBody`/`RawRequestBody`/`RawResponseBody`/`Error`）共用单字段上限（设置项 `raw_log_field_max_kib`，默认 256 KiB），截断一律 `common.TruncateUTF8`（rune 边界 + `…(truncated)` 标记，按字节劈半进 JSON 必出 U+FFFD 乱码尾）。采集读头不吞尾（MultiReader 回填），旁路不得成为下游截断点。
- 流式 SSE 不整读、天然不在本防线内；帧级/累积侧信道截断策略另有约定（见 logs-metrics.md），提高上限需单独立项。

豁免**必须**在读取处注释说明「体积已被上游哪一道限定」——现存唯一豁免：`service/transform/transform_provider_response.go` 整读（已被 attempt 体积闸限定）。可机械复核：`rg 'io\.ReadAll' --type go -g '!*_test.go'` 的命中只允许是上述白名单入口或带豁免注释处。

## 5. HTTP 500 错误响应收口（Stage 3 质量门，2026-09-17）

HTTP 500（本机自己坏，**含 panic**）**禁止**把内部 `err` 原文回给调用方（SQLite/gorm 报错含表名/列名/约束名）；原始错误只进服务端日志。**禁止**绕过 httpresp 手搓 500（`c.JSON(500,…)`、`http.Error`、直用 `StatusInternalServerError` 常量）。合法出口只有两个，两者都先 `slog`（带 `request_id`/`method`/`path`/原始 `error`）再回响应；静态出口的 `message` 也一并进日志（同名分支靠它定位是哪一处）：

- `httpresp.InternalServerError(c, err error)`：对外恒发通用文案 `internal server error [req=<编号>]`（编号内嵌，SDK 用户报障时拿得走），原始 err 只进日志。拼接式旧写法（`"前缀: "+err.Error()`）拆到这里或下面的静态入口（前缀留下、err 进日志）。
- `httpresp.InternalErrorMessage(c, message string, err error)`：仅用于**已审查、不含内部信息**的静态提示（如 `"encryption not configured"`）。`message` **必须**是字符串字面量或纯业务标识符（`action`、`ie.Message`——其取值须可追回常量），**禁止**拼接任何错误文本（`err.Error()`/`fmt.*`/`+`）；`err` 为仅服务端可见的原始错误，配置守卫类无 err 时传 `nil`。
- **级别约定**：客户端断连类（`context.Canceled`/坏管道/连接重置）留痕降 `Warn`——不是服务器故障，防「建连即断」刷爆 ERROR 告警通道；其余一律 `Error`。

**范围边界**：只收本机 500。① 4xx 原文（校验/绑定错误，管理端表单报错提示依赖它）不改；② `/v1` 上游状态码透传（`handler/v1/chat.go` 的 `errors.As(statusCoder)` 分支，转发的是供应商错误，属协议内容）不在本防线。

**请求编号**：`middleware.RequestID()` 挂在所有路由最外层（`main.go` 于 gzip 之前），为每请求定一号——入站 `X-Request-Id` 格式规矩则沿用（对齐调用方自家 trace），否则生成 12 位 hex；编号写三处（context / 响应头 / 回写入站头，令 `middleware.AccessLog` 访问行与 `/v1` 上游请求透传都指向同一号）。上游**响应**自带的 `X-Request-Id` 在 `writeHeader` 改名 `X-Upstream-Request-Id` 回传——网关命名空间唯一，报障编号无双义。`middleware.Recovery` 替代 `gin.Recovery`：panic 栈经 slog 留痕（带 rid），未提交的响应走 `InternalServerError` 收口（客户端拿得到 `[req=]`），已提交半开流不追加 body。

执法：两条静态判据编进 `go test ./...`（`gate_500_test.go`，见上「禁止绕过」「message 禁拼接」）；人肉复核命令（与门同判据）：`rg -n 'httpresp\.InternalErrorMessage\([^,]+, [^,)]*(\+|\.Error\(\)|fmt\.)' --type go -g '!*_test.go'` 零命中。

## 6. 上游响应状态码通过口径（Stage 5 质量门，2026-09-17）

判「这次上游算不算成功」的**唯一入口是 `common.IsPassingStatus(statusCode int) bool`：2xx（`[200,300)`）即通过**。非测试代码**禁止**对上游状态码做判通过的裸比较（`res.StatusCode != http.StatusOK`、`statusCode == 200` 这类），无论写在哪个包。

- **为什么是 2xx 而不是正好 200**：OpenAI 兼容网关有相当一部分照抄 OpenAI 某些接口的习惯回 201，它们工作正常、内容有效。旧的严格 `== 200` 把这类供应商判死；而健康检查这一处判失败的代价不是「诊断报错」，是**自动停用模型-供应商关联**（`handleCheckResult` 连续失败到阈值即停用，且停用原因不入库）——线上静默少一条可用线路，后台只显示「已停用」，看不出是状态码口径造成的。
- **为什么落 `common` 而不是由 `credprobe` 导出**：同一条判定有 7 处需要（`service/healthcheck`、`service/credprobe` 两个探针、`service/chat` attempt、`providers` 两处 `Models()`、`handler/testapi` react），其中 providers 层**禁止** import service 子包（依赖方向），跨 2 域以上的纯逻辑必须下沉 `common`（AGENTS §4.4）。
- **边界（刻意不管的两件事）**：① 只判状态码、**不判内容**——上游回 2xx 但内容并非合法响应（被重定向到登录页、一页 HTML 错误）不在此拦截；各调用点对内容的要求本就不同（聊天要能解析成回复、模型列表要清单、流式测试要真在吐字），捆进同一个判定会让改聊天校验连带牵动模型列表（已登记残留待办）。② 按**具体失败码**分类的写法合法（401/403 归鉴权、429 归限流、5xx 归供应商，如 `chat_attempt_error.go` 的 429 降权、`common.ErrorTypeFromStatus`）——那是「失败之后怎么处置」的另一个决定，不在「算不算通过」这条线上。
- **执法**：`gate_upstream_status_test.go` 编进 `go test ./...`——裸比较 2xx 成功码即红；豁免表按文件登记，且**列了却不再命中也算红**（防清单悄悄过期成橡皮图章）；同文件另有判据样例测试钉住门本身不漏判/不误判。所有机械门共用的目录遍历带**空转哨兵**（扫到源文件数 < 100 即红）：2026-09-17 发现 `gate_500_test.go` 因把遍历根目录 `.` 当隐藏目录跳掉而从未真正扫到任何文件——**新增门必须保留这道哨兵**，否则门是绿的、约束不存在。人肉复核：`rg -n '\.StatusCode\s*(==|!=|<|>|<=|>=)' --type go -g '!*_test.go'`，命中处须是按具体失败码分类。
- **新增判定站点怎么办**：不要新写一份，直接调 `common.IsPassingStatus`；确需与 2xx 不同的例外（某协议规定非 200 即错）**先在本节写明理由**，再进豁免表。
