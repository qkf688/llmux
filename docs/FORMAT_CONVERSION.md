# 格式转换功能说明

## 概述

llmio 现在支持客户端请求格式与上游供应商格式的自动转换。这意味着：

- 客户端可以使用 **OpenAI 格式**请求，路由到 **OpenAI** 或 **Anthropic** 上游供应商
- 客户端可以使用 **Anthropic 格式**请求，路由到 **OpenAI** 或 **Anthropic** 上游供应商

**重要特性**：系统会自动检测客户端格式与供应商类型，当两者不一致时自动进行格式转换，无需任何额外配置！

## 工作原理

系统采用三层转换架构：

```
客户端格式 → 统一格式 → 上游供应商格式
```

### 转换流程

1. **请求转换**：
   - 客户端请求 → 统一格式
   - 统一格式 → 上游供应商格式

2. **响应转换**：
   - 上游供应商响应 → 统一格式
   - 统一格式 → 客户端期望格式

## 使用场景

### 场景 1：OpenAI 客户端 → Anthropic 供应商

客户端使用 OpenAI SDK 或格式：

```bash
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }'
```

如果配置的 `claude-3-opus` 模型关联的是 Anthropic 类型的供应商，系统会自动：
1. 将 OpenAI 格式转换为 Anthropic 格式
2. 发送到 Anthropic API
3. 将 Anthropic 响应转换回 OpenAI 格式返回给客户端

### 场景 2：Anthropic 客户端 → OpenAI 供应商

客户端使用 Anthropic SDK 或格式：

```bash
curl http://localhost:7070/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_TOKEN" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 100
  }'
```

如果配置的 `gpt-4` 模型关联的是 OpenAI 类型的供应商，系统会自动：
1. 将 Anthropic 格式转换为 OpenAI 格式
2. 发送到 OpenAI API
3. 将 OpenAI 响应转换回 Anthropic 格式返回给客户端

## 配置说明

### 1. 添加供应商

在管理后台添加供应商时，选择正确的类型：
- **OpenAI** 类型：用于 OpenAI、Azure OpenAI 等兼容 OpenAI API 的服务
- **Anthropic** 类型：用于 Anthropic Claude API

### 2. 配置模型

在模型配置中：
1. 创建模型（如 `gpt-4` 或 `claude-3-opus`）
2. 关联供应商（可以是任意类型的供应商）
3. 系统会自动处理格式转换

### 3. 客户端请求

客户端可以使用任意支持的格式：

**OpenAI 格式端点：**
- `/v1/chat/completions` - 标准 OpenAI 聊天完成
- `/v1/responses` - OpenAI Responses API

**Anthropic 格式端点：**
- `/v1/messages` - Anthropic Messages API

## 支持的功能

### 基础功能
- ✅ 文本消息转换
- ✅ 流式响应转换
- ✅ 参数转换（temperature, max_tokens, top_p 等）
- ✅ 工具调用（Tool Calls / Function Calling）
- ✅ 系统提示词（System Prompt）

### 高级功能
- ✅ 多轮对话
- ✅ Token 使用统计
- ✅ 错误处理和重试
- ✅ 负载均衡

### Anthropic 特有功能
- ✅ **缓存控制 (Cache Control)** - 降低 API 成本
  - 消息级别缓存
  - 工具级别缓存
  - 内容部分级别缓存
- ✅ **思考配置 (Extended Thinking)** - 深度推理控制
  - `reasoning_budget` (token 预算)
  - `reasoning_effort` (推理强度: low/medium/high)
  - 自动映射转换

### 参数验证和修复
- ✅ **参数范围验证**
  - temperature (0-2)
  - top_p (0-1)
  - frequency_penalty (-2 to 2)
  - presence_penalty (-2 to 2)
  - top_logprobs (0-20)
- ✅ **自动参数修复** - 超出范围的参数自动限制在有效范围内

### 多模态支持
- ✅ 图像输入 (image_url)
- ✅ 音频输入 (input_audio)
- ✅ 音频输出配置 (audio)
- ✅ 多模态输出 (modalities)

### 流式增强
- ✅ 流式使用统计 (stream_options.include_usage)
- ✅ 并行工具调用 (parallel_tool_calls)

## 参数转换映射表

### 通用参数映射

| 统一格式 | OpenAI | Anthropic | 说明 |
|---------|--------|-----------|------|
| `model` | `model` | `model` | 模型名称 |
| `messages` | `messages` | `messages` | 消息数组 |
| `max_tokens` | `max_tokens` | `max_tokens` | 最大生成 token 数 |
| `temperature` | `temperature` | `temperature` | 温度参数 (0-2) |
| `top_p` | `top_p` | `top_p` | 核采样参数 (0-1) |
| `stream` | `stream` | `stream` | 是否流式响应 |
| `tools` | `tools` | `tools` | 工具定义数组 |
| `system` | `messages[0]` (role=system) | `system` | 系统提示词 |

