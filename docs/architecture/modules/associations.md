# associations 模块

> 本文件描述模型↔供应商关联与自动关联域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 真实模型与供应商的多对多关联及自动关联 |
| 目录位置 | `handler/associations/`；`handler/autoassoc/`（HTTP 薄层）；`service/autoassoc/`（统一业务）；`repository/model_with_provider.go`；实体 `ModelWithProvider` |
| 主要职责 | 关联 CRUD、能力标志、状态、健康视图；自动按模板创建关联；清理无效关联 |

## 2. 职责与边界

- **负责什么**：`ModelWithProvider` 的权重、优先级、能力（tool/structured/image）、状态、连续失败等字段管理；自动按模板创建关联；清理无效关联
- **不负责什么**：上游连通性测试执行细节（`testapi`）；定时健康检查（`health-check`）；chat 运行时选路算法（消费本表数据）
- **对外暴露**：`handler/associations` REST；`handler/autoassoc` REST（委托 service）；`service/autoassoc.Service`；`ModelWithProviderRepo`；`NewDefaultAssociation`
- **依赖谁**：`repository`（Model / Provider / ModelWithProvider / ModelTemplateItem）；`service.BuildTemplateIndexFromData`（经门面注入）；`service/modelsync.GetProviderModels`（读 Config 模型列表）

## 3. 内部结构

```
handler/associations/     # CRUD、status、capabilities、health 视图
handler/autoassoc/        # HTTP 预览/执行 + Trigger* 薄封装
service/autoassoc/        # 统一 Associate / CleanInvalid 业务
  ├── service.go            # Service 聚合根 + 构造 + setting/Result/LogResult + 公共入口（Preview/Associate/Trigger*）
  ├── associate.go          # Associate 路径（associate / fetchAssociateData / forEachMissingAssociation / skipAutoAssociate）
  ├── clean.go              # CleanInvalid 路径（CleanInvalid / fetchCleanData / forEachInvalidAssociation）
  ├── shared.go             # 共享 helpers（key/index/blacklist/allows）
  ├── defaults.go           # Fallback 常量 + NewDefaultAssociation
  └── types.go              # Preview / associationCandidate / NameMatcher / BuildIndexFunc
service/autoassoc_facade.go  # GetAutoAssocService / NewAutoAssocService
repository/model_with_provider.go
models.ModelWithProvider
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ModelWithProviderRepo` | 关联持久化与按模型/供应商查询删除 | `repository/model_with_provider.go` | GORM |
| `models.ModelWithProvider` | 关联实体（路由与健康检查的核心行） | `models/model.go` | GORM |
| `autoassoc.Service` | 预览/执行自动关联与清理；带开关的 Trigger* | `service/autoassoc/` | 同包 |
| HTTP associate/clean | 绑定 JSON 与 `httpresp` | `handler/autoassoc/` | 调 `GetAutoAssocService` |

### `ModelWithProvider` 字段约定（路由相关）

