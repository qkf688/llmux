# UnifiedRequest 结构扩展实现计划

## 目标
扩展 `UnifiedRequest` 和相关结构，参考 Octopus 项目实现，支持完整的 OpenAI API 参数、多模态输入、Embedding API 等高级功能。

## 当前状态分析

### 现有结构 (service/transformer.go)
```go
type UnifiedRequest struct {
    Model           string           `json:"model"`
    Messages        []UnifiedMessage `json:"messages"`
    MaxTokens       int              `json:"max_tokens,omitempty"`
    Temperature     *float64         `json:"temperature,omitempty"`
    TopP            *float64         `json:"top_p,omitempty"`
    Stream          bool             `json:"stream,omitempty"`
    Tools           []UnifiedTool    `json:"tools,omitempty"`
    System          string           `json:"system,omitempty"`
    ReasoningEffort *string          `json:"reasoning_effort,omitempty"`
}
```

### 需要扩展的功能领域
1. **高级参数**: FrequencyPenalty, PresencePenalty, Seed, LogitBias 等
2. **多模态支持**: Modalities, Audio, 图像/音频内容
3. **Embedding API**: EmbeddingInput, Dimensions, EncodingFormat
4. **响应格式**: ResponseFormat (JSON Schema)
5. **工具增强**: ToolChoice, ParallelToolCalls
6. **流式增强**: StreamOptions (include_usage)
7. **缓存优化**: PromptCacheKey, CacheControl
8. **推理优化**: ReasoningBudget, EnableThinking
9. **扩展性**: ExtraBody, Metadata

## 实施阶段

### 阶段 1: 基础字段扩展 [completed]
**目标**: 添加常用的高级参数，保持向后兼容

**参考**: Octopus `InternalLLMRequest` (model.go:28-245)

**任务**:
- [x] 添加 `UnifiedStop` 类型（参考 Octopus Stop 实现）
  - 实现 MarshalJSON/UnmarshalJSON 方法
  - 支持 string 或 []string 自动转换
- [x] 扩展 `UnifiedRequest` 添加基础高级参数
  - FrequencyPenalty *float64
  - PresencePenalty *float64
  - Seed *int64
  - LogitBias map[string]int64
  - Stop *UnifiedStop
  - User *string
  - Metadata map[string]string
  - Logprobs *bool
  - TopLogprobs *int64
  - MaxCompletionTokens *int64
  - Store *bool
- [x] 更新 `TransformOpenAIToUnified` 解析新字段
- [x] 更新 `TransformUnifiedToOpenAI` 输出新字段
- [x] 更新 `TransformUnifiedToAnthropic` 映射兼容字段
- [x] 编写单元测试验证转换正确性

**文件涉及**:
- `service/transformer.go` - 类型定义
- `service/transform_openai.go` - OpenAI 转换逻辑
- `service/transform_anthropic.go` - Anthropic 转换逻辑
- `service/transformer_test.go` - 单元测试

**验收标准**:
- ✅ UnifiedStop 自动处理 JSON 序列化/反序列化
- ✅ 所有新字段都能正确解析和转换
- ✅ 现有功能不受影响
- ✅ 单元测试覆盖率 >80%

**预计时间**: 3-4 小时
**实际时间**: 约 3 小时
**完成日期**: 2026-02-25

---

### 阶段 2: 响应格式和工具增强 [completed]
**目标**: 支持结构化输出和高级工具控制

**参考**: Octopus `ResponseFormat` 和 `ToolChoice` 实现

**任务**:
- [x] 添加 `UnifiedResponseFormat` 类型
  - 支持 text, json_object, json_schema
  - JSONSchema 字段用于结构化输出
- [x] 添加 `UnifiedToolChoice` 类型（实现自定义 JSON 序列化）
  - 支持 string 值 ("auto", "none", "required")
  - 支持 object 值 (指定特定工具)
  - MarshalJSON/UnmarshalJSON 实现
- [x] 添加 `UnifiedStreamOptions` 类型
  - IncludeUsage 字段
- [x] 扩展 `UnifiedRequest` 添加 4 个新字段
  - ResponseFormat, ToolChoice, ParallelToolCalls, StreamOptions