### 高级参数映射

| 统一格式 | OpenAI | Anthropic | 说明 |
|---------|--------|-----------|------|
| `frequency_penalty` | `frequency_penalty` | ❌ | 频率惩罚 (-2 to 2) |
| `presence_penalty` | `presence_penalty` | ❌ | 存在惩罚 (-2 to 2) |
| `seed` | `seed` | ❌ | 随机种子 |
| `logit_bias` | `logit_bias` | ❌ | Token 偏置 |
| `stop` | `stop` | `stop_sequences` | 停止序列 |
| `user` | `user` | ❌ | 用户标识 |
| `metadata` | `metadata` | `metadata` | 元数据 |
| `logprobs` | `logprobs` | ❌ | 是否返回对数概率 |
| `top_logprobs` | `top_logprobs` | ❌ | 返回 top-k 对数概率 (0-20) |
| `max_completion_tokens` | `max_completion_tokens` | ❌ | 最大完成 token 数 |
| `store` | `store` | ❌ | 是否存储输出 |

### 响应格式和工具控制

| 统一格式 | OpenAI | Anthropic | 说明 |
|---------|--------|-----------|------|
| `response_format` | `response_format` | ❌ | 响应格式控制 (text/json_object/json_schema) |
| `tool_choice` | `tool_choice` | `tool_choice` | 工具选择控制 (auto/none/required/specific) |
| `parallel_tool_calls` | `parallel_tool_calls` | ❌ | 是否并行调用工具 |
| `stream_options` | `stream_options` | ❌ | 流式选项 (include_usage) |

### Anthropic 特有参数

| 统一格式 | OpenAI | Anthropic | 说明 |
|---------|--------|-----------|------|
| `cache_control` | ❌ | `cache_control` | 缓存控制（消息/工具/内容部分级别） |
| `reasoning_budget` | ❌ | `thinking.budget_tokens` | 推理预算 token 数 |
| `reasoning_effort` | `reasoning_effort` | 自动映射 | 推理强度 (low/medium/high) |

### 多模态参数

| 统一格式 | OpenAI | Anthropic | 说明 |
|---------|--------|-----------|------|
| `modalities` | `modalities` | ❌ | 输出模态 (text/audio/image) |
| `audio` | `audio` | ❌ | 音频输出配置 (voice/format) |
| `content[].image_url` | `content[].image_url` | `content[].source` | 图像输入 |
| `content[].input_audio` | `content[].input_audio` | ❌ | 音频输入 |

### 参数验证规则

| 参数 | 有效范围 | 自动修复 |
|------|---------|---------|
| `temperature` | 0-2 | ✅ 超出范围自动限制 |
| `top_p` | 0-1 | ✅ 超出范围自动限制 |
| `frequency_penalty` | -2 to 2 | ✅ 超出范围自动限制 |
| `presence_penalty` | -2 to 2 | ✅ 超出范围自动限制 |
| `top_logprobs` | 0-20 | ✅ 超出范围自动限制 |
| `max_tokens` | > 0 | ✅ 负数自动修正为 1 |

## 使用示例

### 示例 1: 使用缓存控制降低成本

```bash
# OpenAI 格式请求 → Anthropic 供应商
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "claude-3-opus",
    "messages": [
      {
        "role": "user",
        "content": "Long system context that should be cached...",
        "cache_control": {"type": "ephemeral"}
      }
    ],
    "max_tokens": 100
  }'
```

### 示例 2: 使用思考配置进行深度推理

```bash
# 使用 reasoning_budget (token 预算)
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Solve this complex problem"}],
    "reasoning_budget": 30000,
    "max_tokens": 1000
  }'

# 或使用 reasoning_effort (推理强度)
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Solve this complex problem"}],
    "reasoning_effort": "high",
    "max_tokens": 1000
  }'
```

### 示例 3: 多模态输入

```bash
# 图像 + 文本输入
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "gpt-4-vision",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What is in this image?"},
        {
          "type": "image_url",
          "image_url": {
            "url": "https://example.com/image.jpg",
            "detail": "high"
          }
        }
      ]
    }],
    "max_tokens": 300
  }'
```

### 示例 4: 工具调用与缓存

```bash
# 工具定义使用缓存控制
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "What is the weather?"}],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get weather information",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {"type": "string"}
            }
          }
        },
        "cache_control": {"type": "ephemeral"}
      }
    ],
    "max_tokens": 100
  }'
```

## 技术实现

### 核心组件

1. **统一格式定义** (`service/transformer.go`)
   - `UnifiedRequest` - 统一请求格式
   - `UnifiedResponse` - 统一响应格式
   - `UnifiedMessage` - 统一消息格式

2. **OpenAI 转换器** (`service/transform_openai.go`)
   - `TransformOpenAIToUnified` - OpenAI → 统一格式
   - `TransformUnifiedToOpenAI` - 统一格式 → OpenAI

