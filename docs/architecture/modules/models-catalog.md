# models-catalog 模块

> 本文件描述真实模型目录与模板匹配域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 真实模型目录、批量配置、自动关联用模板名 |
| 目录位置 | `handler/modelapi/`；`service/model_template.go`；`repository/model.go`、`model_template_item.go`；实体 `Model`、`ModelTemplateItem` |
| 主要职责 | 真实模型 CRUD/批量；模板项管理；模板索引匹配供自动关联使用 |

## 2. 职责与边界

- **负责什么**：真实模型生命周期；`ModelTemplateItem`；`TemplateIndex` 匹配逻辑
- **不负责什么**：`ModelWithProvider` 关联行 CRUD（`associations`）；上游列表同步（`model-sync`）；虚拟模型映射
- **对外暴露**：`handler/modelapi` REST；`ModelRepo` / `ModelTemplateItemRepo`；`TemplateIndex` / `BuildTemplateIndexFromData`
- **依赖谁**：`models`、`repository`（部分路径）；设置中的模糊匹配相关键

## 3. 内部结构

```
handler/modelapi/           # routes, CRUD, batch, template handlers
service/model_template.go   # TemplateIndex 构建与 Match
repository/model.go
repository/model_template_item.go
models.Model / ModelTemplateItem
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ModelRepo` | 真实模型持久化与批量更新 | `repository/model.go` | GORM |
| `ModelTemplateItemRepo` | 模板名条目 | `repository/model_template_item.go` | GORM |
| `TemplateIndex` | 上游模型名 → 真实模型模板匹配 | `service/model_template.go` | 同文件 |
| `models.Model` | 真实模型实体（IO 日志、thinking 能力、自动关联标记等；超时/重试已全局化，见 settings 模块「请求参数」） | `models/model.go` | GORM |

## 5. 特殊约定

- `handler/modelapi` 统一经 `repos()` 访问持久层（模型 CRUD、批量、模板项、级联清理），**禁止**新增 `gorm.G` 直连
- 自动关联消费 `TemplateIndex`，实现落在 `service/autoassoc`，本模块只提供匹配能力

### `Model.SupportsThinking`（thinking 能力标记）

- `bool`（**非** `*bool`）：Model 层是能力真实值的 single source of truth，无继承对象，`nil` 与 `false` 业务等价；三态语义只属于 `ModelWithProvider.SupportsThinking`（见 `associations` 文档）。
- 默认 `false`：**存量模型与 modelsync 同步创建的模型升级后默认不支持 thinking**，需手动在 model 编辑页勾选（含存量部署升级场景——升级后旧模型的 thinking 请求会被裁剪，属预期行为，有日志可观测）；自动推断（按模型名匹配已知支持 thinking 的模型）是后续独立任务。
- 消费方：`service/chat` 在构建上游请求时经 `ModelWithProvider.SupportsThinkingResolved` 解析最终状态，`false` 时裁剪请求体中的 thinking 配置字段。

### `Model.ThinkingLevels`（思考档位白名单）

- `[]string`（GORM `serializer:json`）：Model 层的思考档位白名单，空切片/nil=不约束（任意档位透传），非空=只允许白名单内档位。
- 8 档候选：6 档有序 `[minimal, low, medium, high, xhigh, max]` + 2 特殊 `[none, auto]`。
- `SupportsThinking=false` 时此字段被忽略（thinking 整体剥离，白名单无意义）。
- 消费方：`service/chat` 经 `ModelWithProvider.ThinkingLevelsResolved(model)` 解析最终白名单（关联 override 优先，否则继承 model），传给 `transform.ThinkingClampConfig` 在 chat 主路径钳制。
- 钳制规则：请求中的 `reasoning_effort` 不在白名单时，按就近原则钳制到白名单内最接近的档位；`auto` 不在白名单时回退到 `SettingKeyReasoningEffortDefaultValue`；`none` 不在白名单时剥离 thinking 字段。详见 `protocol-transform.md` 钳制章节。
- GORM map-based UpdateFields 不走 `serializer:json`，需手动序列化。`models.SerializeThinkingLevelsForUpdate([]string)` 用于 Model（非三态），`models.SerializeThinkingLevelsPtrForUpdate(*[]string)` 用于 ModelWithProvider（三态）。详见 `associations.md`。
- `models.IsSixLevelEffort(effort)` 校验 effort 是否为 6 档有序档位之一（不含 none/auto），用于 `buildThinkingClampConfig` 兜底校验 autoFallback 设置值。

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
