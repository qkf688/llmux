# 参数转换增强方案

## 概述

本文档描述了如何参考 [octopus 项目](E:\a-2025_12-projects\octopus) 来增强 llmio 的参数转换功能，支持更多高级参数和更完善的格式转换。

## 参考项目

**项目路径**: `E:\a-2025_12-projects\octopus`

**参考模块**:
- `internal/transformer/inbound/` - 输入格式转换
- `internal/transformer/outbound/` - 输出格式转换
- `internal/transformer/model/` - 统一模型定义

## 当前状态与目标

### 当前支持参数
- ✅ 基础参数: `model`, `messages`, `max_tokens`, `temperature`, `top_p`
- ✅ 流式支持: `stream`
- ✅ 工具调用: `tools`, `tool_calls`
- ✅ 系统提示: `system`
- ✅ 推理强度: `reasoning_effort`

### 需要增强的参数

#### 1. 缓存控制 (Cache Control)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\cache_control.go`

**功能描述**: 支持 Anthropic 的缓存控制功能，可以控制特定消息或工具的缓存行为。

**参数结构**:
```go
type CacheControl struct {
    Type string `json:"type"` // "ephemeral" or "ephemeral:cache_control"
}
```

**转换逻辑**:
- **Anthropic → 统一格式**: 将 `cache_control` 字段转换为统一格式
- **统一格式 → OpenAI**: OpenAI 不支持缓存控制，忽略该字段
- **统一格式 → Anthropic**: 保留缓存控制信息

**实现示例** (参考 octopus):
```go
// Anthropic 消息中的缓存控制
type Message struct {
    Role         string        `json:"role"`
    Content      MessageContent `json:"content"`
    CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// 工具定义中的缓存控制
type Tool struct {
    Name         string        `json:"name"`
    Description  string        `json:"description"`
    InputSchema  json.RawMessage `json:"input_schema"`
    CacheControl *CacheControl `json:"cache_control,omitempty"`
}
```

**实现优先级**: 高
**影响范围**: Anthropic 格式转换

---

#### 2. 思考配置 (Thinking Configuration)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\thinking.go`

**功能描述**: 支持 Anthropic 的 Extended Thinking 功能，控制模型的推理行为。

**参数结构**:
```go
type Thinking struct {
    Type         string `json:"type"`         // "enabled" or "disabled"
    BudgetTokens int64  `json:"budget_tokens"` // 推理预算 token 数
}
```

**转换逻辑**:
- **Anthropic → 统一格式**: 
  - `thinking.type = "enabled"` → 设置 `reasoning_effort` 和 `reasoning_budget`
  - `thinking.budget_tokens` → 映射到 `reasoning_budget`
- **统一格式 → OpenAI**: 
  - `reasoning_effort` → 转换为 OpenAI 的推理参数（如果支持）
- **统一格式 → Anthropic**: 
  - `reasoning_effort` 和 `reasoning_budget` → 转换为 `thinking` 配置

**实现示例** (参考 octopus):
```go
func thinkingBudgetToReasoningEffort(budgetTokens int64) string {
    switch {
    case budgetTokens >= 50000:
        return "high"
    case budgetTokens >= 20000:
        return "medium"
    case budgetTokens > 0:
        return "low"
    default:
        return ""
    }
}

func reasoningEffortToThinkingBudget(effort string) int64 {
    switch effort {
    case "high":
        return 50000
    case "medium":
        return 20000
    case "low":
        return 1000
    default:
        return 0
    }
}
```

**实现优先级**: 高
**影响范围**: Anthropic 格式转换，推理强度映射

---

#### 3. Stop 序列 (Stop Sequences)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\messages.go` (第 276-287 行)

**功能描述**: 支持单个或多个停止序列，控制模型生成何时停止。

**参数结构**:
```go
type Stop struct {
    Single   *string  `json:"stop,omitempty"`
    Multiple []string `json:"stop,omitempty"`
}
```

