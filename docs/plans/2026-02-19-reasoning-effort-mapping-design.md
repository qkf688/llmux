# reasoning_effort 参数兼容处理设计

## 概述

为 OpenAI 和 Responses API 请求转换链路添加 `reasoning_effort` 参数的自动映射与规范化功能，解决客户端发送非标准值（如 `minimal`）导致上游服务器（如 SGLang）报错的问题。

## 背景

### 问题描述

客户端（如 Cherry Studio）发送 `reasoning_effort: 'minimal'` 参数时，SGLang 服务器返回 400 错误：

```
status: 400, body: {"error":{"code":"bad_response_status_code","message":"1 validation error:\n  {'type': 'literal_error', 'loc': ('body', 'reasoning_effort'), 'msg': \"Input should be 'low', 'medium' or 'high'\", 'input': 'minimal', 'ctx': {'expected': \"'low', 'medium' or 'high'\"}}\n\n  File \"/workspace/sglang/python/sglang/srt/entrypoints/http_server.py\", line 1279, in openai_v1_chat_completions\n    POST /v1/chat/completions [{'type': 'literal_error', 'loc': ('body', 'reasoning_effort'), 'msg': \"Input should be 'low', 'medium' or 'high'\", 'input': 'minimal', 'ctx': {'expected': \"'low', 'medium' or 'high'\"}}]","param":"","type":"bad_response_status_code"}}
```

### 根本原因

1. LLMIO 直接透传所有参数，未对 `reasoning_effort` 进行验证或规范化
2. 不同客户端可能发送不同的值（`minimal`、`low`、`medium`、`high` 等）
3. 上游服务器（如 SGLang）只接受标准值：`low`、`medium`、`high`

## 设计方案

### 方案选择

采用**方案 A：在统一格式中添加 ReasoningEffort 字段**

**理由**：
- 符合现有架构模式（统一格式作为中间层）
- 逻辑集中，易于维护和测试
- 便于后续扩展其他参数处理
- 测试友好

### 架构设计

```
客户端请求 → TransformXXXToUnified → UnifiedRequest (规范化) → TransformUnifiedToXXX → 上游服务器
```

## 详细设计

### 1. 数据结构修改

#### 1.1 UnifiedRequest 结构体

**文件**: `service/transformer.go`

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
    ReasoningEffort *string          `json:"reasoning_effort,omitempty"` // 新增
}
```

**设计说明**：
- 使用指针 `*string` 区分"未设置"（nil）和"空字符串"（""）
- 只有客户端明确发送该参数时才处理

#### 1.2 数据库配置项

**文件**: `models/model.go`

```go
const (
    // ... 现有常量 ...

    // reasoning_effort 参数映射相关设置
    SettingKeyReasoningEffortMappingEnabled = "reasoning_effort_mapping_enabled" // 是否启用映射
    SettingKeyReasoningEffortDefaultValue   = "reasoning_effort_default_value"   // 默认值（low/medium/high）
)
```

### 2. 转换逻辑

#### 2.1 规范化函数

**文件**: `service/transform_openai.go`

```go
// normalizeReasoningEffort 规范化 reasoning_effort 参数
// 有效值: low, medium, high
// 映射: minimal -> low
// 无效值 -> 使用配置的默认值（默认 low）
func normalizeReasoningEffort(ctx context.Context, value string) string {
    switch strings.ToLower(value) {
    case "low", "medium", "high":
        return strings.ToLower(value)
    case "minimal":
        return "low"
    default:
        // 获取配置的默认值
        defaultValue := getReasoningEffortDefaultValue(ctx)
        if defaultValue != "" {
            return defaultValue
        }
        return "low"
    }
}

// getReasoningEffortDefaultValue 获取默认值配置
func getReasoningEffortDefaultValue(ctx context.Context) string {
    setting, err := gorm.G[models.Setting](models.DB).
        Where("key = ?", models.SettingKeyReasoningEffortDefaultValue).
        First(ctx)
    if err != nil {
        return "low"
    }
    return setting.Value
}

