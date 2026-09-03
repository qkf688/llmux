# 框架专属约定

> 按需加载文档。操作规范总入口见 [AGENTS.md](../../AGENTS.md)。
> 本文收录 Go/Gin、React/Vite、协议代理三块框架特有的踩坑约定——多数条目源自真实事故，改这些区域前先读对应小节。

## 1. Go / Gin

- 路由：`RegisterAll` 建 `/v1` 与 `/api`；`/v1` 挂 `middleware.AuthAPIKey(repo)`，`/api` 登录路由免鉴权，其余挂 `middleware.AuthJWT(secret, repo)`。
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
- 认证：管理后台 `/api/*` 用 JWT（`Authorization: Bearer <JWT>`，登录走 `POST /api/auth/login`）；代理 `/v1/*` 用 per-user API key（`Authorization: Bearer <key>` 或 `x-api-key: <key>`，Anthropic 兼容）。
- 流式：SSE；转换与 process 路径分流行/非流，**禁止**混用缓冲假设。
- 两层 LB：虚拟→真实（virtualmodel）+ 真实→供应商（chatcore/balancer）；改一层时**禁止**破坏另一层的候选过滤（如黑名单在候选池）。
