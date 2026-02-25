# UnifiedRequest 扩展实现 - 研究发现

## 文档分析

### UNIFIED_REQUEST_EXTENSION.md 核心内容
- **来源**: 参考 Octopus 项目的 `InternalLLMRequest` 实现
- **目标**: 将 LLMIO 的 `UnifiedRequest` 扩展为功能完整的统一请求格式
- **优势**: 完整参数支持、多模态、Embedding API、推理功能、高扩展性

### 建议的扩展领域
1. **Embedding API 支持** (新增 3 个字段)
2. **高级参数** (新增 7 个字段: FrequencyPenalty, PresencePenalty, Logprobs, TopLogprobs, Seed, LogitBias, Metadata)
3. **多模态支持** (新增 2 个字段: Modalities, Audio)
4. **缓存和优化** (新增 3 个字段: PromptCacheKey, SafetyIdentifier, User)
5. **推理和服务** (新增 4 个字段: ReasoningBudget, EnableThinking, ServiceTier, Stop, StreamOptions)
6. **响应格式** (新增 1 个字段: ResponseFormat)
7. **扩展和工具** (新增 2 个字段: ParallelToolCalls, ToolChoice)
8. **帮助字段** (新增 6 个字段: ExtraBody, RawRequest, RawAPIFormat, TransformerMetadata, TransformOptions, Include)

---

## 代码库现状分析

### 当前 UnifiedRequest 结构 (service/transformer.go:44-55)
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

**特点**:
- 已有 9 个字段
- 已支持 `ReasoningEffort` (最近添加)
- 使用指针类型实现可选参数 (Temperature, TopP, ReasoningEffort)

### 当前 UnifiedMessage 结构 (service/transformer.go:10-16)
```go
type UnifiedMessage struct {
    Role       string            `json:"role"`
    Content    interface{}       `json:"content,omitempty"`
    ToolCalls  []UnifiedToolCall `json:"tool_calls,omitempty"`
    ToolCallID string            `json:"tool_call_id,omitempty"`
}
```

**特点**:
- Content 使用 `interface{}` 类型，灵活但缺乏类型安全
- 已支持工具调用 (ToolCalls, ToolCallID)
- 缺少多模态支持 (图像、音频)

### 转换逻辑分布

#### 1. OpenAI 转换 (service/transform_openai.go)
- `TransformOpenAIToUnified` (行 19+): OpenAI → Unified
- `TransformUnifiedToOpenAI` (行 100+): Unified → OpenAI
- 流式响应转换逻辑 (行 700+)

**关键发现**:
- 已处理 system 消息提取 (行 40-69)
- 已处理 tool 角色的 tool_call_id (行 78-82)
- 使用辅助函数 `getString`, `getBool` 安全解析

#### 2. Anthropic 转换 (service/transform_anthropic.go)
- `TransformAnthropicToUnified`: Anthropic → Unified
- `TransformUnifiedToAnthropic`: Unified → Anthropic
- 处理 Anthropic 特有的 thinking 块

#### 3. Responses API 转换 (service/transform_responses.go)
- `TransformResponsesToUnified`: Responses → Unified
- `TransformUnifiedToResponses`: Unified → Responses
- 支持 OpenAI Responses API 格式

### 转换管理器 (service/transformer.go:88-139)
```go
type TransformerManager struct {
    clientType   string // 客户端格式类型
    providerType string // 上游供应商类型
}
```

**工作流程**:
1. `ProcessRequest`: 客户端格式 → Unified → 提供商格式
2. `ProcessResponse`: 提供商格式 → Unified → 客户端格式

**支持的格式**:
- `openai`: OpenAI Chat Completions API
- `openai-res`: OpenAI Responses API
- `anthropic`: Anthropic Messages API

---

## 技术发现

### 1. 指针类型的使用模式
**现有实践**:
- `Temperature *float64` - 可选参数，区分未设置和设置为 0
- `TopP *float64` - 同上
- `ReasoningEffort *string` - 可选字符串参数

**建议**:
- 所有新增可选参数都使用指针类型
- 确保 `omitempty` 标签生效
- 避免零值歧义

### 2. Content 字段的灵活性问题
**当前实现**:
```go
Content interface{} `json:"content,omitempty"`
```

