# chat-gateway 模块

> 本文件描述 LLM 代理网关的职责、边界、对外暴露、依赖、接口契约。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | LLM 请求代理：解析、选路、调用上游、流式回写、重试与日志 |
| 目录位置 | `handler/v1/`；`service/chat/`；`service/chatcore/`；`service/chatstats/`；`service/adjustment/`；`service/chat_facade.go`；`balancer/` |
| 主要职责 | 将客户端 OpenAI/Anthropic 兼容请求路由到可用供应商并记录结果 |
| 包形态 | **编排核心 + 旁路叶子包**（选路/重试/协议仍在 `service/chat`；Stats 与权重调整已拆出） |

## 2. 职责与边界

- **负责什么**：按协议 style 解析请求；解析模型名（真实/虚拟）；能力匹配；选择 `ModelWithProvider`；执行上游 HTTP（含重试）；流式/非流式处理；编排写入 `ChatLog`/`ChatIO`（经 chat 内 record 路径）与统计（经 `chatstats`）；失败/成功时触发权重/优先级调整（经 `adjustment`）；健康检查 `AdjustmentHooks` 由 `adjustment` 包级注入
- **不负责什么**：协议字段级互转细节（`protocol-transform`）；供应商 CRUD（`providers` 管理端）；关联/虚拟模型配置 CRUD；健康检查与模型同步调度；stats/权重算法本体（旁路包）
- **对外暴露**：`service` 门面中的 `Beforer`/`Processer`/`ProvidersWithMetaBymodelsName`/`BalanceChat`/`RecordLog` 等；`handler/v1` 的 `/v1/models`、`/chat/completions`、`/responses`、`/messages`
- **依赖谁**：`providers`、`protocol-transform`、`virtual-models`、`repository`（持久层唯一入口）、`models`（实体/设置）、`balancer`/`chatcore`、`chatstats`、`adjustment`、`consts`

## 3. 内部结构

```
handler/v1/                 # /v1 入口与 chatHandler
service/
├── chat_facade.go          # 根包门面 re-export
├── chat/                   # 编排核心
│   ├── registry.go         # Beforer/Processer 注册表
│   ├── before.go           # 请求前处理
│   ├── process.go          # 响应/SSE 处理
│   ├── chat_balance*.go    # 选路入口与虚拟模型故障转移（真实/虚拟两条路径的编排）
│   ├── chat_retry_loop.go  # 单候选池上的重试循环（两条 balance 路径共用）
│   ├── chat_attempt*.go    # 单次上游尝试（types/request/log/error 按职责拆分）
│   ├── chat_record*.go     # 日志后处理编排与 raw 清理
│   ├── chat_provider_meta.go # 供应商元数据装配 + 模型-供应商查询
│   ├── chat_log_storage.go # ChatLog 落库与保留
│   └── preprocess/openai/  # OpenAI 预处理
├── chatstats/              # 请求/token/供应商统计写入（叶子）
├── adjustment/             # 权重/优先级调整 + AdjustmentHooks 注入（叶子）
└── chatcore/               # 优先级+权重选择、Header 构造
balancer/                   # 加权随机纯算法
```