- `MaxTokens *int`：发给上游前的输出上限裁剪阀值。`nil`/`0`/负数 = 不限，原样透传；`>0` 时超过此值会被裁剪到此上限。用于兜底客户端（如 Cursor）发超大值触发上游 400。裁剪在 `service/chat` 的 `buildRequestBodyForProvider` 中执行，passthrough 与 transform 两条路径均生效。**按协议键名穷举**：`max_tokens`（OpenAI Chat 旧名 / Anthropic）、`max_completion_tokens`（OpenAI Chat 新名）、`max_output_tokens`（OpenAI Responses）；同一 body 里出现多个则逐个钳制。漏掉某协议的键名 = 该协议上游的阀值静默空转且无报错，`max_output_tokens` 曾如此。
- `SupportsThinking *bool`：是否支持 thinking（推理）能力，**三态**——`nil`=继承 `Model.SupportsThinking`，`true`/`false`=override。解析语义见 `ModelWithProvider.SupportsThinkingResolved(model)`（纯函数：override 优先，其次继承 model，model 为 nil 时 false）。上传路径：DTO 用 `*bool` + `omitempty`；Create 直接赋值（nil 落库）；Update 在 `repos().RunInTx` 事务内执行两步——struct `Update`（其他字段部分更新，跳过零值）+ `UpdateFields` 显式写 `supports_thinking`（map 中的 nil 会写 NULL），保证原子且不破坏其余字段的部分更新语义。**注意契约**：关联 Update 中 `supports_thinking` **缺省（nil）= 显式清空 override（改回继承）**，与 Model Update 的"nil=不改"语义相反——因为该字段是三态且前端总是显式提交（inherit/true/false 之一）。
- `ThinkingLevels *[]string`：思考档位白名单，**三态**——`nil`=继承 `Model.ThinkingLevels`，`[]`=显式不约束（任意档位透传），非空=override 白名单。解析语义见 `ModelWithProvider.ThinkingLevelsResolved(model)`（纯函数：override 优先，其次继承 model，model 为 nil 时 nil）。GORM `serializer:json`。上传路径：DTO 用 `*[]string` + `omitempty`；Create 直接赋值（nil 落库）；Update 在同一 `RunInTx` 事务内与 `supports_thinking` 一起走 `UpdateFields` 显式写 NULL——**注意**：GORM `serializer:json` 只对 struct `Update` 生效，map-based `UpdateFields` 不走 serializer，需手动 `json.Marshal` 转 JSON 字符串（见 `models.SerializeThinkingLevelsPtrForUpdate` 共享辅助函数，Model 侧用 `SerializeThinkingLevelsForUpdate`）。**注意契约**：与 `supports_thinking` 一致，关联 Update 中 `thinking_levels` **缺省（nil）= 显式清空 override（改回继承）**。
- 裁剪联动：`SupportsThinkingResolved` 为 `false` 时，`service/chat` 的 `buildRequestBodyForProvider` 会删除请求体中的 `thinking`/`reasoning_effort`/`reasoning`/`output_config.effort` 字段（`stripThinkingFields` 纯函数，`output_config.effort` 删后空对象连壳删除），避免不支持 thinking 的上游报 400/静默忽略。
- 钳制联动：`SupportsThinkingResolved` 为 `true` 时，`service/chat` 的 `buildRequestBodyForProvider` 会经 `ThinkingLevelsResolved` 解析白名单，构建 `transform.ThinkingClampConfig` 并在 passthrough/transform 两条路径对 `reasoning_effort` 执行就近钳制 + budget 联动（方案 E）。详见 `protocol-transform.md` 钳制章节。

### 响应键契约（`/api/model-providers`）

`handler/associations` 的 Create / Update / List 直接序列化 `models.ModelWithProvider`，对外键名为 **PascalCase**（实体多数字段无 json tag），前端按同名 interface 消费（`webui/src/lib/api/modules/catalog/model-providers.ts`）。三态字段（`ToolCall` / `StructuredOutput` / `Image` / `WithHeader` / `Status` / `CustomerHeaders` / `MaxTokens` / `SupportsThinking` / `ThinkingLevels`）的 nil 必须序列化成 JSON `null` 且键保留——键缺失会让前端把「继承」误判成 override 并回写错误值。该契约由 `crud_response_shape_test.go` 的 `modelWithProviderResponseKeys` 表在响应体 JSON 上断言（gjson `Exists()` + 类型），实体增删字段时同步加/删一行即可。

## 5. 特殊约定

- chat 与 healthcheck 将关联行视为选路与启停的数据源；本模块是配置写入口
- **自动关联单一入口（已收敛）**：
  - 业务：`service/autoassoc`（`Associate` / `CleanInvalid` / `Preview*` / `Trigger*IfEnabled`）
  - HTTP：`handler/autoassoc` 只做绑定与响应
  - Provider CRUD：`TriggerAutoAssociate` / `TriggerAutoClean` → service
  - 同步后：`modelsync.ActionHooks` → `service.GetAutoAssocService().Associate/CleanInvalid`（开关在 `triggerAutoActions`）
  - 模板匹配：`service/model_template.go` 的 `TemplateIndex`（经 `autoassoc_facade` 注入 `BuildIndexFunc`）
- 规则权威：
  - 尊重 `Model.AutoAssociate`（`false` 跳过）
  - 跳过 `Provider.Blacklisted`
  - 默认 `Weight` 读 `SettingKeyAutoWeightDecayDefault`、`Priority` 读 `SettingKeyAutoPriorityDecayDefault`（schema 默认 100/100；缺失/非法回退到 `DefaultWeightFallback`/`DefaultPriorityFallback` 常量，由调用方 `service.associate` 传入 `NewDefaultAssociation`）
  - 无效关联：provider 不存在，或 `ProviderModel` 不在 Config 的 upstream/custom 列表；拉列表失败则不删

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日收敛 autoassoc 回写。*
