# OpenAI 客户端 → Anthropic 供应商流式输出空白问题修复

## 问题描述

当客户端使用 OpenAI 格式发送流式请求,而后端连接到 Anthropic 类型的供应商时,流式输出会返回空白内容,但非流式输出正常。

### 复现步骤

```bash
# 流式请求(出现空白)
curl -X POST "http://localhost:7070/v1/chat/completions" \
  -H "Authorization: Bearer test123" \
  -H "Content-Type: application/json" \
  -d '{"model": "test", "messages": [{"role": "system", "content": "你是谁?"}], "stream": true}'

# 非流式请求(正常)
curl -X POST "http://localhost:7070/v1/chat/completions" \
  -H "Authorization: Bearer test123" \
  -H "Content-Type: application/json" \
  -d '{"model": "test", "messages": [{"role": "system", "content": "你是谁?"}], "stream": false}'
```

## 根本原因

在 `service/transform_openai.go` 的 `transformStreamResponseRealtime` 函数中,OpenAI → Anthropic 流式转换逻辑存在变量名重复问题:

```go
// 第 412-423 行:处理内容块
if content := getString(delta, "content"); content != "" {
    contentDelta := map[string]interface{}{
        "type":  "content_block_delta",
        "index": 0,
        "delta": map[string]interface{}{
            "type": "text_delta",
            "text": content,
        },
    }
    deltaData, _ := json.Marshal(contentDelta)  // ❌ 变量名 deltaData
    fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(deltaData))
}

// 第 442-457 行:处理结束消息
messageDelta := map[string]interface{}{
    "type": "message_delta",
    "delta": map[string]interface{}{
        "stop_reason": stopReason,
    },
}
if usage, ok := chunk["usage"].(map[string]interface{}); ok {
    messageDelta["usage"] = map[string]interface{}{
        "input_tokens":  int(getFloat(usage, "prompt_tokens")),
        "output_tokens": int(getFloat(usage, "completion_tokens")),
    }
}
deltaData, _ := json.Marshal(messageDelta)  // ❌ 变量名重复
fmt.Fprintf(pw, "event: message_delta\ndata: %s\n\n", string(deltaData))
```

两个不同的代码块都使用了 `deltaData` 变量名,导致后面的赋值覆盖了前面的值,使得 `content_block_delta` 事件发送的数据错误,导致客户端无法正确解析流式响应内容。

## 解决方案

修改 `service/transform_openai.go`,为每个数据块使用独立的变量名:

### 修改内容

**文件**: `service/transform_openai.go`

**行号**: 412-457

```go
// 处理内容块 - 使用 contentDeltaData
if content := getString(delta, "content"); content != "" {
    contentDelta := map[string]interface{}{
        "type":  "content_block_delta",
        "index": 0,
        "delta": map[string]interface{}{
            "type": "text_delta",
            "text": content,
        },
    }
    contentDeltaData, _ := json.Marshal(contentDelta)  // ✅ 使用独立变量名
    fmt.Fprintf(pw, "event: content_block_delta\ndata: %s\n\n", string(contentDeltaData))
}

// 处理结束消息 - 使用 messageDeltaData
messageDelta := map[string]interface{}{
    "type": "message_delta",
    "delta": map[string]interface{}{
        "stop_reason": stopReason,
    },
}
if usage, ok := chunk["usage"].(map[string]interface{}); ok {
    messageDelta["usage"] = map[string]interface{}{
        "input_tokens":  int(getFloat(usage, "prompt_tokens")),
        "output_tokens": int(getFloat(usage, "completion_tokens")),
    }
}
messageDeltaData, _ := json.Marshal(messageDelta)  // ✅ 使用独立变量名
fmt.Fprintf(pw, "event: message_delta\ndata: %s\n\n", string(messageDeltaData))
```

## 测试验证

修复后,流式输出应该能正常返回内容:

```bash
# 测试流式输出
curl -N -X POST "http://localhost:7070/v1/chat/completions" \
  -H "Authorization: Bearer test123" \
  -H "Content-Type: application/json" \
  -d '{"model": "test", "messages": [{"role": "user", "content": "你好"}], "stream": true}'

# 期望输出(SSE格式):
# data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":...,"model":"claude","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}
# data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":...,"model":"claude","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}
# ...
# data: [DONE]
```

## 影响范围

- **受影响场景**: OpenAI 客户端 → Anthropic 供应商的流式请求
- **不受影响**: 
  - 非流式请求
  - OpenAI → OpenAI 转换
  - Anthropic → Anthropic 转换
  - Anthropic → OpenAI 转换(未受此 bug 影响)

## 相关文件

- `service/transform_openai.go` - OpenAI 格式转换逻辑
- `service/transformer.go` - 转换管理器
- `service/chat.go` - 聊天服务主逻辑

## 修复日期

2025-12-12

## 修复版本

当前版本(commit hash: 待补充)