**转换逻辑**:
- **OpenAI → 统一格式**: 支持字符串或字符串数组
- **Anthropic → 统一格式**: `stop_sequences` 数组转换为统一格式
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 转换为 `stop_sequences` 数组

**实现示例** (参考 octopus):
```go
// OpenAI 格式支持单个 stop 字符串或数组
// Anthropic 格式只支持 stop_sequences 数组

// 转换: OpenAI → 统一格式
if len(req.Stop) == 1 {
    unified.Stop = &UnifiedStop{
        Single: &req.Stop[0],
    }
} else if len(req.Stop) > 1 {
    unified.Stop = &UnifiedStop{
        Multiple: req.Stop,
    }
}

// 转换: 统一格式 → Anthropic
if unified.Stop != nil {
    if unified.Stop.Single != nil {
        req.StopSequences = []string{*unified.Stop.Single}
    } else if len(unified.Stop.Multiple) > 0 {
        req.StopSequences = unified.Stop.Multiple
    }
}
```

**实现优先级**: 中
**影响范围**: 所有格式转换

---

#### 4. 响应格式控制 (Response Format)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 控制模型输出的格式，支持 JSON 模式和 JSON Schema。

**参数结构**:
```go
type ResponseFormat struct {
    Type       string          `json:"type"`        // "text", "json_object", "json_schema"
    JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 不支持响应格式控制，忽略
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 忽略该字段

**实现示例** (参考 octopus):
```go
// OpenAI 支持的响应格式类型
// - "text": 默认文本输出
// - "json_object": 强制输出 JSON
// - "json_schema": 使用 JSON Schema 验证输出

// 转换: OpenAI → 统一格式
if req.ResponseFormat != nil {
    unified.ResponseFormat = &UnifiedResponseFormat{
        Type:       req.ResponseFormat.Type,
        JSONSchema: req.ResponseFormat.JSONSchema,
    }
}
```

**实现优先级**: 中
**影响范围**: OpenAI 格式转换

---

#### 5. 工具选择控制 (Tool Choice)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 控制模型是否调用工具以及调用哪个工具。

**参数结构**:
```go
type ToolChoice struct {
    StringValue *string
    ObjectValue *ToolChoiceObject
}

type ToolChoiceObject struct {
    Type     string              `json:"type"`     // "function"
    Function *ToolChoiceFunction `json:"function,omitempty"`
}

type ToolChoiceFunction struct {
    Name string `json:"name"`
}
```

**转换逻辑**:
- **OpenAI → 统一格式**: 支持字符串（"none", "auto", "required"）或对象
- **Anthropic → 统一格式**: Anthropic 不支持工具选择控制，忽略
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 忽略该字段

**实现示例** (参考 octopus):
```go
// OpenAI 支持的工具选择类型
// - "none": 不调用工具
// - "auto": 自动决定是否调用工具
// - "required": 必须调用工具
// - {"type": "function", "function": {"name": "my_function"}}: 指定特定工具

// 转换: OpenAI → 统一格式
if req.ToolChoice != nil {
    if str, ok := req.ToolChoice.(string); ok {
        unified.ToolChoice = &UnifiedToolChoice{
            StringValue: &str,
        }
    } else if obj, ok := req.ToolChoice.(map[string]interface{}); ok {
        // 解析对象格式
        toolChoiceObj := parseToolChoiceObject(obj)
        unified.ToolChoice = &UnifiedToolChoice{
            ObjectValue: toolChoiceObj,
        }
    }
}
```

**实现优先级**: 中
**影响范围**: OpenAI 格式转换

---

#### 6. 并行工具调用 (Parallel Tool Calls)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 控制是否允许模型并行调用多个工具。

**参数结构**:
```go
ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 原生支持并行工具调用，忽略该字段
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 忽略该字段

**实现示例**:
```go
// 转换: OpenAI → 统一格式
if req.ParallelToolCalls != nil {
    unified.ParallelToolCalls = req.ParallelToolCalls
}
```

**实现优先级**: 低
**影响范围**: OpenAI 格式转换

---