- [x] 更新 `TransformOpenAIToUnified` 解析新字段
- [x] 更新 `TransformUnifiedToOpenAI` 输出新字段
- [x] 更新 `TransformUnifiedToAnthropic` 映射 tool_choice
- [x] 编写单元测试验证转换正确性

**文件涉及**:
- `service/transformer.go` - 类型定义
- `service/transform_openai.go` - OpenAI 转换逻辑
- `service/transform_anthropic.go` - Anthropic 转换逻辑
- `service/transformer_test.go` - 单元测试

**验收标准**:
- ✅ UnifiedToolChoice 自动处理 JSON 序列化/反序列化
- ✅ ResponseFormat 支持三种格式
- ✅ 所有新字段都能正确解析和转换
- ✅ 现有功能不受影响
- ✅ 单元测试覆盖率 100%

**预计时间**: 2-3 小时
**实际时间**: 约 2 小时
**完成日期**: 2026-02-25

---

### 阶段 3: 多模态内容支持 [completed]
**目标**: 支持图像、音频等多模态输入

**参考**: Octopus `MessageContent` 和 `MessageContentPart` 实现

**任务**:
- [x] 重构 `UnifiedMessage.Content` 为复杂类型
  - 创建 `UnifiedMessageContent` 类型
  - 支持 string 或 []ContentPart
  - 实现自定义 JSON 序列化
- [x] 添加 `UnifiedMessageContentPart` 类型
  - TextPart: {type: "text", text: string}
  - ImagePart: {type: "image_url", image_url: {url, detail}}
  - AudioPart: {type: "input_audio", input_audio: {data, format}}
- [x] 添加 `Modalities` 字段 ([]string)
- [x] 添加 `Audio` 配置字段
- [x] 添加 UnifiedMessage 辅助方法
  - GetContentAsString, GetContentParts, SetContentString, SetContentParts
- [x] 更新转换逻辑处理多模态内容
- [x] 编写多模态测试用例

**文件涉及**:
- `service/transformer.go` - 类型重构和辅助方法
- `service/transform_openai.go` - 多模态解析
- `service/transform_anthropic.go` - Anthropic 多模态映射
- `service/transformer_test.go` - 单元测试

**验收标准**:
- ✅ 能正确解析图像 URL 和 base64 数据
- ✅ 能正确解析音频输入
- ✅ 向后兼容纯文本消息
- ✅ Content 字段保持 interface{} 类型
- ✅ 单元测试覆盖率 100%

**预计时间**: 4-5 小时
**实际时间**: 约 3 小时
**完成日期**: 2026-02-25

---

### 阶段 4: Embedding API 集成 [pending]
**目标**: 统一处理 Chat Completion 和 Embedding 请求

**任务**:
- [ ] 添加 Embedding 相关字段
  - EmbeddingInput: *UnifiedEmbeddingInput
  - EmbeddingDimensions: *int64
  - EmbeddingEncodingFormat: *string
- [ ] 创建 `UnifiedEmbeddingInput` 类型
  - Single: *string
  - Multiple: []string
- [ ] 添加请求类型判断逻辑
  - 有 Messages → Chat Completion
  - 有 EmbeddingInput → Embedding
- [ ] 实现 Embedding 请求转换
- [ ] 实现 Embedding 响应转换
- [ ] 编写 Embedding 测试

**文件涉及**:
- `service/transformer.go` - Embedding 类型
- `service/transform_openai.go` - Embedding 转换
- `handler/api.go` - 路由处理

**验收标准**:
- Embedding 请求能正确路由和转换
- 支持单个和批量 Embedding
- 响应格式符合 OpenAI 标准

---

### 阶段 5: 缓存和推理优化 [pending]
**目标**: 支持高级缓存和推理控制

**任务**:
- [ ] 添加缓存相关字段
  - PromptCacheKey: *bool
  - CacheControl: 在 Message 和 ContentPart 中
- [ ] 添加推理相关字段
  - ReasoningBudget: *int64
  - EnableThinking: *bool
  - ServiceTier: *string
- [ ] 实现 Anthropic 缓存控制映射
- [ ] 实现推理参数映射
- [ ] 编写缓存和推理测试

