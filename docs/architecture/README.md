# LLMux 架构总览

> 本文档是架构文档的入口，包含全局性内容。各模块详情见 `modules/{x}.md`。
> 定位：现状描述。操作指南（如何新增模块、架构纪律、自检清单）见 `AGENTS.md`。

## 1. 项目概述

| 属性 | 值 |
|------|-----|
| 项目类型 | 全栈（Go 后端 API 网关 + React 管理后台） |
| 技术栈 | Go 1.25 + Gin + GORM/SQLite；React 19 + TypeScript 5.8 + Vite 7 + Tailwind CSS 4；TanStack Query + Zustand |
| 架构模式 | 技术分层 + 业务子包拆分（handler/service 按业务域分子包；扩展点用注册表） |
| 包管理 | Go modules；前端 pnpm |
| Monorepo | 否（单仓前后端，前端 `webui/` 构建产物由 Go `embed`） |
| 设计目标 | 多供应商统一出口、协议互转、可扩展选路与故障转移、管理后台可运维 |

LLMux 是多供应商 LLM API 网关/代理：对外提供 OpenAI / Anthropic 兼容的 `/v1/*` 接口；对内提供 Web 管理后台，用于配置供应商、模型关联、虚拟模型、健康检查、模型同步，以及查看日志与指标。

## 2. 设计原则落地

本架构以 **SRP / OCP / DRY / DIP** 四大原则为基准（原则通用定义不在此重复）。本节只写**本项目如何落地**这些原则的关键决策。

- **OCP 落地点**（注册表可用，但多数扩展仍有「中心文件」要改）：
  - 新供应商类型 = `providers` 实现 + `Register` / `RegisterMetadata`（`init`），**工厂分发无需改**；外围仍可能有 type 特判（见 providers 模块）
  - 新协议 style = **多点注册**：`Beforer`/`Processer` + `FormatAdapter` +（流式）`RegisterRealtimeRoute` + `handler/register_v1` 路由 + `consts.Style*`，不是只改两处
  - 新虚拟模型策略 = `virtualmodel.RegisterSelector`（分发不改）；**算法现状多在 `Service.selectBy*`**，前端策略枚举仍硬编码
  - 新设置项 = `setting_schema.go` + **handler DTO** + **前端 Settings 类型/表单**（schema 驱动读写分发，但契约未单点生成）
  - 新管理端页面 = `webui/src/routes/` + `route-config.ts` 追加一项
  - 新管理 API 域 = 新 `handler/*` 子包 + **修改** `handler.RegisterAll` 挂接（禁止写进 `main.go`）
- **DIP 落地点**：
  - `providers.Provider` / `OpenAICompat`、`virtualmodel.Selector`、`transform.FormatAdapter`、`chat.Beforer`/`Processer` 为真实抽象
  - `repository.*Repo` 接口存在；**管理端 CRUD 与 chat/chatstats/metrics/healthcheck/adjustment/modelsync/modelapi 主路径已走仓储**；settings、v1、virtualmodel 等仍有 `models.DB` 直连（渐进落地）
  - 跨域副作用：`healthcheck.AdjustmentHooks`、`modelsync.ActionHooks`（调用方定义契约；实现为包级注入，非构造 DI）
  - 默认仓储绑定：`main` → `repository.SetDefault`；`service` 根包门面是 **兼容 re-export**，非第二套业务实现
- **SRP 落地点**：
  - HTTP 在 `handler/*`；上游适配在 `providers/`；协议转换在 `transform/`
  - **`service/chat` 为编排核心**（选路/重试 + 日志落库编排 + style 注册）；**Stats 在 `service/chatstats`，权重调整在 `service/adjustment`**
  - 虚拟模型只解析「虚拟→真实」，不发起上游调用
  - 健康检查：权重/优先级经 Hook 回调 `service/adjustment`；关联 Status/连续失败由 healthcheck 经 `ModelWithProviderRepo` 更新
- **DRY 落地点**：
  - 协议中枢：`models/unified/` 为主；**部分响应类型仍在 `models/unified.go`**
  - 设置元数据：`setting_schema.go` 为默认值/解析主源；**热读存在多路径**（models getter / settings.Reader / 部分业务包自读）
  - OpenAI 兼容：`providers/openai_base.go`；管理端响应 `httpresp/`；绑定辅助 `httpx`（**BindJSON 未全量铺开**）
  - 前端：`lib/api` 模块化 + `route-config` 单一路由源

## 3. 全局架构图

（用文字规则表达模块间依赖关系。）

### 允许的依赖（A → B 表示 A 依赖 B）

- `webui` → `handler`（经 `/api` HTTP）
- `handler`（含 `v1` / 管理 API） → `service`（业务编排与门面）
- `handler` → `repository`（管理端 CRUD 数据访问）
- `handler` → `httpresp` / `handler/httpx` / `middleware`
- `service` → `providers`（上游请求构造与模型列表）
- `service` → `models`（实体、设置读、统一协议类型）
- `service/chat` → `service/transform`、`service/virtualmodel`、`service/chatcore`、`service/chatstats`、`service/adjustment`、`service/channel`（三层选路）、`service/credprobe`（惰性探活）、`balancer`
- `service/channel` → `consts`、`common/credentialcrypto`（选路产物可被 `providers.New` 反序列化，不依赖 `providers`）
- `service/modelsync` → `service/channel`（按分组拉取用 `/models`）
- `service/adjustment` → `service/healthcheck`（仅 `init` 注入 `AdjustmentHooks`，无反向业务依赖）
- `service/transform` → `models/unified`、`service/anthropic`、`service/responses`
- `repository` → `models`（GORM 实体与 `models.DB`）
- `providers` → `consts`（及少量 `common` 工具）
- 任意业务层 → `common` / `consts` / `balancer`（共享能力，无业务编排）

