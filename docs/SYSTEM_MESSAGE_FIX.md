# System 消息和 max_tokens 修复文档

## 问题描述

在 OpenAI 客户端 → Anthropic 供应商的转换时,某些提供商的模型会报错：

```
错误信息status: 422, body: {"detail":[{"ctx":{"expected":"'user' or 'assistant'"},"input":"system","loc":["body","messages",0,"role"],"msg":"Input should be 'user' or 'assistant'","type":"literal_error"}]}
```

以及:

```
错误信息: {"detail":"'>' not supported between instances of 'int' and 'NoneType'"}
```

## 根本原因

### 问题 1: System 消息角色错误
Anthropic API 不接受 messages 数组中的 "system" 角色消息,system 消息应该放在单独的 `system` 字段中。

### 问题 2: 缺少 max_tokens 参数
Anthropic API 要求必须提供 `max_tokens` 参数,否则会报错。

### 问题 3: 格式转换条件
在 [`service/chat.go:102`](service/chat.go:102) 中,只有当 `style != provider.Type` 时才会进行格式转换。这导致:
- ✅ OpenAI 客户端 → Anthropic 供应商:会转换,修复生效
- ❌ Anthropic 客户端 → Anthropic 供应商:不转换,修复不生效

## 修复方案

### 已完成的修复

#### 1. TransformAnthropicToUnified - 提取 system 消息
在 [`service/transform_anthropic.go:63-127`](service/transform_anthropic.go:63),修复将 system 角色的消息从 messages 数组中移除,放入 unified.System 字段:

```go
// 转换消息
messages := []interface{}{}
for _, msg := range unified.Messages {
    // Anthropic 格式只接受 user 和 assistant 角色
    if msg.Role == "system" {
        // 将 system 消息内容合并到 system 字段
        if msg.Content != nil {
            if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
                if existing, ok := req["system"].(string); ok && existing != "" {
                    req["system"] = existing + "\n\n" + contentStr
                } else {
                    req["system"] = contentStr
                }
            }
        }
        continue // 跳过此消息
    }
    // ... 处理其他消息
}
```

#### 2. TransformOpenAIToUnified - 提取 system 消息
在 [`service/transform_openai.go:14-66`](service/transform_openai.go:14),同样提取 system 消息:

```go
// 转换消息
if messages, ok := req["messages"].([]interface{}); ok {
    for _, msg := range messages {
        msgMap := msg.(map[string]interface{})
        role := getString(msgMap, "role")
        
        // 提取 system 消息到单独的字段
        if role == "system" {
            if content, ok := msgMap["content"].(string); ok && content != "" {
                if unified.System != "" {
                    unified.System += "\n\n" + content
                } else {
                    unified.System = content
                }
            }
            continue // 不将 system 消息添加到 messages 数组
        }
        // ... 处理其他消息
    }
}
```

#### 3. TransformUnifiedToOpenAI - 恢复 system 消息
在 [`service/transform_openai.go:68-134`](service/transform_openai.go:68),将 unified.System 转回第一条消息:

```go
// 如果有 system 字段,添加为第一条消息
if unified.System != "" {
    messages = append(messages, map[string]interface{}{
        "role":    "system",
        "content": unified.System,
    })
}
```

#### 4. TransformUnifiedToAnthropic - 添加默认 max_tokens
在 [`service/transform_anthropic.go:69-75`](service/transform_anthropic.go:69),确保总是有 max_tokens:

```go
// Anthropic 格式要求必须有 max_tokens,如果没有设置则使用默认值
if unified.MaxTokens > 0 {
    req["max_tokens"] = unified.MaxTokens
} else {
    req["max_tokens"] = 8192 // 设置默认值
}
```

### 待修复问题

当前修复只在客户端格式与供应商格式不同时生效。对于 Anthropic 客户端 → Anthropic 供应商的情况,需要额外处理。

**解决方案选项:**

#### 方案 A: 统一使用格式转换(推荐)
即使客户端格式与供应商格式相同,也先转换为统一格式再转回去:

```go
// service/chat.go:100-111
// 格式转换:总是转换为统一格式以确保数据标准化
requestBody := before.raw
tm := NewTransformerManager(style, provider.Type)
convertedBody, err := tm.ProcessRequest(ctx, before.raw)
if err != nil {
    retryLog <- log.WithError(fmt.Errorf("transform request error: %v", err))
    delete(weightItems, *id)
    continue
}
requestBody = convertedBody
```

#### 方案 B: 在 Anthropic beforer 中预处理
在 `BeforerAnthropic` 函数中检查并提取 system 消息,确保 max_tokens:

```go
// service/before.go
func BeforerAnthropic(rawBody []byte) (*Before, error) {
    // ... 现有代码
    
    // 确保 max_tokens
    if before.MaxTokens == 0 {
        before.MaxTokens = 4096
    }
    
    // 提取 system 消息
    // ... 处理逻辑
}
```

## 测试

### 测试命令
```bash
curl -X POST "http://localhost:7070/v1/messages" \
  -H "anthropic-version: 2023-06-01" \
  -H "Authorization: Bearer test123" \
  -H "Content-Type: application/json" \
  -d '{"model":"test","messages":[{"role":"user","content":"你是?"}],"stream":true}'
```

### 预期结果
- ✅ 不应该出现 422 错误(system 角色)
- ✅ 不应该出现 NoneType 错误(max_tokens)
- ✅ 应该正常返回流式响应

## 相关文件

- [`service/transform_anthropic.go`](service/transform_anthropic.go) - Anthropic 格式转换
- [`service/transform_openai.go`](service/transform_openai.go) - OpenAI 格式转换
- [`service/transformer.go`](service/transformer.go) - 转换管理器
- [`service/chat.go`](service/chat.go) - 聊天处理主逻辑
- [`handler/chat.go`](handler/chat.go) - HTTP 处理器

## 更新日志

- 2025-12-12: 修复 system 消息处理和 max_tokens 默认值
- 待定: 修复相同格式不转换的问题