// getReasoningEffortMappingEnabled 获取映射开关
func getReasoningEffortMappingEnabled(ctx context.Context) bool {
    setting, err := gorm.G[models.Setting](models.DB).
        Where("key = ?", models.SettingKeyReasoningEffortMappingEnabled).
        First(ctx)
    if err != nil {
        return true // 默认启用
    }
    return setting.Value == "true"
}
```

#### 2.2 读取阶段（客户端 → 统一格式）

**文件**: `service/transform_openai.go` - `TransformOpenAIToUnified`

```go
// 在 TransformOpenAIToUnified 函数中添加
if effort, ok := req["reasoning_effort"].(string); ok && effort != "" {
    // 检查是否启用映射
    if getReasoningEffortMappingEnabled(ctx) {
        normalized := normalizeReasoningEffort(ctx, effort)
        unified.ReasoningEffort = &normalized
    } else {
        // 不启用映射时直接透传
        unified.ReasoningEffort = &effort
    }
}
```

**文件**: `service/transform_responses.go` - `TransformResponsesToUnified`

```go
// 在 TransformResponsesToUnified 函数中添加
if effort, ok := req["reasoning_effort"].(string); ok && effort != "" {
    if getReasoningEffortMappingEnabled(ctx) {
        normalized := normalizeReasoningEffort(ctx, effort)
        unified.ReasoningEffort = &normalized
    } else {
        unified.ReasoningEffort = &effort
    }
}
```

#### 2.3 输出阶段（统一格式 → 供应商格式）

**文件**: `service/transform_openai.go` - `TransformUnifiedToOpenAI`

```go
// 在 TransformUnifiedToOpenAI 函数中添加
if unified.ReasoningEffort != nil {
    req["reasoning_effort"] = *unified.ReasoningEffort
}
```

**文件**: `service/transform_responses.go` - `TransformUnifiedToResponses`

```go
// 在 TransformUnifiedToResponses 函数中添加
if unified.ReasoningEffort != nil {
    req["reasoning_effort"] = *unified.ReasoningEffort
}
```

**注意**: `TransformUnifiedToAnthropic` 中**不输出**该参数，因为 Anthropic API 不支持此字段。

#### 2.4 函数签名修改

需要为转换函数添加 `context.Context` 参数以访问数据库配置：

```go
// 修改前
func TransformOpenAIToUnified(rawBody []byte) (*UnifiedRequest, error)

// 修改后
func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error)
```

同样修改：
- `TransformResponsesToUnified`
- 以及调用这些函数的 `TransformerManager.ProcessRequest`

### 3. 后端 API 修改

#### 3.1 SettingsResponse 结构体

**文件**: `handler/api.go`

```go
type SettingsResponse struct {
    // ... 现有字段 ...

    // reasoning_effort 映射相关设置
    ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
    ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"` // low/medium/high
}
```

#### 3.2 UpdateSettingsRequest 结构体

```go
type UpdateSettingsRequest struct {
    // ... 现有字段 ...

    // reasoning_effort 映射相关设置
    ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
    ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"`
}
```

#### 3.3 GetSettings 函数

```go
// 在 GetSettings 函数中添加默认值
response := SettingsResponse{
    // ... 现有默认值 ...

    ReasoningEffortMappingEnabled: true,    // 默认启用
    ReasoningEffortDefaultValue:   "low",   // 默认值为 low
}

// 在 switch 中添加读取逻辑
case models.SettingKeyReasoningEffortMappingEnabled:
    response.ReasoningEffortMappingEnabled = setting.Value == "true"
case models.SettingKeyReasoningEffortDefaultValue:
    response.ReasoningEffortDefaultValue = setting.Value
```

#### 3.4 UpdateSettings 函数

```go
// 更新 reasoning_effort 映射开关
reasoningEffortMappingEnabledValue := "false"
if req.ReasoningEffortMappingEnabled {
    reasoningEffortMappingEnabledValue = "true"
}
if _, err := gorm.G[models.Setting](models.DB).
    Where("key = ?", models.SettingKeyReasoningEffortMappingEnabled).
    Update(ctx, "value", reasoningEffortMappingEnabledValue); err != nil {
    common.InternalServerError(c, "Failed to update settings: "+err.Error())
    return
}

// 更新 reasoning_effort 默认值（验证合法性）
if req.ReasoningEffortDefaultValue != "low" &&
   req.ReasoningEffortDefaultValue != "medium" &&
   req.ReasoningEffortDefaultValue != "high" {
    req.ReasoningEffortDefaultValue = "low"
}
if _, err := gorm.G[models.Setting](models.DB).
    Where("key = ?", models.SettingKeyReasoningEffortDefaultValue).
    Update(ctx, "value", req.ReasoningEffortDefaultValue); err != nil {
    common.InternalServerError(c, "Failed to update settings: "+err.Error())
    return
}
```

### 4. 前端 UI 设计

#### 4.1 类型定义

**文件**: `webui/src/lib/api.ts`

```typescript
export interface Settings {
  // ... 现有字段 ...