各子目录职责：
- `handler/v1/`：HTTP 接入与 style 分发
- `service/chat/`：端到端编排与注册表；日志落库编排；调用旁路包
- `service/chatstats/`：Stats* 累加（无 chat 类型依赖）
- `service/adjustment/`：关联权重/优先级/连续失败；向 healthcheck 注入 hooks
- `service/chatcore/`：供应商选择算法与请求头
- `balancer/`：无状态加权随机

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Beforer` / `Processer` | 按 style 预处理请求 / 处理上游响应 | `service/chat`（门面 re-export） | style 注册实现 |
| `BalanceChat` | 在候选供应商上执行带重试的转发；入参 `BalanceInput`、返回 `*BalanceResult` | `service/chat_facade.go` | `service/chat` |
| `runProviderRetryLoop` | 在**单个候选池**上执行「选路 → 尝试 → 淘汰」重试循环，产出 `retryLoopOutcome`（成功/穷尽/中止） | `service/chat/chat_retry_loop.go` | `service/chat`（包内，真实与虚拟路径共用） |
| `ProvidersWithMetaBymodelsName` | 按模型名解析候选供应商元信息（含虚拟模型分支） | `service/chat` | `service/chat` |
| `chatcore.SelectByPriorityAndWeight` | 在关联列表上按优先级与权重选供应商 | `service/chatcore/` | `service/chatcore` |
| `balancer.WeightedRandom` | 加权随机选取 | `balancer/balancer.go` | 泛型算法 |
| `chatstats.Record*` | 请求/token/供应商/真实模型统计写入 | `service/chatstats/` | 同包 |
| `adjustment.Apply*` | 成功增权、失败衰减、连续失败 | `service/adjustment/` | 同包（兼 healthcheck hooks） |

## 5. 特殊约定

- **扩展点分布（现状）**：style 注册 `Beforer`/`Processer` + `register_v1` 路由 + transform 适配/流路由 + `consts`；业务路由不在 `main.go`
- **数据访问**：统一经 `repos()`（`service/chat/repos.go` → `repository.Default()`），**禁止**直连 `models.DB`/`gorm.G`；旁路包 `chatstats` 同样经 `repos().Stats`（`repository.StatsRepo`）
- **职责拆分（现状）**：选路/重试/协议/日志落库编排仍在 `service/chat`；Stats 在 `chatstats`；权重调整在 `adjustment`。改统计策略与改选路策略不再同文件碰撞；日志 IO 存储仍可后续下沉
- 虚拟模型路径：先由 `virtualmodel` 产出有序真实模型，再在真实模型层做 provider 级选路（两层 LB）。两条路径的差异只体现在**候选池如何构建**与**穷尽后如何处置**（真实路径整体失败；虚拟路径换下一个真实模型），provider 级重试循环本身由 `runProviderRetryLoop` 单点承载，不再各写一份
- **retryLog 生命周期**：`RecordRetryLog` 消费的通道由 `runProviderRetryLoop` 独占持有（一候选池一通道，`defer close`），调用方**禁止**手动 close——此前虚拟路径在多个返回分支各写一次 close，漏一处即泄漏 goroutine
- **thinking 裁剪（能力标记联动）**：`ProvidersWithMeta` 携带 `Model`（真实路径为查询到的 model；虚拟路径为正在尝试的 ordered model），经 `retryLoopInput.Model` → `singleProviderAttemptInput.Model` 传入单次尝试；`buildRequestBodyForProvider` 在入口处调用纯函数 `stripThinkingFields`（`chat_attempt_request.go`）——当 `ModelWithProvider.SupportsThinkingResolved(model)` 为 `false` 时删除请求体中的 `thinking`/`reasoning_effort`/`reasoning`/`output_config.effort` 字段（Anthropic adaptive thinking 字段），避免不支持 thinking 的上游报 400/静默忽略；`output_config.effort` 删除后若 `output_config` 变空对象则连壳删除，避免残留空对象；失败仅记录日志不阻断主流程。裁剪与 `clampMaxTokens` 同属"构建请求体时的保护性改写"，两条路径（真实/虚拟模型）共享同一入口
- **思考档位钳制（Stage B 扩档）**：`buildRequestBodyForProvider` 重构为接收 `ProviderRequestCaps` 结构体（含 `Style`/`ProviderType`/`Raw`/`MaxTokensLimit`/`SupportsThinking`/`ThinkingClamp`），避免位置参数横向膨胀。`SupportsThinking=true` 时由 `buildThinkingClampConfig`（`chat_settings.go`）从 ctx 读取设置 + `ThinkingLevelsResolved` 白名单构建 `transform.ThinkingClampConfig`，传入 `ProcessRequest`（transform 路径）或 `clampPassthroughReasoning`（passthrough 路径）执行就近钳制 + budget 联动（方案 E）。`SupportsThinking=false` 时 `ThinkingClamp` 为 nil（thinking 已被 `stripThinkingFields` 剥离，钳制无意义）。钳制规则详见 `protocol-transform.md`。
- **budget 超 max_tokens 的协议级收敛**：`reconcileThinkingBudgetWithMaxTokens`（`chat_attempt_thinking_clamp.go`）在两条路径的 `clampMaxTokens` **之后**执行——`clampMaxTokens` 把 `max_tokens` 压到 `MaxTokensLimit` 时不看 `thinking.budget_tokens`，会自造 `budget >= max_tokens` 的非法组合（Anthropic 硬约束，违反必 400）。收敛只降 budget、不抬 max_tokens（运维阀值不能被请求绕过）：仅 `ProviderType=anthropic` 且组合已非法时介入，把 budget 降到 `floor(max_tokens*0.8)`；目标值低于 `anthropic.MinThinkingBudget`（1024）时钳制无意义，整体剥离 thinking。这是独立于白名单钳制的**协议合法性**关注点，故为 `clampMaxTokens` 之后的单独一步而非塞进它。openai 无 budget 字段、responses 的 budget 在 `reasoning.max_tokens`（其上限键 `max_output_tokens` 不被 `clampMaxTokens` 触及），故不存在此冲突。
- `service/chat_facade.go` 为兼容 re-export，不是第二实现
- `chatstats` / `adjustment` 为叶子包，**禁止** import `service/chat`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；2026-07-23 拆分 chatstats/adjustment 后回写。*
