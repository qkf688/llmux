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
| **auth** | `service/auth/`、`handler/auth/`、`middleware/auth.go` | 密码哈希、JWT 签发/解析、登录/me/轮换 API key/改密码、JWT+API key 中间件 |
| **ops-io** | `handler/importexport/`、`handler/database/` | 配置导入导出、DB stats/vacuum/export |
| **test** | `handler/testapi/`、`handler/testsupport/` | 供应商连通/工具/structured/thinking/vision 测试与测试辅助 |
| **shared-http** | `httpresp/`、`handler/httpx/` | 管理端 JSON 信封；分页/绑定辅助（**渐进采用**） |
| **shared-lib** | `common/`（含 `common/bgtask/`）、`consts/` | 无业务编排的工具与常量；`bgtask` 为后台 goroutine 登记/排空器 |

- **不负责什么**：chat 选路、虚拟模型策略、协议转换、各业务 CRUD 规则
- **对外暴露**：`RegisterAll`、`Deps`、`middleware.AuthJWT`、`middleware.AuthAPIKey`、`service/auth.{HashPassword,VerifyPassword,Sign,Parse}`、`httpresp.*`、`httpx.*`、`bgtask.{Go,SetDefault,Default}`、import/export/database/test REST
- **依赖谁**：聚合各 handler 子包与 service 启动接口；`middleware`/`handler/auth` 依赖 `repository.UserRepo`（DIP）；`service/auth` 只依赖 bcrypt/jwt 标准库，无循环 import

## 3. 内部结构

