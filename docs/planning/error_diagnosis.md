# 错误诊断报告：field messages is required

## 错误信息
```json
{
  "error": {
    "message": "field messages is required (request id: 20260211050019275368962Od73SarW)",
    "type": "new_api_error",
    "param": "",
    "code": "invalid_request"
  }
}
```

## 根本原因分析

### 问题定位
错误消息 `"field messages is required"` 表明请求到达了一个 **Chat Completions API 端点**（需要 `messages` 字段），而不是 **Responses API 端点**（需要 `input` 字段）。

### 错误场景
最可能的场景是：

1. **客户端**：发送 Responses 格式请求到 `/v1/responses`
2. **llmio 识别**：`style = "openai-res"`（通过 `BeforerOpenAIRes` 解析）
3. **提供商配置**：提供商类型也是 `openai-res`
4. **直接透传**：由于 `style == provider.Type`，系统执行直接透传（`service/chat.go:113-134`）
   ```go
   if style == provider.Type {
       // 直接透传，不进行格式转换
       requestBody = before.raw
   }
   ```
5. **发送请求**：`OpenAIRes.BuildReq` 将请求发送到 `{baseURL}/responses`
6. **上游错误**：上游服务器不支持 `/responses` 端点，返回错误

### 证据支持

1. **错误类型不匹配**：
   - 错误类型 `"new_api_error"` 不是标准 OpenAI 错误类型
   - 标准 OpenAI 使用 `"invalid_request_error"` 等类型
   - 这表明上游是第三方 API 网关（可能是中国云服务商）

2. **请求 ID 格式**：
   - `20260211050019275368962Od73SarW` 的格式不是标准 OpenAI 格式
   - 进一步证实上游是第三方服务

3. **字段要求不匹配**：
   - Responses API 要求 `input` 字段
   - Chat Completions API 要求 `messages` 字段
   - 错误要求 `messages`，说明上游期望 Chat Completions 格式

## 代码路径追踪

### 请求流程
```
客户端 Responses 请求
  ↓
handler/chat.go:ResponsesHandler
  ↓
service/chat.go:BalanceChat (style="openai-res")
  ↓
service/before.go:BeforerOpenAIRes (解析请求，保存 raw)
  ↓
service/chat.go:113-134 (style == provider.Type，直接透传)
  ↓
providers/openai_res.go:BuildReq (发送到 {baseURL}/responses)
  ↓
上游 API (不支持 /responses，返回错误)
```

### 关键代码位置

1. **直接透传逻辑** (`service/chat.go:113-134`)：
   ```go
   if style == provider.Type {
       // 直接透传，不进行格式转换
       requestBody = before.raw
   } else if !enableFormatConversion {
       // 格式转换已关闭，跳过此供应商
       continue
   } else {
       // 需要格式转换
       tm := NewTransformerManager(style, provider.Type)
       convertedBody, err := tm.ProcessRequest(ctx, before.raw)
       requestBody = convertedBody
   }
   ```

2. **OpenAIRes 提供商** (`providers/openai_res.go:23-28`)：
   ```go
   func (o *OpenAIRes) BuildReq(ctx context.Context, header http.Header, model string, rawBody []byte) (*http.Request, error) {
       body, err := sjson.SetBytes(rawBody, "model", model)
       if err != nil {
           return nil, err
       }
       req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/responses", o.BaseURL), bytes.NewReader(body))
       // ...
   }
   ```

## 解决方案

### 方案 1：配置修复（推荐）
**将提供商类型改为 `openai`**

如果上游 API 实际上是 Chat Completions 兼容的（支持 `/chat/completions` 端点），应该：

1. 在数据库中将提供商的 `type` 字段从 `openai-res` 改为 `openai`
2. 系统会自动进行格式转换：Responses → Chat Completions
3. 请求会被发送到 `{baseURL}/chat/completions`

**操作步骤**：
```sql
UPDATE providers
SET type = 'openai'
WHERE type = 'openai-res'
  AND base_url NOT LIKE '%/responses%';
```

### 方案 2：代码增强（防御性编程）
**在 `OpenAIRes.BuildReq` 中添加验证和降级逻辑**

```go
func (o *OpenAIRes) BuildReq(ctx context.Context, header http.Header, model string, rawBody []byte) (*http.Request, error) {
    body, err := sjson.SetBytes(rawBody, "model", model)
    if err != nil {
        return nil, err
    }

    // 验证请求体是否包含 input 字段
    if !gjson.GetBytes(body, "input").Exists() {
        return nil, fmt.Errorf("openai-res provider requires 'input' field in request body")
    }

    req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/responses", o.BaseURL), bytes.NewReader(body))
    // ...
}
```

### 方案 3：文档说明
**在文档中明确说明 `openai-res` 的使用限制**

在 `docs/FORMAT_CONVERSION.md` 中添加：

```markdown
## 提供商类型选择指南

### openai-res 类型
- **仅用于**：真正支持 OpenAI Responses API 的提供商
- **端点要求**：必须支持 `/v1/responses` 或 `/responses` 路径
- **字段要求**：请求必须包含 `input` 字段（而非 `messages`）
- **典型提供商**：OpenAI 官方 API（需要 API 版本支持）

### openai 类型
- **用于**：支持 OpenAI Chat Completions API 的提供商
- **端点要求**：必须支持 `/v1/chat/completions` 或 `/chat/completions` 路径
- **字段要求**：请求必须包含 `messages` 字段
- **典型提供商**：OpenAI、Azure OpenAI、大多数第三方 API 网关

### 如何判断
如果不确定上游 API 支持哪种格式，建议：
1. 优先使用 `openai` 类型（Chat Completions）
2. 启用格式转换功能（默认启用）
3. llmio 会自动处理 Responses ↔ Chat Completions 的转换
```

## 验证步骤

### 1. 检查提供商配置
```bash
# 查看当前配置为 openai-res 的提供商
sqlite3 llmio.db "SELECT id, name, type, base_url FROM providers WHERE type = 'openai-res';"
```

### 2. 测试 Chat Completions 端点
```bash
curl -X POST {baseURL}/chat/completions \
  -H "Authorization: Bearer {api_key}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### 3. 测试 Responses 端点
```bash
curl -X POST {baseURL}/responses \
  -H "Authorization: Bearer {api_key}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "input": "Hello"
  }'
```

如果 Responses 端点返回 404 或类似错误，说明上游不支持 Responses API，应使用方案 1 修复配置。

## 后续改进建议

### 1. 提供商健康检查增强
在 `service/health.go` 中添加端点兼容性检查：
- 检测提供商是否真正支持其声明的 API 类型
- 自动标记配置错误的提供商

### 2. 错误消息改进
在 `service/chat.go` 中添加更友好的错误提示：
```go
if strings.Contains(errMsg, "field messages is required") && provider.Type == "openai-res" {
    return fmt.Errorf("provider %s is configured as openai-res but upstream requires Chat Completions format. Please change provider type to 'openai'", provider.Name)
}
```

### 3. 配置验证
在提供商创建/更新时验证配置一致性：
- 如果 `type = "openai-res"`，警告用户确认上游支持 Responses API
- 提供自动检测功能

## 总结

**问题**：提供商配置为 `openai-res` 但上游 API 不支持 Responses API

**影响**：所有使用该提供商的 Responses 格式请求都会失败

**修复**：将提供商类型改为 `openai`，启用自动格式转换

**预防**：添加配置验证和健康检查，改进错误提示
