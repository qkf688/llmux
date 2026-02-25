# 扩展 UnifiedRequest 字段建议文档

## 1. 现有结构分析

### 1.1 当前 UnifiedRequest 结构

```go
// UnifiedRequest 统一请求格式
type UnifiedRequest struct {
    Model           string            `json:"model"`
    Messages        []UnifiedMessage  `json:"messages"`
    MaxTokens       int               `json:"max_tokens,omitempty"`
    Temperature     *float64          `json:"temperature,omitempty"`
    TopP            *float64          `json:"top_p,omitempty"`
    Stream          bool              `json:"stream,omitempty"`
    Tools           []UnifiedTool     `json:"tools,omitempty"`
    System          string            `json:"system,omitempty"`
    ReasoningEffort *string           `json:"reasoning_effort,omitempty"` // 推理强度参数
}
```

### 1.2 当前 UnifiedMessage 结构

```go
// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
    Role       string            `json:"role"`
    Content    interface{}       `json:"content,omitempty"`
    ToolCalls  []UnifiedToolCall `json:"tool_calls,omitempty"`
    ToolCallID string            `json:"tool_call_id,omitempty"` // OpenAI tool 角色消息的 tool_call_id
}
```

## 2. 参考 Octopus 的实现

### 2.1 Octopus 的 InternalLLMRequest 主要特性

Octopus 项目的 `InternalLLMRequest` 结构具有以下优势：

- **完整的参数支持**：包含 OpenAI API 的所有标准参数
- **多模态支持**：支持文本、图像、音频等多种输入类型
- **Embedding API 支持**：集成了 Embedding 接口的参数
- **推理功能**：支持各种推理相关参数
- **扩展性**：通过 `ExtraBody` 等字段支持自定义扩展

## 3. 建议的扩展结构

### 3.1 扩展后的 UnifiedRequest

```go
// UnifiedRequest 统一请求格式
type UnifiedRequest struct {
    // 现有字段
    Model           string            `json:"model"`
    Messages        []UnifiedMessage  `json:"messages,omitempty"`
    MaxTokens       int               `json:"max_tokens,omitempty"`
    Temperature     *float64          `json:"temperature,omitempty"`
    TopP            *float64          `json:"top_p,omitempty"`
    Stream          bool              `json:"stream,omitempty"`
    Tools           []UnifiedTool     `json:"tools,omitempty"`
    System          string            `json:"system,omitempty"`
    ReasoningEffort *string           `json:"reasoning_effort,omitempty"` // 推理强度参数

    // 新增字段：Embedding API 支持
    EmbeddingInput          *UnifiedEmbeddingInput `json:"embedding_input,omitempty"`
    EmbeddingDimensions     *int64                 `json:"embedding_dimensions,omitempty"`
    EmbeddingEncodingFormat *string                `json:"embedding_encoding_format,omitempty"`

    // 新增字段：高级参数
    FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
    PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
    Logprobs         *bool    `json:"logprobs,omitempty"`
    TopLogprobs      *int64   `json:"top_logprobs,omitempty"`
    Seed             *int64   `json:"seed,omitempty"`
    LogitBias        map[string]int64 `json:"logit_bias,omitempty"`
    Metadata         map[string]string `json:"metadata,omitempty"`

    // 新增字段：多模态支持
    Modalities []string `json:"modalities,omitempty"`
    Audio      *UnifiedAudio `json:"audio,omitempty"`

    // 新增字段：缓存和优化
    PromptCacheKey     *bool   `json:"prompt_cache_key,omitempty"`
    SafetyIdentifier   *string `json:"safety_identifier,omitempty"`
    User               *string `json:"user,omitempty"`

    // 新增字段：推理和服务
    ReasoningBudget *int64  `json:"reasoning_budget,omitempty"`
    EnableThinking  *bool   `json:"enable_thinking,omitempty"`
    ServiceTier     *string `json:"service_tier,omitempty"`
    Stop            *UnifiedStop `json:"stop,omitempty"`
    StreamOptions   *UnifiedStreamOptions `json:"stream_options,omitempty"`

    // 新增字段：响应格式
    ResponseFormat *UnifiedResponseFormat `json:"response_format,omitempty"`

    // 新增字段：扩展和工具
    ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
    ToolChoice        *UnifiedToolChoice `json:"tool_choice,omitempty"`

    // 帮助字段（不发送到 LLM 服务）
    ExtraBody            json.RawMessage `json:"extra_body,omitempty"`
    RawRequest           []byte          `json:"-"`
    RawAPIFormat         string          `json:"-"`
    TransformerMetadata  map[string]string `json:"-"`
    TransformOptions     UnifiedTransformOptions `json:"-"`
    Include              []string        `json:"-"`
}
```

### 3.2 扩展后的 UnifiedMessage

```go
// UnifiedMessage 统一消息格式
type UnifiedMessage struct {
    Role       string            `json:"role"`
    Content    UnifiedMessageContent `json:"content,omitempty"`
    ToolCalls  []UnifiedToolCall `json:"tool_calls,omitempty"`
    ToolCallID string            `json:"tool_call_id,omitempty"`
    Name       *string           `json:"name,omitempty"`
    Refusal    string            `json:"refusal,omitempty"`

    // 新增字段
    MessageIndex    *int    `json:"-"`
    ToolCallName    *string `json:"-"`
    ToolCallIsError *bool   `json:"-"`
    Audio           *UnifiedMessageAudio `json:"audio,omitempty"`
    ReasoningContent *string `json:"reasoning_content,omitempty"`
    Reasoning        *string `json:"reasoning,omitempty"`
    ReasoningSignature *string `json:"reasoning_signature,omitempty"`
    CacheControl      *UnifiedCacheControl `json:"-"`
}
```