**问题**:
- 缺乏类型安全
- 难以处理多模态内容 (图像、音频)
- 转换逻辑需要大量类型断言

**建议方案** (参考文档):
```go
type UnifiedMessageContent struct {
    Content         *string                     `json:"content,omitempty"`
    MultipleContent []UnifiedMessageContentPart `json:"multiple_content,omitempty"`
}
```

**优势**:
- 类型安全
- 明确区分纯文本和多模态
- 易于扩展

### 3. 辅助函数模式
**现有辅助函数** (service/transform_openai.go):
- `getString(m map[string]interface{}, key string) string`
- `getBool(m map[string]interface{}, key string) bool`

**建议**:
- 添加 `getInt64`, `getFloat64Ptr`, `getStringPtr` 等
- 统一错误处理
- 提取到独立的 utils 文件

### 4. 流式响应处理复杂度
**发现**:
- 流式转换逻辑非常复杂 (transform_openai.go 行 700+)
- 需要维护状态 (sequenceNumber, accumulatedText 等)
- 事件顺序严格 (response.created → in_progress → output_item.added → ...)

**影响**:
- 新增字段需要同步更新流式逻辑
- 测试难度高
- 建议优先实现非流式，再扩展流式

---

## 架构洞察

### 转换层的职责边界
```
客户端请求 → [TransformXToUnified] → UnifiedRequest
                                          ↓
                                    [业务逻辑层]
                                          ↓
UnifiedRequest → [TransformUnifiedToY] → 提供商请求
```

**关键点**:
- Unified 格式是中间表示，不应包含特定提供商的字段
- 转换层负责映射和兼容性处理
- 业务逻辑层只操作 Unified 格式

### 向后兼容性策略
**现有实践**:
- 使用 `omitempty` 标签
- 指针类型表示可选参数
- 保持字段名称稳定

**建议**:
- 新字段都使用 `omitempty`
- 不修改现有字段类型
- 提供默认值处理

---

## 实现优先级建议

### 高优先级 (立即实现)
1. **基础高级参数** (阶段 1)
   - 原因: 常用且简单，影响面小
   - 字段: FrequencyPenalty, PresencePenalty, Seed, Stop, User

2. **响应格式和工具增强** (阶段 2)
   - 原因: 结构化输出需求高
   - 字段: ResponseFormat, ToolChoice, ParallelToolCalls, StreamOptions

### 中优先级 (后续实现)
3. **多模态支持** (阶段 3)
   - 原因: 需要重构 Content 字段，影响面大
   - 风险: 可能破坏现有代码

4. **Embedding API** (阶段 4)
   - 原因: 独立功能，不影响现有 Chat API
   - 收益: 统一 API 入口

### 低优先级 (可选)
5. **缓存和推理优化** (阶段 5)
   - 原因: 提供商特定功能，使用场景有限

6. **扩展性字段** (阶段 6)
   - 原因: 高级用法，普通用户不需要

---

## 测试策略

### 单元测试
- 每个转换函数独立测试
- 测试用例覆盖:
  - 正常情况
  - 边界情况 (nil, 空值, 零值)
  - 错误情况 (格式错误, 缺失字段)

### 集成测试
- 端到端转换测试
- 多提供商兼容性测试
- 流式和非流式测试

### 性能测试
- 转换延迟基准测试
- 内存使用测试
- 并发转换测试

---

## 风险评估

### 高风险
1. **Content 字段重构** (阶段 3)
   - 影响: 所有消息处理逻辑
   - 缓解: 渐进式迁移，保持向后兼容

2. **流式响应逻辑** (所有阶段)
   - 影响: 复杂的状态管理
   - 缓解: 优先实现非流式，充分测试

### 中风险
3. **提供商兼容性** (所有阶段)
   - 影响: 不同提供商支持不同功能
   - 缓解: 优雅降级，清晰错误信息

4. **性能影响** (阶段 3, 6)
   - 影响: 复杂类型转换可能增加延迟
   - 缓解: 性能测试，优化热路径

### 低风险
5. **基础参数扩展** (阶段 1, 2)
   - 影响: 简单字段添加
   - 缓解: 充分测试

---

## 辅助函数分析

