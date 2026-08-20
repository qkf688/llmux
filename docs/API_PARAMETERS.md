# API 参数完整指南

## 概述

本文档详细说明 llmux 支持的所有 API 参数，包括参数说明、使用示例和最佳实践。

## 目录

- [基础参数](#基础参数)
- [高级参数](#高级参数)
- [Anthropic 特有参数](#anthropic-特有参数)
- [多模态参数](#多模态参数)
- [工具和响应控制](#工具和响应控制)
- [流式参数](#流式参数)
- [参数验证规则](#参数验证规则)
- [最佳实践](#最佳实践)

---

## 基础参数

### model
- **类型**: `string`
- **必需**: ✅
- **说明**: 要使用的模型名称
- **示例**: `"gpt-4"`, `"claude-3-opus"`, `"gpt-3.5-turbo"`

```json
{
  "model": "gpt-4"
}
```

### messages
- **类型**: `array`
- **必需**: ✅
- **说明**: 对话消息数组
- **格式**:
  ```json
  [
    {"role": "system", "content": "You are a helpful assistant"},
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi! How can I help?"}
  ]
  ```

**支持的角色**:
- `system`: 系统提示词
- `user`: 用户消息
- `assistant`: 助手响应
- `tool`: 工具调用结果

### max_tokens
- **类型**: `integer`
- **必需**: ❌
- **说明**: 生成的最大 token 数
- **范围**: > 0
- **默认值**: 模型默认值
- **示例**: `100`, `1000`, `4096`

```json
{
  "max_tokens": 1000
}
```

### temperature
- **类型**: `float`
- **必需**: ❌
- **说明**: 控制输出的随机性
- **范围**: 0-2
- **默认值**: 1.0
- **建议**:
  - `0-0.3`: 确定性输出（代码生成、数据提取）
  - `0.7-1.0`: 平衡创造性和一致性（通用对话）
  - `1.0-2.0`: 高创造性（创意写作、头脑风暴）

```json
{
  "temperature": 0.7
}
```

### top_p
- **类型**: `float`
- **必需**: ❌
- **说明**: 核采样参数，控制输出多样性
- **范围**: 0-1
- **默认值**: 1.0
- **建议**: 通常与 temperature 二选一使用

```json
{
  "top_p": 0.9
}
```

### stream
- **类型**: `boolean`
- **必需**: ❌
- **说明**: 是否使用流式响应
- **默认值**: `false`

```json
{
  "stream": true
}
```

---

## 高级参数

### frequency_penalty
- **类型**: `float`
- **必需**: ❌
- **说明**: 基于 token 出现频率的惩罚
- **范围**: -2.0 to 2.0
- **默认值**: 0
- **效果**:
  - 正值: 减少重复内容
  - 负值: 增加重复内容

```json
{
  "frequency_penalty": 0.5
}
```

### presence_penalty
- **类型**: `float`
- **必需**: ❌
- **说明**: 基于 token 是否出现的惩罚
- **范围**: -2.0 to 2.0
- **默认值**: 0
- **效果**:
  - 正值: 鼓励讨论新话题
  - 负值: 鼓励保持当前话题

```json
{
  "presence_penalty": 0.3
}
```

### seed
- **类型**: `integer`
- **必需**: ❌
- **说明**: 随机种子，用于确定性输出
- **用途**: 重现相同的输出结果

```json
{
  "seed": 12345
}
```

### logit_bias
- **类型**: `object`
- **必需**: ❌
- **说明**: 修改特定 token 的出现概率
- **范围**: -100 to 100
- **格式**: `{"token_id": bias_value}`

```json
{
  "logit_bias": {
    "50256": -100,
    "1234": 50
  }
}
```

### stop
- **类型**: `string` 或 `array`
- **必需**: ❌
- **说明**: 停止序列，遇到时停止生成
- **最多**: 4 个序列

```json
{
  "stop": "END"
}
// 或
{
  "stop": ["END", "STOP", "\n\n"]
}
```

### user
- **类型**: `string`
- **必需**: ❌
- **说明**: 用户标识符，用于监控和滥用检测

```json
{
  "user": "user-123"
}
```

### metadata
- **类型**: `object`
- **必需**: ❌
- **说明**: 附加元数据（最多 16 个键值对）

```json
{
  "metadata": {
    "session_id": "abc123",
    "user_tier": "premium"
  }
}
```

### logprobs
- **类型**: `boolean`
- **必需**: ❌
- **说明**: 是否返回对数概率
- **默认值**: `false`

```json
{
  "logprobs": true
}
```

### top_logprobs
- **类型**: `integer`
- **必需**: ❌
- **说明**: 返回每个位置的 top-k 对数概率
- **范围**: 0-20
- **前提**: `logprobs` 必须为 `true`

```json
{
  "logprobs": true,
  "top_logprobs": 5
}
```

### max_completion_tokens
- **类型**: `integer`
- **必需**: ❌
- **说明**: 完成部分的最大 token 数
- **范围**: > 0

```json
{
  "max_completion_tokens": 500
}
```

### store
- **类型**: `boolean`
- **必需**: ❌
- **说明**: 是否存储此次对话输出
- **默认值**: `false`

```json
{
  "store": true
}
```

---

## Anthropic 特有参数

### cache_control
- **类型**: `object`
- **必需**: ❌
- **说明**: 缓存控制，降低 API 成本
- **支持级别**:
  - 消息级别
  - 工具级别
  - 内容部分级别
- **格式**: `{"type": "ephemeral"}`
- **缓存有效期**: 5 分钟

**消息级别缓存**:
```json
{
  "messages": [
    {
      "role": "user",
      "content": "Long system context...",
      "cache_control": {"type": "ephemeral"}
    }
  ]
}
```

**工具级别缓存**:
```json
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

**内容部分级别缓存**:
```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Long context...",
          "cache_control": {"type": "ephemeral"}
        }
      ]
    }
  ]
}
```

**成本优化建议**:
- ✅ 长系统提示词（>1000 tokens）
- ✅ 大量工具定义（>10 个工具）
- ✅ 重复使用的上下文
- ❌ 短消息（<100 tokens）
- ❌ 频繁变化的内容

### reasoning_budget
- **类型**: `integer`
- **必需**: ❌
- **说明**: 推理预算 token 数（Anthropic Extended Thinking）
- **范围**: > 0
- **建议值**:
  - `1000-5000`: 简单推理
  - `10000-20000`: 中等复杂度
  - `30000-50000`: 深度推理

```json
{
  "reasoning_budget": 30000
}
```

### reasoning_effort
- **类型**: `string`
- **必需**: ❌
- **说明**: 推理强度级别
- **可选值**: `"low"`, `"medium"`, `"high"`
- **自动映射**:
  - `"low"` → 1000 tokens
  - `"medium"` → 20000 tokens
  - `"high"` → 50000 tokens

```json
{
  "reasoning_effort": "high"
}
```

**优先级**: `reasoning_budget` > `reasoning_effort`

**使用场景**:
- **low**: 简单问答、快速响应
- **medium**: 代码生成、数据分析
- **high**: 复杂推理、数学证明、深度分析

---

## 多模态参数

### modalities
- **类型**: `array`
- **必需**: ❌
- **说明**: 输出模态类型
- **可选值**: `"text"`, `"audio"`, `"image"`
- **默认值**: `["text"]`

```json
{
  "modalities": ["text", "audio"]
}
```

### audio
- **类型**: `object`
- **必需**: ❌（当 `modalities` 包含 `"audio"` 时必需）
- **说明**: 音频输出配置

**字段**:
- `voice`: 语音类型
  - 可选值: `"alloy"`, `"echo"`, `"fable"`, `"onyx"`, `"nova"`, `"shimmer"`
- `format`: 音频格式
  - 可选值: `"wav"`, `"mp3"`, `"pcm16"`

```json
{
  "modalities": ["text", "audio"],
  "audio": {
    "voice": "alloy",
    "format": "wav"
  }
}
```

### 图像输入
- **类型**: 消息内容部分
- **说明**: 在消息中包含图像

```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "What's in this image?"},
        {
          "type": "image_url",
          "image_url": {
            "url": "https://example.com/image.jpg",
            "detail": "high"
          }
        }
      ]
    }
  ]
}
```

**detail 参数**:
- `"auto"`: 自动选择（默认）
- `"low"`: 低分辨率（512x512）
- `"high"`: 高分辨率（2048x2048）

### 音频输入
- **类型**: 消息内容部分
- **说明**: 在消息中包含音频

```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "Transcribe this audio"},
        {
          "type": "input_audio",
          "input_audio": {
            "data": "base64_encoded_audio_data",
            "format": "wav"
          }
        }
      ]
    }
  ]
}
```

**支持的格式**: `"wav"`, `"mp3"`

---

## 工具和响应控制

### tools
- **类型**: `array`
- **必需**: ❌
- **说明**: 工具定义数组

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather information",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "City name"
            }
          },
          "required": ["location"]
        }
      }
    }
  ]
}
```