### 3.3 新增的辅助结构

```go
// UnifiedEmbeddingInput Embedding 输入
type UnifiedEmbeddingInput struct {
    Single   *string   `json:"single,omitempty"`
    Multiple []string  `json:"multiple,omitempty"`
}

// UnifiedMessageContent 消息内容
type UnifiedMessageContent struct {
    Content         *string                  `json:"content,omitempty"`
    MultipleContent []UnifiedMessageContentPart `json:"multiple_content,omitempty"`
}

// UnifiedMessageContentPart 消息内容部分
type UnifiedMessageContentPart struct {
    Type       string                 `json:"type"`
    Text       *string                `json:"text,omitempty"`
    ImageURL   *UnifiedImageURL       `json:"image_url,omitempty"`
    Audio      *UnifiedAudio          `json:"input_audio,omitempty"`
    File       *UnifiedFile           `json:"file,omitempty"`
    CacheControl *UnifiedCacheControl `json:"-"`
}

// UnifiedImageURL 图像 URL
type UnifiedImageURL struct {
    URL     string  `json:"url"`
    Detail  *string `json:"detail,omitempty"`
}

// UnifiedAudio 音频内容
type UnifiedAudio struct {
    Format string `json:"format,omitempty"`
    Voice  string `json:"voice,omitempty"`
}

// UnifiedFile 文件内容
type UnifiedFile struct {
    Filename string `json:"filename"`
    FileData string `json:"file_data"`
}

// UnifiedCacheControl 缓存控制
type UnifiedCacheControl struct {
    Type string `json:"type,omitempty"`
}

// UnifiedStop 停止序列
type UnifiedStop struct {
    Stop         *string
    MultipleStop []string
}

// UnifiedStreamOptions 流式选项
type UnifiedStreamOptions struct {
    IncludeUsage bool `json:"include_usage,omitempty"`
}

// UnifiedResponseFormat 响应格式
type UnifiedResponseFormat struct {
    Type       string          `json:"type"`
    JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// UnifiedToolChoice 工具选择
type UnifiedToolChoice struct {
    StringValue *string
    ObjectValue *UnifiedToolChoiceObject
}

// UnifiedToolChoiceObject 工具选择对象
type UnifiedToolChoiceObject struct {
    Type     string                   `json:"type"`
    Function *UnifiedToolChoiceFunction `json:"function,omitempty"`
}

// UnifiedToolChoiceFunction 工具选择函数
type UnifiedToolChoiceFunction struct {
    Name string `json:"name"`
}

// UnifiedTransformOptions 转换选项
type UnifiedTransformOptions struct {
    ArrayInputs *bool `json:"-"`
}
```

## 4. 实现建议

### 4.1 分步实现策略

1. **第一阶段**：基础字段扩展
   - 添加常用参数（`FrequencyPenalty`、`PresencePenalty`、`Seed` 等）
   - 保持向后兼容

2. **第二阶段**：多模态支持
   - 实现 `Modalities`、`Audio` 等字段
   - 扩展消息内容结构

3. **第三阶段**：Embedding API 集成
   - 添加 Embedding 相关参数
   - 实现 Embedding 请求的处理逻辑

4. **第四阶段**：高级功能
   - 实现缓存控制、推理优化等高级功能

### 4.2 向后兼容性

- **使用指针类型**：所有新增字段使用指针类型，确保 `omitempty` 生效
- **默认值处理**：在转换逻辑中为缺失字段提供合理的默认值
- **渐进式迁移**：保持现有 API 不变，逐步添加新功能

### 4.3 转换逻辑调整

1. **请求转换**：
   - 在 `TransformOpenAIToUnified` 等函数中添加对新字段的处理
   - 确保所有新增参数都能正确映射

2. **响应转换**：
   - 更新响应转换逻辑以处理新的字段结构
   - 确保多模态响应能正确返回

3. **流式处理**：
   - 确保流式响应能正确处理新增字段
   - 保持流式处理的性能

## 5. 优势和收益

### 5.1 功能增强

- **完整的 API 覆盖**：支持 OpenAI API 的所有标准参数
- **多模态能力**：支持图像、音频等多种输入类型
- **Embedding 集成**：统一处理 Chat Completion 和 Embedding 请求
- **推理优化**：支持各种推理相关参数，提升模型性能

### 5.2 扩展性提升

- **模块化结构**：清晰的字段组织，便于维护和扩展
- **向前兼容**：为未来的 API 变更做好准备
- **灵活性**：通过 `ExtraBody` 等字段支持自定义扩展

### 5.3 性能优化

- **缓存支持**：通过 `PromptCacheKey` 等字段支持缓存优化
- **资源管理**：通过 `ReasoningBudget` 等字段控制资源使用
- **流式处理**：优化流式响应的处理逻辑

## 6. 测试建议

1. **单元测试**：为新的字段结构编写单元测试
2. **集成测试**：测试完整的请求/响应转换流程
3. **边界测试**：测试各种边缘情况和异常输入
4. **性能测试**：确保新字段不影响转换性能

## 7. 总结

通过参考 Octopus 项目的实现，扩展 `UnifiedRequest` 字段可以显著提升 llmio 项目的功能完整性和扩展性。建议采用渐进式的实现策略，确保向后兼容性的同时，逐步添加新的功能和字段。

这种扩展不仅能满足当前的 API 需求，还能为未来的功能增强和 API 变更做好准备，使 llmio 项目成为一个更加全面和强大的 LLM 代理服务。