### 现有辅助函数 (service/transform_openai.go:2428-2505)
```go
func getString(m map[string]interface{}, key string) string
func getNestedString(m map[string]interface{}, path string) string  // 支持 "response.id"
func getNestedFloat(m map[string]interface{}, path string) float64
func getNestedMap(m map[string]interface{}, path string) map[string]interface{}
func getBool(m map[string]interface{}, key string) bool
func getFloat(m map[string]interface{}, key string) float64
```

**特点**:
- 安全的类型断言，失败返回零值
- `getNestedString/Float/Map` 支持点路径访问嵌套字段
- 无错误返回，简化调用代码

**需要补充的辅助函数**:
```go
func getInt(m map[string]interface{}, key string) int
func getInt64(m map[string]interface{}, key string) int64
func getFloat64Ptr(m map[string]interface{}, key string) *float64
func getStringPtr(m map[string]interface{}, key string) *string
func getInt64Ptr(m map[string]interface{}, key string) *int64
func getBoolPtr(m map[string]interface{}, key string) *bool
func getStringArray(m map[string]interface{}, key string) []string
func getIntMap(m map[string]interface{}, key string) map[string]int64
```

---

## 测试框架分析

### 现有测试文件
1. `service/transformer_test.go` - 基础转换测试
2. `service/transform_responses_test.go` - Responses API 测试
3. `service/transform_responses_stream_test.go` - 流式响应测试
4. `service/model_template_test.go` - 模型模板测试

### 测试模式 (transformer_test.go:1-100)
```go
func TestTransformOpenAIToUnified(t *testing.T) {
    openaiRequest := []byte(`{...}`)
    unified, err := TransformOpenAIToUnified(openaiRequest)
    if err != nil {
        t.Fatalf("TransformOpenAIToUnified failed: %v", err)
    }
    // 断言字段值
}
```

**特点**:
- 使用 JSON 字符串作为测试输入
- 简单的字段值断言
- 缺少边界情况测试 (nil, 空值, 零值)

**建议改进**:
- 使用表驱动测试 (table-driven tests)
- 添加边界情况测试用例
- 添加错误情况测试
- 使用测试夹具 (fixtures) 管理复杂测试数据

---

## 实施细节补充

### 阶段 1 实施细节

#### 1.1 新增字段定义
```go
type UnifiedRequest struct {
    // 现有字段...

    // 阶段 1: 基础高级参数
    FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
    PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
    Seed             *int64            `json:"seed,omitempty"`
    LogitBias        map[string]int64  `json:"logit_bias,omitempty"`
    Stop             *UnifiedStop      `json:"stop,omitempty"`
    User             *string           `json:"user,omitempty"`
    Metadata         map[string]string `json:"metadata,omitempty"`
}

type UnifiedStop struct {
    Single   *string
    Multiple []string
}
```

#### 1.2 转换逻辑更新点

**TransformOpenAIToUnified** (service/transform_openai.go:19+):
```go
// 在现有代码后添加
if freq, ok := req["frequency_penalty"].(float64); ok {
    unified.FrequencyPenalty = &freq
}
if pres, ok := req["presence_penalty"].(float64); ok {
    unified.PresencePenalty = &pres
}
if seed, ok := req["seed"].(float64); ok {
    seedInt := int64(seed)
    unified.Seed = &seedInt
}
// ... 其他字段
```

**TransformUnifiedToOpenAI** (service/transform_openai.go:100+):
```go
// 在现有代码后添加
if unified.FrequencyPenalty != nil {
    req["frequency_penalty"] = *unified.FrequencyPenalty
}
if unified.PresencePenalty != nil {
    req["presence_penalty"] = *unified.PresencePenalty
}
// ... 其他字段
```

**TransformUnifiedToAnthropic** (service/transform_anthropic.go:102+):
```go
// Anthropic 不支持 frequency_penalty, presence_penalty, seed
// 只映射兼容字段
if unified.Stop != nil {
    if unified.Stop.Single != nil {
        req["stop_sequences"] = []string{*unified.Stop.Single}
    } else if len(unified.Stop.Multiple) > 0 {
        req["stop_sequences"] = unified.Stop.Multiple
    }
}
if unified.Metadata != nil {
    req["metadata"] = unified.Metadata
}
```