#### 7. 流式选项 (Stream Options)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 控制流式响应的详细选项，如是否包含使用统计。

**参数结构**:
```go
type StreamOptions struct {
    IncludeUsage bool `json:"include_usage,omitempty"`
}
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 流式响应自动包含使用统计，忽略该字段
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 忽略该字段

**实现示例**:
```go
// 转换: OpenAI → 统一格式
if req.StreamOptions != nil {
    unified.StreamOptions = &UnifiedStreamOptions{
        IncludeUsage: req.StreamOptions.IncludeUsage,
    }
}
```

**实现优先级**: 低
**影响范围**: OpenAI 格式转换，流式响应

---

#### 8. 多模态输出 (Modalities)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 指定模型生成的输出类型，支持文本、音频、图像。

**参数结构**:
```go
Modalities []string `json:"modalities,omitempty"` // ["text"], ["text", "audio"], ["text", "audio", "image"]
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 主要支持文本，忽略音频和图像选项
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 仅保留 "text" 模态

**实现示例**:
```go
// 转换: OpenAI → 统一格式
if len(req.Modalities) > 0 {
    unified.Modalities = req.Modalities
}

// 转换: 统一格式 → Anthropic
if len(unified.Modalities) > 0 {
    // Anthropic 只支持文本，过滤其他模态
    if contains(unified.Modalities, "text") {
        // 保留文本模态
    } else {
        // 如果没有文本模态，添加默认文本模态
    }
}
```

**实现优先级**: 低
**影响范围**: 多模态功能

---

#### 9. 音频输出配置 (Audio Output)

**参考位置**: `E:\a-2025_11-projects-git\llmio\service\transformer.go` (已定义)

**功能描述**: 配置音频输出的参数，如语音类型和格式。

**参数结构**:
```go
type Audio struct {
    Voice  string `json:"voice,omitempty"`  // "alloy", "echo", "fable", "onyx", "nova", "shimmer"
    Format string `json:"format,omitempty"` // "wav", "mp3", "pcm16"
}
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 不支持音频输出，忽略
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 忽略该字段

**实现优先级**: 低
**影响范围**: 音频功能

---

#### 10. 高级参数 (Advanced Parameters)

**参考位置**: `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**功能描述**: 支持更多高级参数，如频率惩罚、存在惩罚、种子、logit 偏置等。

**参数结构**:
```go
// 频率惩罚 (-2.0 到 2.0)
FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

// 存在惩罚 (-2.0 到 2.0)
PresencePenalty *float64 `json:"presence_penalty,omitempty"`

// 随机种子
Seed *int64 `json:"seed,omitempty"`

// Token 偏置
LogitBias map[string]int64 `json:"logit_bias,omitempty"`

// 用户标识
User *string `json:"user,omitempty"`

// 元数据
Metadata map[string]string `json:"metadata,omitempty"`

// Log 概率
Logprobs *bool `json:"logprobs,omitempty"`

// Top Log 概率数量
TopLogprobs *int64 `json:"top_logprobs,omitempty"`

// 最大完成 token 数
MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`

// 存储输出
Store *bool `json:"store,omitempty"`
```

**转换逻辑**:
- **OpenAI → 统一格式**: 直接映射
- **Anthropic → 统一格式**: Anthropic 支持部分参数（如 `top_k`），其他忽略
- **统一格式 → OpenAI**: 保持原格式
- **统一格式 → Anthropic**: 转换支持的参数，忽略不支持的参数

**实现示例** (参考 octopus):
```go
// 转换: OpenAI → 统一格式
if req.FrequencyPenalty != nil {
    unified.FrequencyPenalty = req.FrequencyPenalty
}
if req.PresencePenalty != nil {
    unified.PresencePenalty = req.PresencePenalty
}
if req.Seed != nil {
    unified.Seed = req.Seed
}
if req.LogitBias != nil {
    unified.LogitBias = req.LogitBias
}
if req.User != nil {
    unified.User = req.User
}
if req.Metadata != nil {
    unified.Metadata = req.Metadata
}

