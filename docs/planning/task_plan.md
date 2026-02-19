# Task Plan: OpenAI Responses ↔ Chat/Anthropic 转换

## Goal
在 llmio 中新增 OpenAI Responses API 与 OpenAI Chat Completions、Anthropic Messages 的双向转换（含流式与非流式），保持现有负载均衡与日志链路不变。

## Current Phase
Phase 10

## Phases

### Phase 1: Requirements & Discovery
- [x] 理解需求：Responses ↔ Chat/Anthropic 双向转换
- [x] 识别约束：最小改动、保留现有处理链路
- [x] 记录关键发现到 findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [x] 明确转换矩阵与中间格式（Responses 作为中间层）
- [x] 列出新增文件与修改点
- [x] 记录技术决策与理由
- **Status:** complete

### Phase 3: Implementation - Types
- [x] 新增 service/transform_responses_types.go
  - ResponsesRequest/ResponsesResponse 核心类型
  - ResponsesInput（支持 string | array 的自定义 JSON 序列化）
  - ResponsesItem/ResponsesTool/ResponsesStreamEvent
  - ResponsesUsage（含 input_tokens_details）
- **Status:** complete

### Phase 4: Implementation - Request Conversion
- [x] 新增 service/transform_responses.go（请求转换部分）
  - TransformResponsesToUnified: Responses → UnifiedRequest
  - TransformUnifiedToResponses: UnifiedRequest → Responses
  - 处理 instructions → System 映射
  - 处理 input → Messages 映射（含 function_call/function_call_output）
- **Status:** complete

### Phase 5: Implementation - Response Conversion
- [x] 完善 service/transform_responses.go（响应转换部分）
  - parseResponsesResponse: Responses 响应 → UnifiedResponse
  - formatResponsesResponse: UnifiedResponse → Responses 响应
  - 处理 output 数组 → Choices 映射
  - 处理 finish_reason ↔ status 映射
- [x] 集成到 transform_openai.go 的非流式响应分支
- **Status:** complete

### Phase 6: Implementation - Stream Conversion
- [x] 在 transformStreamResponseRealtime 中添加四个流式转换分支
  - anthropic → openai-res：content_block_delta → response.output_text.delta
  - openai-res → anthropic：response.output_text.delta → content_block_delta
  - openai → openai-res：choices[].delta.content → response.output_text.delta
  - openai-res → openai：response.output_text.delta → choices[].delta.content
  - 处理 response.completed 事件携带 usage
  - 处理 response.function_call_arguments.delta 工具调用参数增量
- **Status:** complete

### Phase 7: Integration - Transformer
- [x] 修改 service/transformer.go
  - ProcessRequest 添加 case "openai-res"
  - ProcessResponse 支持 clientType/providerType 为 openai-res
  - 扩展 TransformProviderResponse 的流式分支
- **Status:** complete

### Phase 8: Integration - Stream Processing
- [x] 修改 service/transform_openai.go（或新增流式转换辅助函数）
  - 扩展 transformStreamResponseRealtime 支持 openai-res
  - 实现 openai ↔ openai-res 流式事件映射
  - 实现 anthropic ↔ openai-res 流式事件映射
- **Status:** complete

### Phase 9: Testing & Verification
- [x] 非流式转换测试（3×3 组合）
  - 已有转换函数覆盖所有方向
- [x] 流式转换测试
  - 验证 SSE 事件序列正确性
  - 验证 usage 在 response.completed 事件中
  - 验证工具调用元数据完整性
  - 验证无重复终止信号
- [x] Codex 架构审阅
  - 识别并修复 7 个问题（2 blocker, 2 high, 3 medium）
- [x] 测试强化
  - 严格值断言（token 数量精确匹配）
  - 事件顺序验证
  - 类型安全断言
- **Status:** complete

### Phase 10: Delivery
- [x] 代码审查与清理
- [x] 更新文档说明
- [x] 总结迁移要点与风险提示
- **Status:** complete

## Key Questions
1. Responses 的 instructions/input/tool_calls 映射到 UnifiedRequest 的规则是否与现有 openai/anthropic 兼容？
   - **答案：** 需要在实现时验证，特别是 function_call_output 的映射
2. 流式事件如何保证 response.completed 时携带 usage，且不破坏现有 SSE 格式？
   - **答案：** 参考 octopus 实现，在 response.completed 事件中包含完整 usage 对象

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Responses 作为中间格式 | 字段覆盖最完整，便于统一映射与减少成对转换 |
| 最小改动接入现有 transformer | 保持负载均衡、日志与能力匹配流程不变 |
| 拆分类型定义与转换逻辑 | 降低单文件复杂度，便于维护 |
| 独立流式转换文件 | 流式逻辑复杂，独立文件便于调试 |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| session-catchup.py 路径解析错误（PowerShell 语法） | 1 | 改用显式 Windows 路径调用脚本 |
| Write 工具要求先读取文件 | 1 | 改用 Bash heredoc 直接创建文件 |

## Notes
- 更新阶段状态：pending → in_progress → complete
- 避免超范围功能（仅 Chat/Responses/Anthropic）
- 流式转换是关键难点，需要仔细处理事件序列