```
# bootstrap
main.go / middleware/ / handler/register*.go / repos.go
# auth
service/auth/ / handler/auth/ / middleware/auth.go
# ops-io
handler/importexport/ / handler/database/
# test
handler/testapi/ / handler/testsupport/
# shared
httpresp/ / handler/httpx/ / common/ (dataurl, maputil/, bgtask/) / consts/
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `handler.RegisterAll` | 注册全部业务路由（新域需改此函数挂接） | `handler/register.go` | 调用各子包 `Register*` |
| `handler.Deps` | 注入 JWTSecret 等启动依赖 | `handler/register.go` | `main` 构造 |
| `middleware.AuthJWT` | 管理后台 JWT 鉴权（注入 currentUser 到 context） | `middleware/auth.go` | 同文件 |
| `middleware.AuthAPIKey` | /v1 代理 API 的 per-user API key 鉴权 | `middleware/auth.go` | 同文件 |
| `service/auth.Sign/Parse` | JWT 签发与解析 | `service/auth/jwt.go` | 同包 |
| `service/auth.HashPassword/VerifyPassword` | bcrypt 密码哈希与校验 | `service/auth/password.go` | 同包 |
| `service/auth.BootstrapAdmin` | 首启动创建 admin 账号 | `service/auth/bootstrap.go` | `main` 调用 |
| `repository.SetDefault` / `Default` | 默认仓储绑定 | `repository/repository.go` | `main` + handler `repos()` |
| `bgtask.Go` / `SetDefault` / `Default` / `(*Manager).Shutdown` | 登记 fire-and-forget 后台 goroutine，关闭时统一排空（有超时上限） | `common/bgtask/bgtask.go` | `main` 注入 Manager；`service/chat`、`service/healthcheck`、`service/modelsync`、`handler/*` 调 `bgtask.Go` |
| `models.Close` | 关闭底层 `*sql.DB`，**须在** bgtask 排空之后调用 | `models/init.go` | `main.shutdown` |
| `httpresp.Success/Error*` | 统一管理端响应 | `httpresp/` | 同包 |
| `testapi.ResolveAssociationLink` / `ResolveProviderLink` | 连通性测试的**唯一**上游链路口径：主协议端点（`SelectPrimaryEndpoint`）→ 候选分组（关联级按白名单 `GroupCandidatesForModel`；供应商级不过滤，显式 `group_id` 时锁组）→ 组内首个可用凭据（`channel.ProbeGroup`，掩码只进 `slog`）。装配失败即 401 类缺陷（#19），六个测试端点（`GET /api/test/:id`、`POST /api/providers/:id/test`、`/test/react/:id`、`/test/structured-output/:id`、`/test/thinking/:id`、`/test/vision/:id`）**禁止**直用 `Provider.Config` 造客户端 | `handler/testapi/models.go` | `handler/testapi/{provider_handlers,react_handler,structured_output_handler,thinking,vision}.go`（structured/thinking/vision 的装配/探针/失败回包公共流在 `completion_flow.go`） |
| `testapi` 测试结果形状 | 链路装配成功之后的一切结论（含上游非 2xx）都用 HTTP 200 + 信封 `code:200` + `data:{passed, error?, error_type?, group_id, group_name}`；解析**之前**的问题（ID 非数字、显式 `group_id` 不属于该供应商、记录不存在）才是 400/404 信封。`group_id`/`group_name` **恒在**（未选定链路 = `0` / `""`），前端按 `(model, group)` 把 ✓/✕ 归位到分组徽标（C2 形态），所以失败不能走非 200 信封——`apiRequest` 遇 `code!=200` 只抛 message、丢掉 data。`/test/react/:id` 是 SSE 流（`fetchEventSource` 消费），只复用 `respondRequestError` 判调用方错误，其余结论走 `error`/`success` 事件。**探针阶段的 `error_type`/失败文案/细节由 `credprobe.RunCompletionProbe` 统一产出**，端点只负责套信封（`BuildDetailedError` / `summary: detail`）；`error_type` 值域单一来源 = `common`（前端 `webui/src/lib/errors.ts` 的 `ERROR_TYPES` 镜像该值域） | `handler/testapi/respond.go` | `webui/src/lib/api/modules/catalog/{providers,model-providers}.ts` 的 `*TestResult` |
| `common.ErrorTypeFromStatus` | 诊断测试 `error_type` 的**唯一**状态码映射：`{network,auth,provider,timeout,validation,unknown}` 常量 + 状态码→类型 switch（401/403=auth、404/≥500=provider、其余 4xx=validation、其他=unknown）；`network`/`timeout` 无状态码可判，由 `credprobe.RunCompletionProbe` 在 Do 与读 body 两个错误分支按 `errors.Is(DeadlineExceeded)` 区分（滴流超时归 timeout，不归 network）。下沉自 `handler/testapi`（旧 `getErrorTypeFromStatus` 薄委托已删除），因 #24-B 若另写一份必与之漂移 | `common/errtype.go` | `service/credprobe/completion_probe.go`、`handler/testapi/react_anthropic.go`；前端 `webui/src/lib/errors.ts` |
| `common.ExtractUpstreamErrorDetail` / `UpstreamErrorDetail` | 上游错误体细节提取**唯一**来源：沿 JSON 信封的 `error`/`message` 字段**递归**追最内层人读文本（含「`message` 本身又是 JSON 信封」的网关套娃，如 opencode go；层数预算 5），追不到回退原文，统一截断 512B；`UpstreamErrorDetail` 附加 `HTTP 状态码: N` 行。旧 `testapi.parseProviderErrorDetail`（不截断）与 `pools.truncateDetail`（不解析）双轨已收敛于此 | `common/upstream_detail.go` | `credprobe.RunCompletionProbe`、`handler/testapi/react_anthropic.go` |
| `credprobe.RunCompletionProbe` | 「一次真实最小 completion 探针」编排的**唯一**来源（BuildReq→带 deadline 的 Do→网络/超时归类→读 body→非 2xx 归类→细节提取；通过=任意 2xx；`Timeout≤0` 兜底 `DefaultCompletionTimeout`，须传常量口径避免放大 client 缓存）。ctx deadline 刻意早于 transport 超时（+`CompletionTransportBuffer`），慢上游（含 headers 后 body 滴流）稳定归 `timeout` 而非误归 `network`。管理端**新增 completion 类测试端点必须调本函数，禁止再抄编排**；client 装配与探针体仍归各调用方（链路语义不同） | `service/credprobe/completion_probe.go` | `handler/testapi/{provider_handlers,completion_flow}.go`、`handler/pools/credential_probe.go` |

## 5. 特殊约定

- **禁止**在 `main.go` 增加业务路由明细；新 API 域改 `RegisterAll` + 子包 `Register*`
- 鉴权：`/api/*` 用 `middleware.AuthJWT`（登录路由 `auth.RegisterLogin` 在 AuthJWT 之前挂，免鉴权）；`/v1/*` 用 `middleware.AuthAPIKey`（per-user key，兼容 `Authorization: Bearer` 与 `x-api-key`）。`service/auth` 是纯工具层，不依赖 repository/models
- `database`/`importexport` 允许 PRAGMA/VACUUM/`c.File`/事务 SQL 等白名单直连
- `httpx`：**存在** Strict/Loose 分页与 `BindJSON`，但 handler 仍大量手写 `ShouldBindJSON`——属渐进落地，非已全量统一
- `common` 不再依赖 Gin（响应已迁 `httpresp`），避免沦为业务垃圾桶
- **优雅关闭序**（`main.shutdown`）：`srv.Shutdown`（停收新请求 + 等在途，**必须带 timeout**——SSE 回写无 deadline）→ `credwrite.Stop`（Flush 凭据写队列 + 停收，10s 超时，**必须**在此处、`bgtask.Shutdown` 之前）→ `bgtask.Default().Shutdown`（排空后台写库 goroutine）→ `models.Close()`；顺序不可颠倒，排空前关库会让在途写库任务撞上「DB 已关闭」。注意 ticker 服务的停止由信号 ctx 取消驱动，与「等在途请求」**并行**发生，不在上述四步之内；排空在两者之后
- 关闭的**唯一入口**是取消 `main` 的可取消 ctx：信号与 `ListenAndServe` 启动失败共用此路径，**禁止**在任何分支用 `os.Exit` 旁路 `shutdown`（会硬切已登记的后台写库任务）
- 容器/编排部署**必须**把宽限期设到大于关闭预算（`serverShutdownTimeout` + 2×`bgtaskDrainTimeout`，当前 30s+10s+10s）：compose `stop_grace_period`、k8s `terminationGracePeriodSeconds`；否则进程在排空完成前被 SIGKILL，优雅关闭形同虚设
- **禁止**用裸 `go f(context.Background())` 起会写库的后台 goroutine；**必须**经 `bgtask.Go` 登记。请求内且已用 `sync.WaitGroup` 等待完成的 goroutine 例外（由 `srv.Shutdown` 覆盖）
- 连通测试**一次解析一次请求**：`buildLink` 跨候选组只补「本组无可用凭据」，上游返回失败**不**跨组重试——测试体是真实计费的 completion（`providers/meta_bodies.go`）且前端批量并发 3，放大成 N 倍花费不可接受；react 是 SSE 多轮，重跑会重复推事件。代价已知：组内只验首个可用凭据，排在后面的坏 key 由 credprobe / 号池详情按凭据测试覆盖
- 前端静态：`//go:embed webui/dist`，`NoRoute` 对非 `/api` `/v1` 的 GET 回 `index.html`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写子边界；2026-08-16 补充优雅关闭序与 `bgtask` 契约；2026-09-09 补充 testapi 诊断链路与测试结果形状（#19）；2026-09-12 completion 探针编排收敛至 `credprobe.RunCompletionProbe` 后回写契约；2026-09-13 新增 thinking 能力测试端点（`/test/thinking/:id`）；2026-09-14 新增图片识别能力测试端点（`/test/vision/:id`）。*
