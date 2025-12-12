# 流式响应转换修复

## 问题描述

当 OpenAI 客户端通过 LLMIO 访问 Anthropic 供应商时，流式请求无法正常工作。客户端无法实时接收数据，必须等待整个响应完成后才能收到内容。

### 问题表现

- **OpenAI 客户端 → Anthropic 供应商 (流式)**：客户端无法接收响应
- **curl 命令 (非流式)**：正常工作
- **直接 curl 上游 (流式)**：正常工作

### 问题日志

客户端请求：
```json
{
  "model": "test",
  "messages": [
    {
      "role": "system",
      "content": "..."
    },
    {
      "role": "user",
      "content": "你是谁？"
    }
  ],
  "stream": true,
  "temperature": 0.6,
  "top_p": 1,
  "stream_options": {
    "include_usage": true
  }
}
```

响应输出：暂无数据（客户端一直等待）

## 根本原因

在 `service/transform_openai.go:173-194` 的 `TransformProviderResponse` 函数中：

1. 使用 `io.ReadAll(response.Body)` 读取整个响应体到内存
2. 对于流式响应，这会等待所有数据传输完毕才开始处理
3. 破坏了流式传输的实时性，导致客户端无法实时接收数据

### 问题代码

```go
// TransformProviderResponse 转换供应商响应
func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
    if providerType == clientType {
        return response, nil
    }

    // ❌ 问题：先读取完整响应体
    body, err := io.ReadAll(response.Body)
    if err != nil {
        return nil, err
    }
    response.Body.Close()

    // 检查是否是流式响应
    contentType := response.Header.Get("Content-Type")
    isStream := strings.Contains(contentType, "text/event-stream")

    if isStream {
        // ❌ 此时整个响应已经被读取到内存
        return transformStreamResponse(response, body, providerType, clientType)
    }

    return transformNonStreamResponse(response, body, providerType, clientType)
}
```

## 解决方案

### 修改策略

1. **先判断是否为流式响应**：根据 `Content-Type` 头判断
2. **流式响应**：使用新函数 `transformStreamResponseRealtime` 直接从 `response.Body` 读取器进行实时转换
3. **非流式响应**：继续使用原有的 `io.ReadAll` 方式

### 修改后的代码

```go
// TransformProviderResponse 转换供应商响应
func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
    if providerType == clientType {
        return response, nil
    }

    // ✅ 先检查是否是流式响应
    contentType := response.Header.Get("Content-Type")
    isStream := strings.Contains(contentType, "text/event-stream")

    if isStream {
        // ✅ 流式响应：直接从 Body 读取器进行实时转换
        return transformStreamResponseRealtime(response, providerType, clientType)
    }

    // ✅ 非流式响应：读取完整响应体后转换
    body, err := io.ReadAll(response.Body)
    if err != nil {
        return nil, err
    }
    response.Body.Close()

    return transformNonStreamResponse(response, body, providerType, clientType)
}
```

### 新增函数：transformStreamResponseRealtime

该函数实现了真正的实时流式转换：

```go
func transformStreamResponseRealtime(response *http.Response, providerType, clientType string) (*http.Response, error) {
    pr, pw := io.Pipe()

    go func() {
        defer pw.Close()
        defer response.Body.Close()

        // ✅ 使用 Scanner 逐行读取，实时处理
        scanner := bufio.NewScanner(response.Body)
        scanner.Buffer(make([]byte, 0, 8192), 1024*1024)

        for scanner.Scan() {
            line := scanner.Text()
            // 实时解析和转换每一行 SSE 数据
            // ...
        }
    }()

    return &http.Response{
        // ...
        Body: pr,  // ✅ 返回 Pipe Reader，支持流式读取
        ContentLength: -1,
    }, nil
}
```

## 关键改进

1. **实时性**：不再等待整个响应完成，而是逐行实时处理和转发
2. **低延迟**：首字延迟显著降低，客户端可以立即开始接收数据
3. **内存效率**：不需要将整个响应缓存到内存
4. **向下兼容**：保留了原有的 `transformStreamResponse` 函数用于兼容

## 测试验证

### 测试场景

1. **OpenAI 客户端 → Anthropic 供应商 (流式)** ✅
2. **OpenAI 客户端 → OpenAI 供应商 (流式)** ✅
3. **Anthropic 客户端 → OpenAI 供应商 (流式)** ✅
4. **非流式请求** ✅

### 预期结果

- 客户端可以实时接收流式响应
- 首字延迟显著降低
- 响应格式正确转换（Anthropic ↔ OpenAI）

## 相关文件

- `service/transform_openai.go` - 主要修改文件
- `service/transformer.go` - 转换管理器
- `service/chat.go` - 聊天服务调用

## 后续优化建议

1. 添加流式转换的单元测试
2. 监控流式请求的性能指标（首字延迟、吞吐量）
3. 考虑添加流式转换的错误恢复机制
4. 优化 Scanner 的缓冲区大小以适应不同场景

## 修复日期

2025-12-12

## 修复人员

Claude Code Assistant