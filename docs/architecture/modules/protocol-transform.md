# protocol-transform 模块

> 本文件描述协议转换模块的职责、边界、对外暴露、依赖、接口契约。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 外部协议 ↔ 内部统一模型的双向转换（含流式） |
| 目录位置 | `service/transform/`；`service/anthropic/`；`service/responses/`；`service/transform_facade.go`；`models/unified/` |
| 主要职责 | OpenAI Chat / OpenAI Responses / Anthropic 与 `UnifiedRequest`/`UnifiedResponse` 互转 |

## 2. 职责与边界

- **负责什么**：请求/响应格式转换；流式 SSE 转换；适配器注册；实时流路由；统一模型校验/修复辅助
- **不负责什么**：供应商选择、HTTP 调用、重试、日志持久化、管理端 CRUD
- **对外暴露**：`FormatAdapter`、`TransformerManager`、包级 `Transform*ToUnified` / `TransformProviderResponse` 等（经 `transform_facade`）
- **依赖谁**：`models` / `models/unified`、`common`/`common/maputil`；内部依赖 `service/anthropic`、`service/responses`

## 3. 内部结构

```
service/
├── transform_facade.go
├── transform/              # 转换引擎与适配器
│   ├── adapters_register.go
│   ├── openai/ | anthropic/ | responses/ | streaming/ | shared/
├── anthropic/              # Anthropic 编解码细节
└── responses/              # Responses API 编解码细节
models/
├── unified.go              # 统一类型门面/别名
└── unified/                # UnifiedRequest/Response 等
```

各子目录职责：
- `transform/`：引擎入口、适配器注册、流式路由
- `anthropic/` / `responses/`：协议特有编解码
- `models/unified/`：canonical 中间表示

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `FormatAdapter` | 外部格式 ↔ 统一模型 | `service/transform/` | openai / anthropic / openai-res 适配器 |
| `TransformerManager` | 请求/响应转换编排 | `service/transform/` | `service/transform` |
| `RegisterAdapter` | 注册协议适配器 | `service/transform/` | `adapters_register.go` |
| `RegisterRealtimeRoute` | 注册实时流协议组合 | `service/transform/streaming/` | 各路由实现 |
| `UnifiedRequest` / `UnifiedResponse` | 协议中枢类型；`UnifiedMessage.RedactedThinkingData` 独立保存 Anthropic `redacted_thinking.data` 不透明密文，不与 reasoning 文本混用 | `models/unified/` 与 `models/unified.go`（响应等部分类型仍在门面文件） | 被转换器读写 |

## 5. 特殊约定

- 转换矩阵以统一模型为中枢，避免 N×M 直接互转
- 同格式路径可短路；流式与非流式分路径
- **扩展最小改动集（现状）**：新外部格式通常同时需要 `RegisterAdapter`、流式 `RegisterRealtimeRoute`（若涉及 SSE）、chat 侧 `Beforer`/`Processer`、`register_v1` 路由与 `consts.Style*`；Realtime 矩阵当前未覆盖全部协议组合，部分路径走 pivot/遗留逻辑
- **Responses `function_call_output.output` 多模态**：`ResponsesItem.Output` 为 `interface{}`——纯文本 tool result 输出 `string`（老上游兼容），含图片块输出 `input_text`/`input_image` 数组（OpenAI Responses 协议规范）。编解码 helper 在 `service/responses/tool_content_codec.go`，入站解析复用 `parsePartsToUnifiedContent`（纯文本→string、含图片→块数组，与 Anthropic 路径行为一致）
- 专题细节可参考历史图示（本地 `local/架构文档/格式转换架构图.md`，未入库）

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