// 转换: 统一格式 → Anthropic
// Anthropic 支持 top_k，但不支持 frequency_penalty 等
if unified.TopP != nil {
    // 可以考虑将 top_p 转换为 top_k
}
```

**实现优先级**: 中
**影响范围**: 所有格式转换

---

## 实现计划

### 阶段 1: 高优先级参数 (1-2 周)

1. **缓存控制** (Cache Control)
   - 实现 `CacheControl` 类型定义
   - 添加 Anthropic → 统一格式转换
   - 添加统一格式 → Anthropic 转换
   - 测试缓存控制功能

2. **思考配置** (Thinking Configuration)
   - 实现 `Thinking` 类型定义
   - 实现 `reasoning_effort` 和 `reasoning_budget` 映射
   - 添加 Anthropic ↔ 统一格式转换
   - 测试推理强度映射

### 阶段 2: 中优先级参数 (2-3 周)

3. **Stop 序列** (Stop Sequences)
   - 完善 `UnifiedStop` 类型
   - 添加 OpenAI ↔ 统一格式转换
   - 添加 Anthropic ↔ 统一格式转换
   - 测试单个和多个停止序列

4. **响应格式控制** (Response Format)
   - 完善 `UnifiedResponseFormat` 类型
   - 添加 OpenAI ↔ 统一格式转换
   - 测试 JSON 模式和 JSON Schema

5. **工具选择控制** (Tool Choice)
   - 完善 `UnifiedToolChoice` 类型
   - 添加 OpenAI ↔ 统一格式转换
   - 测试各种工具选择模式

6. **高级参数** (Advanced Parameters)
   - 添加所有高级参数到 `UnifiedRequest`
   - 实现 OpenAI ↔ 统一格式转换
   - 实现部分参数的 Anthropic 转换
   - 测试高级参数功能

### 阶段 3: 低优先级参数 (1-2 周)

7. **并行工具调用** (Parallel Tool Calls)
   - 添加参数支持
   - 实现 OpenAI ↔ 统一格式转换
   - 测试并行工具调用

8. **流式选项** (Stream Options)
   - 添加参数支持
   - 实现流式响应的使用统计
   - 测试流式选项

9. **多模态输出** (Modalities)
   - 添加参数支持
   - 实现模态过滤和转换
   - 测试多模态输出

10. **音频输出配置** (Audio Output)
    - 完善现有实现
    - 添加更多音频格式支持
    - 测试音频输出

---

## 架构优化建议

### 1. 模块化转换器

参考 octopus 的架构，将转换器重构为模块化结构：

```
service/transformer/
├── model/
│   ├── interface.go       # 统一类型定义
│   └── types.go           # 具体类型实现
├── inbound/
│   ├── openai/
│   │   ├── chat.go        # OpenAI 聊天转换
│   │   └── embedding.go   # OpenAI 嵌入转换
│   ├── anthropic/
│   │   ├── messages.go    # Anthropic 消息转换
│   │   ├── cache_control.go
│   │   └── thinking.go
│   └── register.go        # 转换器注册
└── outbound/
    ├── openai/
    │   ├── chat.go
    │   └── embedding.go
    ├── anthropic/
    │   └── messages.go
    └── register.go
```

### 2. 转换器接口

定义统一的转换器接口：

```go
type Transformer interface {
    // TransformRequest 将客户端请求转换为统一格式
    TransformRequest(ctx context.Context, body []byte) (*model.InternalLLMRequest, error)
    
    // TransformResponse 将统一格式响应转换为客户端格式
    TransformResponse(ctx context.Context, response *model.InternalLLMResponse) ([]byte, error)
    
    // TransformStream 处理流式响应
    TransformStream(ctx context.Context, stream *model.InternalLLMResponse) ([]byte, error)
    
    // GetInternalResponse 获取完整的内部响应（用于日志和统计）
    GetInternalResponse(ctx context.Context) (*model.InternalLLMResponse, error)
}
```

### 3. 参数验证和修复

参考 octopus 的参数处理逻辑，添加参数验证和自动修复：

```go
// 验证参数范围
func validateTemperature(temp *float64) error {
    if temp != nil && (*temp < 0 || *temp > 2) {
        return errors.New("temperature must be between 0 and 2")
    }
    return nil
}