### 禁止的依赖

- ❌ `repository` → `service` / `handler`（下层不依赖上层）
- ❌ `models` → `service` / `handler` / `repository` 业务逻辑
- ❌ `providers` → `handler` / `service` 业务编排
- ❌ `common` / `httpresp` / `consts` / `balancer` / `models/unified` → 业务模块
- ❌ `middleware` → `handler` 业务包
- ❌ `main` 直接注册业务路由明细（应只调 `handler.RegisterAll`）

### 层次总览（一句话）

`webui` / 外部客户端 → `middleware` + `handler` → `service`（chat / transform / virtualmodel / healthcheck / modelsync / channel / settings）→ `providers` + `repository` → `models` + SQLite；`common` / `consts` / `balancer` / `httpresp` 被上层依赖，自身不依赖业务。

### 请求主路径（文字）

1. **代理路径**：`POST /v1/chat/completions|responses|messages` → `handler/v1` → `service` 门面 → `service/chat`（可选虚拟模型解析）→ `channel` 三层选路（端点→分组→凭据，见 [modules/channel-routing.md](modules/channel-routing.md)）→ `transform` → `providers.BuildReq` → 上游 → 流式/非流式回写 + `ChatLog`/`Stats`
2. **管理路径**：`/api/*` + `middleware.Auth` → 各 `handler/*` 子包（含 `handler/pools` 号池/凭据/端点/分组管理域）→ `repository` 和/或 `service` → `models`/SQLite
3. **后台任务**：`main` 启动 `HealthChecker`、`ModelSyncService.StartAutoSync` 与 `credwrite.Start`（凭据写队列 worker）；ticker 服务（健康检查/模型同步）ctx 派生自 `signal.NotifyContext`，队列 worker 退出由 `Stop` 的 requestStop 驱动（与 healthcheck 同构）；请求路径里 fire-and-forget 的写库任务经 `common/bgtask` 登记
4. **优雅关闭**：SIGINT/SIGTERM → 信号 ctx 取消（ticker 服务的停止与「等在途请求」**并行**收敛，不在下述步骤之内）→ `srv.Shutdown`（等在途请求）→ `credwrite.Stop`（Flush 凭据写队列 + 停收）→ `bgtask.Default().Shutdown`（排空后台写库）→ `models.Close()`，逐步限时。`ListenAndServe` 启动失败取消同一 ctx，走同一条关闭序

### 现状备注（依赖事实）

- 目标分层是 `handler → service → repository → models`。**聊天主路径（`service/chat`、`service/chatstats`）、`handler/metrics`、`service/healthcheck`、`service/adjustment`、`service/autoassoc`、`service/modelsync`、`handler/modelapi` 已全部经 `repository.*Repo` 访问持久层**；存量直连集中在 `handler/settings`、`handler/v1`、`handler/database`、`handler/importexport` 与 `service/virtualmodel`（构造持 `*gorm.DB`）。`repository` 是渐进落地的抽象，不是全路径强制边界。
- 自动关联已收敛为 **`service/autoassoc` 单一入口**；handler HTTP 与 modelsync ActionHooks 均委托该服务（见 associations 模块）。
- 文档描述以**当前代码结构**为准，不以理想重构目标替代现状。

## 4. 模块索引

| 模块 | 业务领域 | 详情文件 |
|------|----------|----------|
| `chat-gateway` | LLM 代理 / 选路 / 重试 / 日志写入（编排核心 + chatstats/adjustment 旁路） | [modules/chat-gateway.md](modules/chat-gateway.md) |
| `protocol-transform` | 协议互转与统一模型 | [modules/protocol-transform.md](modules/protocol-transform.md) |
| `providers` | 供应商适配与元数据 | [modules/providers.md](modules/providers.md) |
| `models-catalog` | 真实模型目录与模板 | [modules/models-catalog.md](modules/models-catalog.md) |
| `associations` | 模型-供应商关联与自动关联 | [modules/associations.md](modules/associations.md) |
| `virtual-models` | 虚拟模型与策略选路 | [modules/virtual-models.md](modules/virtual-models.md) |
| `pools` | 供应商号池 / 凭据 / 协议端点 / 分组 | [modules/pools.md](modules/pools.md) |
| `channel-routing` | 供应商内部三层选路（端点匹配 / 分组加权 / 凭据轮询+冷却） | [modules/channel-routing.md](modules/channel-routing.md) |
| `health-check` | 健康探测与关联启停 | [modules/health-check.md](modules/health-check.md) |
| `model-sync` | 上游模型同步 | [modules/model-sync.md](modules/model-sync.md) |
| `logs-metrics` | 请求日志、ChatIO、仪表盘指标 | [modules/logs-metrics.md](modules/logs-metrics.md) |
| `settings` | Schema 驱动系统设置 | [modules/settings.md](modules/settings.md) |
| `system-ops` | 横切接入与运维（bootstrap / ops-io / test 合集，非单一业务实体） | [modules/system-ops.md](modules/system-ops.md) |
| `webui` | 管理后台前端 SPA | [modules/webui.md](modules/webui.md) |

> 新增业务域时：新建 `modules/{name}.md` 并在本表加一行；不改已有模块文件正文。  
> `system-ops` 是文档上的横切合集（非单一业务实体），内部子边界见该模块文件。

## 5. 相关文件

| 文件 | 用途 |
|------|------|
| [modules/](modules/) | 各业务模块的职责、边界、接口契约 |
| [interactions.md](interactions.md) | 模块间交互方式 |
| [conventions.md](conventions.md) | 命名规范 |
| [../virtual-models-guide.md](../virtual-models-guide.md) | 虚拟模型使用指南（用户向，非架构） |

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-08-21*