### tool_choice
- **类型**: `string` 或 `object`
- **必需**: ❌
- **说明**: 控制工具调用行为

**字符串值**:
- `"auto"`: 自动决定（默认）
- `"none"`: 不调用工具
- `"required"`: 必须调用工具

**对象值**（指定特定工具）:
```json
{
  "tool_choice": {
    "type": "function",
    "function": {"name": "get_weather"}
  }
}
```

### parallel_tool_calls
- **类型**: `boolean`
- **必需**: ❌
- **说明**: 是否允许并行调用多个工具
- **默认值**: `true`

```json
{
  "parallel_tool_calls": false
}
```

### response_format
- **类型**: `object`
- **必需**: ❌
- **说明**: 响应格式控制

**类型**:
- `"text"`: 纯文本（默认）
- `"json_object"`: JSON 对象
- `"json_schema"`: 符合指定 JSON Schema

**text 格式**:
```json
{
  "response_format": {"type": "text"}
}
```

**json_object 格式**:
```json
{
  "response_format": {"type": "json_object"}
}
```

**json_schema 格式**:
```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "user_info",
      "schema": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "age": {"type": "integer"}
        },
        "required": ["name", "age"]
      }
    }
  }
}
```

---

## 流式参数

### stream_options
- **类型**: `object`
- **必需**: ❌（仅在 `stream: true` 时有效）
- **说明**: 流式响应选项