// 修复无效参数
func repairInvalidJSON(jsonStr string) string {
    if json.Valid([]byte(jsonStr)) {
        return jsonStr
    }
    return "{}"
}
```

---

## 测试策略

### 单元测试

为每个转换函数编写单元测试：

```go
func TestCacheControlConversion(t *testing.T) {
    tests := []struct {
        name     string
        input    CacheControl
        expected *UnifiedCacheControl
    }{
        {
            name:  "ephemeral cache control",
            input: CacheControl{Type: "ephemeral"},
            expected: &UnifiedCacheControl{
                Type: "ephemeral",
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := convertCacheControl(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 集成测试

测试完整的转换流程：

```go
func TestOpenAIToAnthropicConversion(t *testing.T) {
    // 1. 准备 OpenAI 请求
    openaiReq := OpenAIRequest{
        Model: "gpt-4",
        Messages: []OpenAIMessage{
            {Role: "user", Content: "Hello"},
        },
        Temperature: &[]float64{0.7}[0],
    }
    
    // 2. 转换为统一格式
    unified, err := TransformOpenAIToUnified(openaiReq)
    assert.NoError(t, err)
    
    // 3. 转换为 Anthropic 格式
    anthropicReq, err := TransformUnifiedToAnthropic(unified)
    assert.NoError(t, err)
    
    // 4. 验证转换结果
    assert.Equal(t, "gpt-4", anthropicReq.Model)
    assert.Equal(t, 0.7, *anthropicReq.Temperature)
}
```

### 边界测试

测试边界情况和异常输入：

```go
func TestEdgeCases(t *testing.T) {
    // 测试空请求
    // 测试无效参数
    // 测试极大/极小值
    // 测试特殊字符
    // 测试并发转换
}
```

---

## 文档更新

实现完成后，需要更新以下文档：

1. **[FORMAT_CONVERSION.md](FORMAT_CONVERSION.md)**: 添加新支持的参数说明
2. **[UNIFIED_REQUEST_EXTENSION.md](UNIFIED_REQUEST_EXTENSION.md)**: 更新统一请求格式定义
3. **API 文档**: 添加新参数的 API 说明
4. **示例代码**: 提供使用新参数的示例

---

## 性能考虑

1. **缓存**: 对频繁使用的转换结果进行缓存
2. **并行处理**: 对独立的转换操作使用并发处理
3. **内存优化**: 避免不必要的数据复制
4. **延迟加载**: 按需加载转换器

---

## 兼容性

### 向后兼容

- 保持现有 API 不变
- 新参数使用可选字段
- 提供默认值处理

### 跨供应商兼容

- 明确标注每个参数的供应商支持情况
- 对不支持的参数提供优雅降级
- 记录参数转换的注意事项

---

## 监控和日志

### 转换监控

- 记录转换成功率
- 监控转换耗时
- 统计参数使用情况

### 日志记录

- 记录转换失败的详细信息
- 记录参数转换的映射关系
- 提供调试级别的详细日志

---

## 参考资源

- [OpenAI API 文档](https://platform.openai.com/docs/api-reference)
- [Anthropic API 文档](https://docs.anthropic.com/claude/reference)
- [octopus 项目](E:\a-2025_12-projects\octopus)
- [llmio 现有转换代码](../service/transformer.go)

---

## 贡献指南

如果您想参与参数转换功能的增强，请：

1. 选择一个待实现的参数
2. 创建功能分支
3. 实现转换逻辑
4. 编写测试用例
5. 更新相关文档
6. 提交 Pull Request

---

## 变更日志

### 2026-02-25
- 创建参数转换增强方案文档
- 参考 octopus 项目分析
- 定义实现计划和优先级