#### 1.3 测试用例设计
```go
func TestTransformOpenAIToUnified_AdvancedParams(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        validate func(*testing.T, *UnifiedRequest)
    }{
        {
            name: "frequency_penalty",
            input: `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"frequency_penalty":0.5}`,
            validate: func(t *testing.T, u *UnifiedRequest) {
                if u.FrequencyPenalty == nil || *u.FrequencyPenalty != 0.5 {
                    t.Errorf("expected frequency_penalty=0.5, got %v", u.FrequencyPenalty)
                }
            },
        },
        // ... 更多测试用例
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            unified, err := TransformOpenAIToUnified([]byte(tt.input))
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            tt.validate(t, unified)
        })
    }
}
```

---

## 兼容性矩阵

### 参数支持情况

| 参数 | OpenAI | Anthropic | Responses API | 备注 |
|------|--------|-----------|---------------|------|
| FrequencyPenalty | ✅ | ❌ | ✅ | Anthropic 不支持 |
| PresencePenalty | ✅ | ❌ | ✅ | Anthropic 不支持 |
| Seed | ✅ | ❌ | ✅ | Anthropic 不支持 |
| LogitBias | ✅ | ❌ | ✅ | Anthropic 不支持 |
| Stop | ✅ | ✅ (stop_sequences) | ✅ | 字段名不同 |
| User | ✅ | ❌ | ✅ | Anthropic 不支持 |
| Metadata | ✅ | ✅ | ✅ | 都支持 |
| ResponseFormat | ✅ | ❌ | ✅ | Anthropic 部分支持 |
| ToolChoice | ✅ | ✅ (tool_choice) | ✅ | 格式略有不同 |
| ParallelToolCalls | ✅ | ❌ | ✅ | Anthropic 不支持 |
| StreamOptions | ✅ | ❌ | ✅ | Anthropic 不支持 |

**处理策略**:
- ✅ 支持: 直接映射
- ❌ 不支持: 忽略该字段，不报错
- 部分支持: 映射到最接近的功能

---

## 性能考虑

### 转换性能基准
**当前性能** (需要实际测试):
- 简单请求转换: ~100-500μs
- 复杂请求 (含工具): ~500-2000μs
- 流式响应转换: ~50-200μs/事件

**优化策略**:
1. **延迟解析**: 使用 `json.RawMessage` 延迟解析不常用字段
2. **对象池**: 复用 `UnifiedRequest` 对象
3. **避免反射**: 使用类型断言而非反射
4. **减少分配**: 预分配切片容量

### 内存使用
**当前估算**:
- `UnifiedRequest` 基础大小: ~200 bytes
- 每个 `UnifiedMessage`: ~100 bytes
- 每个 `UnifiedTool`: ~150 bytes

**扩展后估算**:
- `UnifiedRequest` 扩展后: ~400-500 bytes (增加 ~100%)
- 影响: 可接受，大部分字段是指针 (8 bytes)

---

## 参考资料

### 内部文档
- `docs/UNIFIED_REQUEST_EXTENSION.md` - 扩展建议文档
- `CLAUDE.md` - 项目规范
- `memory/MEMORY.md` - 项目记忆

### 代码文件
- `service/transformer.go` - 核心类型定义
- `service/transform_openai.go` - OpenAI 转换逻辑 (2500+ 行)
- `service/transform_anthropic.go` - Anthropic 转换逻辑
- `service/transform_responses.go` - Responses API 转换逻辑
- `service/transformer_test.go` - 基础测试

### 外部参考
- Octopus 项目的 `InternalLLMRequest` 实现
- OpenAI API 文档
- Anthropic API 文档

---

## Octopus 项目参考分析

### InternalLLMRequest 完整结构 (E:\a-2025_12-projects\octopus\internal\transformer\model\model.go)

#### 核心字段对比