**字段**:
- `include_usage`: 是否在流式响应中包含 token 使用统计

```json
{
  "stream": true,
  "stream_options": {
    "include_usage": true
  }
}
```

**使用场景**:
- ✅ 需要实时显示 token 使用情况
- ✅ 成本监控和预算控制
- ❌ 不关心使用统计（减少响应大小）

---

## 参数验证规则

### 网关不做入站参数校验

LLMux **不**校验也**不**钳制客户端传入的取值范围。`temperature`、`top_p`、`frequency_penalty`、
`presence_penalty`、`top_logprobs`、`max_tokens` 等参数原样透传给上游，非法值由上游返错。

原因：网关的职责是协议转换与选路，不是充当客户端的 linter；各家上游对同一参数的合法区间
并不一致（例如 `temperature` 上限有 1 也有 2），网关自建一套区间只会与上游产生二次冲突。

**例外**：确有必要的钳制是模型能力驱动的，不是通用区间校验——见 `MaxTokensLimit`
（关联级 max_tokens 上限）与 `ClampReasoningEffort`（按模型 `thinking_levels` 白名单钳制
推理档位）。这两处的依据是「该模型支持什么」，而非「该参数的通用合法范围」。

---


## 最佳实践

### 1. 参数组合建议

**代码生成**:
```json
{
  "temperature": 0.2,
  "top_p": 0.95,
  "frequency_penalty": 0.3,
  "presence_penalty": 0.0
}
```

**创意写作**:
```json
{
  "temperature": 1.2,
  "top_p": 0.9,
  "frequency_penalty": 0.5,
  "presence_penalty": 0.5
}
```

**数据提取**:
```json
{
  "temperature": 0.0,
  "response_format": {"type": "json_object"}
}
```

**对话助手**:
```json
{
  "temperature": 0.7,
  "top_p": 0.9,
  "presence_penalty": 0.3
}
```

### 2. 成本优化

**使用缓存控制**:
```json
{
  "messages": [
    {
      "role": "user",
      "content": "Long system prompt (>1000 tokens)...",
      "cache_control": {"type": "ephemeral"}
    }
  ]
}
```

**限制输出长度**:
```json
{
  "max_tokens": 500,
  "max_completion_tokens": 400
}
```

**使用流式响应监控**:
```json
{
  "stream": true,
  "stream_options": {"include_usage": true}
}
```

### 3. 性能优化

**并行工具调用**:
```json
{
  "tools": [...],
  "parallel_tool_calls": true
}
```

**合理的推理预算**:
```json
{
  "reasoning_effort": "medium"  // 而非总是 "high"
}
```

**避免过大的 max_tokens**:
```json
{
  "max_tokens": 1000  // 而非 4096
}
```

### 4. 安全性建议

**使用 user 标识**:
```json
{
  "user": "user-123"
}
```

**限制输出格式**:
```json
{
  "response_format": {"type": "json_schema", ...}
}
```

**设置停止序列**:
```json
{
  "stop": ["CONFIDENTIAL", "PRIVATE"]
}
```

### 5. 调试技巧

**启用 logprobs**:
```json
{
  "logprobs": true,
  "top_logprobs": 5
}
```

**使用 seed 重现问题**:
```json
{
  "seed": 12345
}
```

**添加 metadata**:
```json
{
  "metadata": {
    "debug_session": "test-001",
    "version": "1.0"
  }
}
```

---

## 参考资料

- [OpenAI API 文档](https://platform.openai.com/docs/api-reference)
- [Anthropic API 文档](https://docs.anthropic.com/claude/reference)
- [格式转换文档](FORMAT_CONVERSION.md)
- [统一请求扩展文档](UNIFIED_REQUEST_EXTENSION.md)

---

**最后更新**: 2026-02-26