  reasoning_effort_mapping_enabled: boolean;
  reasoning_effort_default_value: 'low' | 'medium' | 'high';
}

export interface UpdateSettingsRequest {
  // ... 现有字段 ...

  reasoning_effort_mapping_enabled?: boolean;
  reasoning_effort_default_value?: string;
}
```

#### 4.2 UI 组件

**文件**: `webui/src/routes/settings/routing-settings.tsx`

**位置**: 在"格式转换" Card 之后添加新的 Card

```tsx
<Card>
  <CardHeader>
    <CardTitle>参数映射</CardTitle>
    <CardDescription>配置请求参数的自动映射与规范化</CardDescription>
  </CardHeader>
  <CardContent className="space-y-4">
    {/* 启用开关 */}
    <div className="flex items-center justify-between">
      <div className="space-y-0.5">
        <Label>reasoning_effort 参数映射</Label>
        <p className="text-sm text-muted-foreground">
          自动将 'minimal' 映射为有效值，处理无效参数
        </p>
      </div>
      <Switch
        checked={localSettings?.reasoning_effort_mapping_enabled ?? true}
        onCheckedChange={(checked) => {
          if (localSettings) {
            setLocalSettings({
              ...localSettings,
              reasoning_effort_mapping_enabled: checked
            });
            setHasChanges(true);
          }
        }}
      />
    </div>

    {/* 默认值选择 */}
    {localSettings?.reasoning_effort_mapping_enabled && (
      <div className="space-y-2">
        <Label>默认值</Label>
        <Select
          value={localSettings.reasoning_effort_default_value}
          onValueChange={(value) => {
            if (localSettings) {
              setLocalSettings({
                ...localSettings,
                reasoning_effort_default_value: value
              });
              setHasChanges(true);
            }
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="low">low（低推理强度）</SelectItem>
            <SelectItem value="medium">medium（中等推理强度）</SelectItem>
            <SelectItem value="high">high（高推理强度）</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-sm text-muted-foreground">
          无效参数将被映射为此默认值
        </p>
      </div>
    )}
  </CardContent>
</Card>
```

## 边界行为

### 参数映射规则

| 输入值 | 映射开关开启 | 映射开关关闭 | 说明 |
|--------|-------------|-------------|------|
| `low` | `low` | `low` | 有效值，保持不变 |
| `medium` | `medium` | `medium` | 有效值，保持不变 |
| `high` | `high` | `high` | 有效值，保持不变 |
| `minimal` | `low` | `minimal` | 映射为 low / 直接透传 |
| 其他无效值 | 配置的默认值 | 原值透传 | 使用前端设置的默认值 / 直接透传 |
| 未传递 | 不添加字段 | 不添加字段 | 不影响现有请求 |

### 配置异常处理

1. **数据库配置读取失败**：
   - `reasoning_effort_mapping_enabled` 默认为 `true`（启用）
   - `reasoning_effort_default_value` 默认为 `"low"`

2. **前端设置非法默认值**：
   - 后端验证，非 `low/medium/high` 时强制设为 `low`

3. **映射开关关闭**：
   - 直接透传原值，不做任何处理
   - 保留现有兼容性

## 用户可见变化

### 前端设置页面

1. **新增"参数映射"配置区块**（位于"通用设置"页面）
2. **新增开关**：`reasoning_effort 参数映射`
   - 默认开启
   - 说明：自动将 'minimal' 映射为有效值，处理无效参数
3. **新增下拉选择**：`默认值`
   - 选项：low（低推理强度）/ medium（中等推理强度）/ high（高推理强度）
   - 默认值：low
   - 说明：无效参数将被映射为此默认值

### 行为变化

- **开启映射后**：
  - `minimal` 自动转换为 `low`
  - 其他非法值转换为配置的默认值
  - 避免 SGLang 等服务器返回 400 错误

- **关闭映射后**：
  - 所有参数直接透传，保持原有行为

## 影响范围

### 修改文件

**后端**：
- `models/model.go` - 添加配置常量
- `service/transformer.go` - 添加 `ReasoningEffort` 字段
- `service/transform_openai.go` - 添加规范化逻辑和辅助函数
- `service/transform_responses.go` - 添加规范化逻辑
- `handler/api.go` - 添加设置接口字段和处理逻辑

**前端**：
- `webui/src/lib/api.ts` - 添加类型定义
- `webui/src/routes/settings/routing-settings.tsx` - 添加 UI 组件

### 不影响

- 现有 API 端点
- 数据库 schema（使用现有 `settings` 表）
- Anthropic 转换逻辑（不输出该参数）
- 其他参数处理

## 实施步骤

1. **后端数据结构**：
   - 修改 `models/model.go` 添加配置常量
   - 修改 `service/transformer.go` 添加 `ReasoningEffort` 字段

2. **后端转换逻辑**：
   - 在 `service/transform_openai.go` 添加规范化函数和辅助函数
   - 修改 `TransformOpenAIToUnified` 和 `TransformUnifiedToOpenAI`
   - 修改 `service/transform_responses.go` 的对应函数
   - 修改函数签名添加 `context.Context` 参数

3. **后端 API 接口**：
   - 修改 `handler/api.go` 的 `SettingsResponse` 和 `UpdateSettingsRequest`
   - 修改 `GetSettings` 和 `UpdateSettings` 函数

4. **前端类型定义**：
   - 修改 `webui/src/lib/api.ts` 添加类型

5. **前端 UI 组件**：
   - 修改 `webui/src/routes/settings/routing-settings.tsx` 添加新 Card

6. **测试验证**：
   - 单元测试：规范化函数
   - 集成测试：完整转换流程
   - 手动测试：前端设置页面和实际请求

## 测试策略

### 单元测试

**文件**: `service/transform_openai_test.go`

```go
func TestNormalizeReasoningEffort(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid low", "low", "low"},
        {"valid medium", "medium", "medium"},
        {"valid high", "high", "high"},
        {"minimal maps to low", "minimal", "low"},
        {"invalid maps to default", "invalid", "low"},
        {"empty maps to default", "", "low"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := normalizeReasoningEffort(context.Background(), tt.input)
            if result != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

### 集成测试

1. **OpenAI → OpenAI**（透传）
   - 输入：`reasoning_effort: "minimal"`
   - 输出：`reasoning_effort: "low"`

2. **OpenAI → Anthropic**（不输出）
   - 输入：`reasoning_effort: "minimal"`
   - 输出：无 `reasoning_effort` 字段

3. **Responses → OpenAI**
   - 输入：`reasoning_effort: "invalid"`
   - 输出：`reasoning_effort: "low"`（默认值）

### 手动测试

1. **前端设置页面**：
   - 验证新 Card 显示正常
   - 验证开关和下拉选择功能正常
   - 验证保存和重置功能

2. **实际请求测试**：
   - 使用 Cherry Studio 发送 `reasoning_effort: "minimal"`
   - 验证 SGLang 不再返回 400 错误
   - 验证日志中参数已被规范化

## 验收标准

- [ ] 后端数据结构修改完成
- [ ] 后端转换逻辑实现完成
- [ ] 后端 API 接口修改完成
- [ ] 前端类型定义添加完成
- [ ] 前端 UI 组件实现完成
- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 手动测试通过
- [ ] `minimal` 正确映射为 `low`
- [ ] 无效值正确映射为配置的默认值
- [ ] 映射开关关闭时直接透传
- [ ] Anthropic 转换不输出该参数
- [ ] 前端设置页面功能正常

## 风险评估

**风险等级**: 低

**理由**：
1. 只添加新功能，不修改现有逻辑
2. 默认启用映射，向后兼容
3. 可通过开关关闭，回退到原有行为
4. 不影响其他参数处理

**潜在风险**：
1. 函数签名修改可能影响其他调用点
   - **缓解措施**：仔细检查所有调用点，确保传递 `context.Context`

2. 数据库配置读取失败
   - **缓解措施**：提供合理的默认值

## 时间估算

- 后端数据结构修改：10 分钟
- 后端转换逻辑实现：30 分钟
- 后端 API 接口修改：20 分钟
- 前端类型定义：5 分钟
- 前端 UI 组件：20 分钟
- 单元测试：15 分钟
- 集成测试：15 分钟
- 手动测试：15 分钟
- **总计**：约 2.5 小时

## 参考资料

- SGLang 错误信息：`reasoning_effort` 只接受 `low/medium/high`
- 现有转换架构：`service/transformer.go`
- 现有设置管理：`handler/api.go` 的 `GetSettings` 和 `UpdateSettings`
- 前端设置页面：`webui/src/routes/settings/routing-settings.tsx`