| 字段 | Octopus | LLMIO 当前 | 建议 |
|------|---------|-----------|------|
| **基础参数** |
| Model | ✅ string | ✅ string | 保持 |
| Messages | ✅ []Message | ✅ []UnifiedMessage | 保持 |
| MaxTokens | ✅ *int64 | ✅ int | 改为 *int64 |
| Temperature | ✅ *float64 | ✅ *float64 | 保持 |
| TopP | ✅ *float64 | ✅ *float64 | 保持 |
| Stream | ✅ *bool | ✅ bool | 改为 *bool |
| Tools | ✅ []Tool | ✅ []UnifiedTool | 保持 |
| System | ❌ (在 Messages 中) | ✅ string | 保持 LLMIO 设计 |
| **高级参数** |
| FrequencyPenalty | ✅ *float64 | ❌ | 需添加 |
| PresencePenalty | ✅ *float64 | ❌ | 需添加 |
| Seed | ✅ *int64 | ❌ | 需添加 |
| LogitBias | ✅ map[string]int64 | ❌ | 需添加 |
| Logprobs | ✅ *bool | ❌ | 需添加 |
| TopLogprobs | ✅ *int64 | ❌ | 需添加 |
| MaxCompletionTokens | ✅ *int64 | ❌ | 需添加 |
| Store | ✅ *bool | ❌ | 需添加 |
| **缓存和优化** |
| PromptCacheKey | ✅ *bool | ❌ | 需添加 |
| SafetyIdentifier | ✅ *string | ❌ | 需添加 |
| User | ✅ *string | ❌ | 需添加 |
| Metadata | ✅ map[string]string | ❌ | 需添加 |
| **多模态** |
| Modalities | ✅ []string | ❌ | 需添加 |
| Audio | ✅ *struct{Format, Voice} | ❌ | 需添加 |
| **推理** |
| ReasoningEffort | ✅ string | ✅ *string | 改为 string |
| ReasoningBudget | ✅ *int64 | ❌ | 需添加 |
| EnableThinking | ✅ *bool | ❌ | 需添加 |
| ServiceTier | ✅ *string | ❌ | 需添加 |
| **工具和响应** |
| Stop | ✅ *Stop | ❌ | 需添加 |
| StreamOptions | ✅ *StreamOptions | ❌ | 需添加 |
| ParallelToolCalls | ✅ *bool | ❌ | 需添加 |
| ToolChoice | ✅ *ToolChoice | ❌ | 需添加 |
| ResponseFormat | ✅ *ResponseFormat | ❌ | 需添加 |
| **Embedding API** |
| EmbeddingInput | ✅ *EmbeddingInput | ❌ | 需添加 |
| EmbeddingDimensions | ✅ *int64 | ❌ | 需添加 |
| EmbeddingEncodingFormat | ✅ *string | ❌ | 需添加 |
| **帮助字段** |
| ExtraBody | ✅ json.RawMessage | ❌ | 需添加 |
| RawRequest | ✅ []byte | ❌ | 需添加 |
| RawAPIFormat | ✅ APIFormat | ❌ | 需添加 |
| TransformerMetadata | ✅ map[string]string | ❌ | 需添加 |
| TransformOptions | ✅ TransformOptions | ❌ | 需添加 |
| Include | ✅ []string | ❌ | 需添加 |
| Query | ✅ url.Values | ❌ | 需添加 |

#### 关键发现

**1. Stop 类型实现 (行 316-351)**
```go
type Stop struct {
    Stop         *string
    MultipleStop []string
}

func (s Stop) MarshalJSON() ([]byte, error) {
    if s.Stop != nil {
        return json.Marshal(s.Stop)
    }
    if len(s.MultipleStop) > 0 {
        return json.Marshal(s.MultipleStop)
    }
    return []byte("[]"), nil
}

func (s *Stop) UnmarshalJSON(data []byte) error {
    var str string
    err := json.Unmarshal(data, &str)
    if err == nil {
        s.Stop = &str
        return nil
    }
    var strs []string
    err = json.Unmarshal(data, &strs)
    if err == nil {
        s.MultipleStop = strs
        return nil
    }
    return errors.New("invalid stop type")
}
```
**优势**: 自动处理 JSON 序列化/反序列化，支持 string 或 []string

**2. MessageContent 类型实现 (行 429-464)**
```go
type MessageContent struct {
    Content         *string              `json:"content,omitempty"`
    MultipleContent []MessageContentPart `json:"multiple_content,omitempty"`
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
    if len(c.MultipleContent) > 0 {
        // 优化: 单个 text 类型直接序列化为字符串
        if len(c.MultipleContent) == 1 && c.MultipleContent[0].Type == "text" {
            return json.Marshal(c.MultipleContent[0].Text)
        }
        return json.Marshal(c.MultipleContent)
    }
    return json.Marshal(c.Content)
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
    var str string
    err := json.Unmarshal(data, &str)
    if err == nil {
        c.Content = &str
        return nil
    }
    var parts []MessageContentPart
    err = json.Unmarshal(data, &parts)
    if err == nil {
        c.MultipleContent = parts
        return nil
    }
    return errors.New("invalid content type")
}
```
**优势**:
- 自动处理 string 或 []ContentPart
- 优化单个文本的序列化
- 完全向后兼容