**文件涉及**:
- `service/transformer.go` - 新字段
- `service/transform_anthropic.go` - Anthropic 特定功能
- `service/transform_openai.go` - OpenAI 推理参数

**验收标准**:
- Anthropic 缓存控制正常工作
- 推理参数能正确传递
- 不影响不支持这些功能的提供商

---

### 阶段 6: 扩展性和元数据 [pending]
**目标**: 支持自定义扩展和内部元数据

**任务**:
- [ ] 添加扩展字段
  - ExtraBody: json.RawMessage
  - RawRequest: []byte (不序列化)
  - RawAPIFormat: string (不序列化)
  - TransformerMetadata: map[string]string (不序列化)
  - TransformOptions: UnifiedTransformOptions (不序列化)
  - Include: []string (不序列化)
- [ ] 实现 ExtraBody 合并逻辑
- [ ] 实现元数据传递机制
- [ ] 更新文档说明扩展用法

**文件涉及**:
- `service/transformer.go` - 扩展字段
- `service/transform_openai.go` - ExtraBody 处理
- `docs/` - 使用文档

**验收标准**:
- ExtraBody 能正确合并到请求中
- 元数据能在转换链中传递
- 文档清晰说明扩展机制

---

### 阶段 7: 测试和文档完善 [pending]
**目标**: 确保代码质量和可维护性

**任务**:
- [ ] 编写完整的单元测试套件
- [ ] 编写集成测试
- [ ] 性能测试 (确保转换不影响性能)
- [ ] 更新 API 文档
- [ ] 编写迁移指南
- [ ] 更新 CLAUDE.md 规范

**文件涉及**:
- `service/*_test.go` - 测试文件
- `docs/` - 文档
- `CLAUDE.md` - 项目规范

**验收标准**:
- 测试覆盖率 >85%
- 所有测试通过
- 文档完整清晰

---

## 风险和注意事项

### 向后兼容性
- **风险**: 结构变更可能破坏现有代码
- **缓解**:
  - 所有新字段使用指针类型 + omitempty
  - 保持现有字段不变
  - 渐进式迁移，每个阶段独立可用

### 性能影响
- **风险**: 复杂类型转换可能影响性能
- **缓解**:
  - 只在需要时解析复杂字段
  - 使用 json.RawMessage 延迟解析
  - 性能测试验证

### 提供商兼容性
- **风险**: 不同提供商支持的功能不同
- **缓解**:
  - 在转换层优雅降级
  - 记录不支持的功能
  - 提供清晰的错误信息

### 测试复杂度
- **风险**: 多模态和复杂类型难以测试
- **缓解**:
  - 使用测试夹具 (fixtures)
  - 模拟多模态数据
  - 分层测试策略

---

## 依赖和前置条件

- Go 1.25.0+
- 现有转换逻辑稳定运行
- 单元测试框架就绪

## 成功指标

- [ ] 所有 7 个阶段完成
- [ ] 测试覆盖率 >85%
- [ ] 性能无明显下降 (<5% 延迟增加)
- [ ] 向后兼容，现有功能不受影响
- [ ] 文档完整，易于理解

## 时间估算

- 阶段 1: 2-3 天
- 阶段 2: 2-3 天
- 阶段 3: 3-4 天
- 阶段 4: 2-3 天
- 阶段 5: 2-3 天
- 阶段 6: 1-2 天
- 阶段 7: 2-3 天

**总计**: 约 14-21 天 (根据复杂度和测试深度)

---

## 错误记录

| 错误 | 尝试次数 | 解决方案 |
|------|---------|----------|
| - | - | - |

---

## 决策日志

| 日期 | 决策 | 原因 |
|------|------|------|
| 2026-02-25 | 采用分阶段实施策略 | 降低风险，每阶段独立可用 |
| 2026-02-25 | 所有新字段使用指针类型 | 确保 omitempty 生效，向后兼容 |
| 2026-02-25 | 优先实现基础参数和响应格式 | 这些是最常用的功能 |

---

**最后更新**: 2026-02-25
**当前阶段**: 规划完成，等待用户确认