3. **Anthropic 转换器** (`service/transform_anthropic.go`)
   - `TransformAnthropicToUnified` - Anthropic → 统一格式
   - `TransformUnifiedToAnthropic` - 统一格式 → Anthropic

4. **Responses API 转换器** (`service/transform_responses.go`)
   - `TransformResponsesToUnified` - Responses API → 统一格式
   - `TransformUnifiedToResponses` - 统一格式 → Responses API

5. **参数验证和修复** (`service/validation.go`)
   - `ValidateUnifiedRequest` - 验证请求参数
   - `RepairUnifiedRequest` - 自动修复无效参数

6. **转换管理器** (`service/transformer.go`)
   - `TransformerManager` - 协调请求和响应的转换流程

### 集成点

格式转换在 [`service/chat.go`](../service/chat.go) 的 [`BalanceChat`](../service/chat.go:23) 函数中自动触发：

```go
// 当客户端格式与供应商格式不一致时，自动转换请求
if style != provider.Type {
    tm := NewTransformerManager(style, provider.Type)
    convertedBody, err := tm.ProcessRequest(ctx, before.raw)
    // ...
}

// 转换响应
if style != provider.Type {
    tm := NewTransformerManager(style, provider.Type)
    convertedRes, err := tm.ProcessResponse(res)
    // ...
}
```

## 性能考虑

- 格式转换在内存中进行，性能开销极小
- 仅在客户端格式与供应商格式不一致时才进行转换
- 相同格式的请求直接透传，无额外开销

## 测试

运行格式转换测试：

```bash
go test ./service -v -run TestTransform
```

## 注意事项

1. **模型名称**：确保模型名称在客户端和供应商之间正确映射
2. **特殊参数**：某些供应商特有的参数可能无法完全转换
3. **流式响应**：流式响应的转换可能会有轻微延迟
4. **工具调用**：工具调用格式在不同供应商间可能有细微差异
5. **缓存控制**：仅 Anthropic 供应商支持，OpenAI 会静默忽略
6. **参数验证**：系统会自动验证和修复超出范围的参数
7. **推理配置**：`reasoning_budget` 和 `reasoning_effort` 会自动映射转换

## 最佳实践

### 1. 使用缓存控制降低成本

对于长系统提示词或大量工具定义，使用缓存控制可以显著降低 Anthropic API 成本：

```javascript
// 缓存长系统提示词
{
  "messages": [
    {
      "role": "user",
      "content": "Very long system context...",
      "cache_control": {"type": "ephemeral"}
    }
  ]
}

// 缓存工具定义
{
  "tools": [
    {
      "type": "function",
      "function": {...},
      "cache_control": {"type": "ephemeral"}
    }
  ]
}
```

**缓存有效期**: 5 分钟

### 2. 合理使用推理配置

根据任务复杂度选择合适的推理强度：

- **low** (1000 tokens): 简单问题、快速响应
- **medium** (20000 tokens): 中等复杂度问题
- **high** (50000 tokens): 复杂推理、深度分析

```javascript
// 简单任务
{"reasoning_effort": "low"}

// 复杂任务
{"reasoning_effort": "high"}
// 或
{"reasoning_budget": 30000}
```

### 3. 参数验证

系统会自动修复超出范围的参数，但建议在客户端就使用正确的范围：

```javascript
{
  "temperature": 0.7,        // ✅ 有效范围 0-2
  "top_p": 0.9,             // ✅ 有效范围 0-1
  "frequency_penalty": 0.5,  // ✅ 有效范围 -2 to 2
  "presence_penalty": 0.3    // ✅ 有效范围 -2 to 2
}
```

### 4. 多模态内容

使用多模态内容时，确保格式正确：

```javascript
{
  "messages": [{
    "role": "user",
    "content": [
      {"type": "text", "text": "Describe this image"},
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/image.jpg",
          "detail": "high"  // auto/low/high
        }
      }
    ]
  }]
}
```

### 5. 流式响应优化

使用流式响应时，启用使用统计：

```javascript
{
  "stream": true,
  "stream_options": {
    "include_usage": true  // 在流式响应中包含 token 使用统计
  }
}
```

## 故障排查

### 问题：转换失败

**检查项：**
1. 供应商类型配置是否正确
2. 请求格式是否符合规范
3. 查看日志中的转换错误信息

### 问题：响应格式不正确

**检查项：**
1. 确认客户端使用的端点与期望格式匹配
2. 检查供应商返回的原始响应
3. 查看转换器日志

## 未来计划

- [ ] 支持更多供应商格式（Google Gemini, Cohere 等）
- [ ] 优化流式响应转换性能
- [ ] 添加格式转换的详细日志和监控
- [ ] 支持自定义转换规则

## 参考

- [OpenAI API 文档](https://platform.openai.com/docs/api-reference)
- [Anthropic API 文档](https://docs.anthropic.com/claude/reference)