**3. ToolChoice 类型实现 (行 816-852)**
```go
type ToolChoice struct {
    ToolChoice      *string          `json:"tool_choice,omitempty"`
    NamedToolChoice *NamedToolChoice `json:"named_tool_choice,omitempty"`
}

type NamedToolChoice struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

func (t ToolChoice) MarshalJSON() ([]byte, error) {
    if t.ToolChoice != nil {
        return json.Marshal(t.ToolChoice)
    }
    return json.Marshal(t.NamedToolChoice)
}

func (t *ToolChoice) UnmarshalJSON(data []byte) error {
    var str string
    err := json.Unmarshal(data, &str)
    if err == nil {
        t.ToolChoice = &str
        return nil
    }
    var named NamedToolChoice
    err = json.Unmarshal(data, &named)
    if err == nil {
        t.NamedToolChoice = &named
        return nil
    }
    return errors.New("invalid tool choice type")
}
```
**优势**: 支持 "auto"/"none"/"required" 或 {type: "function", function: {name: "..."}}

**4. EmbeddingInput 类型实现 (行 884-921)**
```go
type EmbeddingInput struct {
    Single   *string
    Multiple []string
}

func (i EmbeddingInput) MarshalJSON() ([]byte, error) {
    if i.Single != nil {
        return json.Marshal(i.Single)
    }
    if len(i.Multiple) > 0 {
        return json.Marshal(i.Multiple)
    }
    return []byte("null"), nil
}

func (i *EmbeddingInput) UnmarshalJSON(data []byte) error {
    var str string
    err := json.Unmarshal(data, &str)
    if err == nil {
        i.Single = &str
        return nil
    }
    var strs []string
    err = json.Unmarshal(data, &strs)
    if err == nil {
        i.Multiple = strs
        return nil
    }
    return errors.New("invalid input type")
}
```
**优势**: 自动处理单个字符串或字符串数组

**5. Message 结构 (行 354-404)**
```go
type Message struct {
    Role    string         `json:"role,omitempty"`
    Content MessageContent `json:"content,omitzero"` // 注意: omitzero 而非 omitempty
    Name    *string        `json:"name,omitempty"`
    Refusal string         `json:"refusal,omitempty"`

    // Tool call 相关
    MessageIndex    *int    `json:"-"`
    ToolCallID      *string `json:"tool_call_id,omitempty"`
    ToolCallName    *string `json:"-"`
    ToolCallIsError *bool   `json:"-"`
    ToolCalls       []ToolCall `json:"tool_calls,omitempty"`

    // 多模态
    Images []MessageContentPart `json:"images,omitempty"`
    Audio  *struct{...}         `json:"audio,omitempty"`

    // 推理
    ReasoningContent   *string `json:"reasoning_content,omitempty"`
    Reasoning          *string `json:"reasoning,omitempty"`
    ReasoningSignature *string `json:"reasoning_signature,omitempty"`

    // 缓存控制
    CacheControl *CacheControl `json:"-"`
}
```
**关键点**:
- `omitzero` 标签: 确保空 Content 不被序列化
- 多个推理字段: 兼容不同提供商 (DeepSeek, OpenRouter, Ollama)
- 帮助字段: 使用 `json:"-"` 标记不序列化

**6. MessageContentPart 结构 (行 466-486)**
```go
type MessageContentPart struct {
    Type     string        `json:"type"`
    Text     *string       `json:"text,omitempty"`
    ImageURL *ImageURL     `json:"image_url,omitempty"`
    Audio    *Audio        `json:"input_audio,omitempty"`
    File     *File         `json:"file,omitempty"`
    CacheControl *CacheControl `json:"-"`
}

type ImageURL struct {
    URL    string  `json:"url"`
    Detail *string `json:"detail,omitempty"` // "auto", "low", "high"
}

type Audio struct {
    Format string `json:"format"` // "wav", "mp3"
    Data   string `json:"data"`   // base64
}

type File struct {
    Filename string `json:"filename"`
    FileData string `json:"file_data"` // base64
}
```
**支持的类型**: text, image_url, input_audio, file

