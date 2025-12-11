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

4. **转换管理器** (`service/transformer.go`)
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