**7. 验证方法 (行 247-277)**
```go
func (r *InternalLLMRequest) Validate() error {
    if r.Model == "" {
        return errors.New("model is required")
    }

    isEmbeddingRequest := r.EmbeddingInput != nil
    isChatRequest := len(r.Messages) > 0

    if isEmbeddingRequest && isChatRequest {
        return errors.New("cannot specify both messages and input")
    }

    if !isEmbeddingRequest && !isChatRequest {
        return errors.New("either messages or input is required")
    }

    // ... 更多验证
    return nil
}
```
**优势**: 清晰的请求类型验证

**8. 帮助方法**
```go
func (r *InternalLLMRequest) IsEmbeddingRequest() bool
func (r *InternalLLMRequest) IsChatRequest() bool
func (r *InternalLLMRequest) IsImageGenerationRequest() bool
func (r *InternalLLMRequest) ClearHelpFields()
func (m *Message) GetReasoningContent() string
func (m *Message) SetReasoningContent(s string)
```

---

## 实施建议更新

### 关键改进点

**1. 采用自定义 JSON 序列化**
- Stop, MessageContent, ToolChoice, EmbeddingInput 都实现 MarshalJSON/UnmarshalJSON
- 优势: 自动处理多种类型，代码更简洁

**2. 使用 omitzero 标签**
- 对于复杂类型使用 `omitzero` 而非 `omitempty`
- 确保空值不被序列化

**3. 添加验证方法**
- Validate() 方法验证请求合法性
- IsXXXRequest() 方法判断请求类型

**4. 帮助字段设计**
- 使用 `json:"-"` 标记不序列化的字段
- TransformerMetadata, TransformOptions 等用于内部传递

**5. 类型命名**
- Octopus 使用 `InternalLLMRequest`
- LLMIO 使用 `UnifiedRequest`
- 建议保持 LLMIO 命名，更符合项目定位

---

## 更新后的实施优先级

### 阶段 1: 基础字段扩展 (高优先级)
**新增字段**:
- FrequencyPenalty, PresencePenalty, Seed, LogitBias
- Stop (使用自定义 JSON 序列化)
- User, Metadata
- Logprobs, TopLogprobs
- MaxCompletionTokens, Store

**预计时间**: 3-4 小时

### 阶段 2: 响应格式和工具增强 (高优先级)
**新增字段**:
- ResponseFormat (使用自定义类型)
- ToolChoice (使用自定义 JSON 序列化)
- ParallelToolCalls
- StreamOptions

**预计时间**: 2-3 小时

### 阶段 3: 多模态内容支持 (中优先级)
**重构**:
- MessageContent (使用自定义 JSON 序列化)
- MessageContentPart (支持 text, image_url, input_audio, file)
- 添加 Modalities, Audio 字段

**预计时间**: 4-5 小时

### 阶段 4: Embedding API 集成 (中优先级)
**新增字段**:
- EmbeddingInput (使用自定义 JSON 序列化)
- EmbeddingDimensions, EmbeddingEncodingFormat
- 添加 Validate(), IsEmbeddingRequest() 方法

**预计时间**: 3-4 小时

### 阶段 5: 缓存和推理优化 (低优先级)
**新增字段**:
- PromptCacheKey, SafetyIdentifier
- ReasoningBudget, EnableThinking, ServiceTier
- CacheControl (在 Message 和 ContentPart 中)

**预计时间**: 2-3 小时

### 阶段 6: 扩展性和元数据 (低优先级)
**新增字段**:
- ExtraBody, RawRequest, RawAPIFormat
- TransformerMetadata, TransformOptions
- Include, Query

**预计时间**: 2-3 小时

### 阶段 7: 测试和文档完善
**任务**:
- 完整测试套件
- 性能测试
- 文档更新

**预计时间**: 3-4 小时

---

**最后更新**: 2026-02-25
**研究状态**: Octopus 参考分析完成，准备实施
