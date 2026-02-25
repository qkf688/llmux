# reasoning_effort 参数映射实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 OpenAI 和 Responses API 请求转换链路添加 reasoning_effort 参数的自动映射与规范化功能，解决客户端发送非标准值（如 minimal）导致上游服务器报错的问题。

**Architecture:** 在统一格式（UnifiedRequest）中添加 ReasoningEffort 字段，在转换阶段进行规范化处理（minimal→low，无效值→配置的默认值），通过数据库配置控制映射行为，前端提供可视化设置界面。

**Tech Stack:** Go 1.25+, Gin, GORM, TypeScript, React 19, Tailwind CSS

---

## Task 1: 添加数据库配置常量

**Files:**
- Modify: `models/model.go:197` (在现有常量列表末尾添加)

**Step 1: 添加配置常量**

在 `models/model.go` 的常量定义区域（约第 197 行，`SettingKeyAutoSaveTemplateOnAssociate` 之后）添加：

```go
	// reasoning_effort 参数映射相关设置
	SettingKeyReasoningEffortMappingEnabled = "reasoning_effort_mapping_enabled" // 是否启用映射
	SettingKeyReasoningEffortDefaultValue   = "reasoning_effort_default_value"   // 默认值（low/medium/high）
```

**Step 2: 验证语法**

```bash
cd E:\a-2025-11-projects-git\llmio
go build -o /dev/null ./models
```

Expected: 编译成功，无错误

**Step 3: 提交**

```bash
git add models/model.go
git commit -m "feat(models): 添加 reasoning_effort 映射配置常量"
```

---

## Task 2: 添加 UnifiedRequest 字段

**Files:**
- Modify: `service/transformer.go:54` (在 System 字段之后添加)

**Step 1: 添加 ReasoningEffort 字段**

在 `service/transformer.go` 的 `UnifiedRequest` 结构体中（约第 54 行，System 字段之后）添加：

```go
	ReasoningEffort *string          `json:"reasoning_effort,omitempty"` // 推理强度参数
```

**Step 2: 验证语法**

```bash
go build -o /dev/null ./service
```

Expected: 编译成功，无错误

**Step 3: 提交**

```bash
git add service/transformer.go
git commit -m "feat(service): UnifiedRequest 添加 ReasoningEffort 字段"
```

---

## Task 3: 实现规范化函数

**Files:**
- Modify: `service/transform_openai.go` (在文件末尾添加辅助函数)

**Step 1: 添加规范化函数**

在 `service/transform_openai.go` 文件末尾添加三个辅助函数：

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

**Step 2: 检查导入**

确保文件顶部有必要的导入：

```go
import (
	"context"
	"strings"
	// ... 其他导入
	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)
```

**Step 3: 验证语法**

```bash
go build -o /dev/null ./service
```

Expected: 编译成功，无错误

**Step 4: 提交**

```bash
git add service/transform_openai.go
git commit -m "feat(service): 添加 reasoning_effort 规范化函数"
```

---

## Task 4: 修改 TransformOpenAIToUnified 函数签名

**Files:**
- Modify: `service/transform_openai.go` (TransformOpenAIToUnified 函数)
- Modify: `service/transformer.go:108` (调用点)

**Step 1: 修改函数签名**

找到 `TransformOpenAIToUnified` 函数定义，修改签名：

```go
// 修改前
func TransformOpenAIToUnified(rawBody []byte) (*UnifiedRequest, error)

// 修改后
func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error)
```

**Step 2: 在函数末尾添加 reasoning_effort 处理**

在 `TransformOpenAIToUnified` 函数返回之前（return unified, nil 之前）添加：

```go
	// 处理 reasoning_effort 参数
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

**Step 3: 修改调用点**

在 `service/transformer.go` 的 `ProcessRequest` 函数中（约第 108 行），修改调用：

```go
// 修改前
case "openai":
	unified, err = TransformOpenAIToUnified(rawBody)

// 修改后
case "openai":
	unified, err = TransformOpenAIToUnified(ctx, rawBody)
```

**Step 4: 验证语法**

```bash
go build -o /dev/null ./service
```

Expected: 编译成功，无错误

**Step 5: 提交**

```bash
git add service/transform_openai.go service/transformer.go
git commit -m "feat(service): TransformOpenAIToUnified 添加 context 参数和 reasoning_effort 处理"
```

---

## Task 5: 修改 TransformUnifiedToOpenAI 函数

**Files:**
- Modify: `service/transform_openai.go` (TransformUnifiedToOpenAI 函数)

**Step 1: 添加 reasoning_effort 输出**

在 `TransformUnifiedToOpenAI` 函数中，找到其他参数输出的位置（如 temperature, top_p 等），在附近添加：

```go
	// 输出 reasoning_effort 参数
	if unified.ReasoningEffort != nil {
		req["reasoning_effort"] = *unified.ReasoningEffort
	}
```

**Step 2: 验证语法**

```bash
go build -o /dev/null ./service
```

Expected: 编译成功，无错误

**Step 3: 提交**

```bash
git add service/transform_openai.go
git commit -m "feat(service): TransformUnifiedToOpenAI 添加 reasoning_effort 输出"
```

---

## Task 6: 修改 Responses API 转换函数

**Files:**
- Modify: `service/transform_responses.go` (TransformResponsesToUnified 和 TransformUnifiedToResponses)
- Modify: `service/transformer.go:110` (调用点)

**Step 1: 修改 TransformResponsesToUnified 函数签名**

```go
// 修改前
func TransformResponsesToUnified(rawBody []byte) (*UnifiedRequest, error)

// 修改后
func TransformResponsesToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error)
```

**Step 2: 在 TransformResponsesToUnified 函数末尾添加处理**

在返回之前添加：

```go
	// 处理 reasoning_effort 参数
	if effort, ok := req["reasoning_effort"].(string); ok && effort != "" {
		if getReasoningEffortMappingEnabled(ctx) {
			normalized := normalizeReasoningEffort(ctx, effort)
			unified.ReasoningEffort = &normalized
		} else {
			unified.ReasoningEffort = &effort
		}
	}
```

**Step 3: 在 TransformUnifiedToResponses 函数中添加输出**

找到其他参数输出的位置，添加：

```go
	// 输出 reasoning_effort 参数
	if unified.ReasoningEffort != nil {
		req["reasoning_effort"] = *unified.ReasoningEffort
	}
```

**Step 4: 修改调用点**

在 `service/transformer.go` 的 `ProcessRequest` 函数中（约第 110 行），修改调用：

```go
// 修改前
case "openai-res":
	unified, err = TransformResponsesToUnified(rawBody)

// 修改后
case "openai-res":
	unified, err = TransformResponsesToUnified(ctx, rawBody)
```

**Step 5: 验证语法**

```bash
go build -o /dev/null ./service
```

Expected: 编译成功，无错误

**Step 6: 提交**

```bash
git add service/transform_responses.go service/transformer.go
git commit -m "feat(service): Responses API 转换添加 reasoning_effort 处理"
```

---

## Task 7: 修改后端 API 结构体

**Files:**
- Modify: `handler/api.go` (SettingsResponse 和 UpdateSettingsRequest)

**Step 1: 读取 handler/api.go 找到结构体位置**

```bash
grep -n "type SettingsResponse struct" handler/api.go
grep -n "type UpdateSettingsRequest struct" handler/api.go
```

**Step 2: 在 SettingsResponse 结构体末尾添加字段**

```go
	// reasoning_effort 映射相关设置
	ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
	ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"` // low/medium/high
```

**Step 3: 在 UpdateSettingsRequest 结构体末尾添加字段**

```go
	// reasoning_effort 映射相关设置
	ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
	ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"`
```

**Step 4: 验证语法**

```bash
go build -o /dev/null ./handler
```

Expected: 编译成功，无错误

**Step 5: 提交**

```bash
git add handler/api.go
git commit -m "feat(handler): API 结构体添加 reasoning_effort 映射字段"
```

---

## Task 8: 修改 GetSettings 函数

**Files:**
- Modify: `handler/api.go` (GetSettings 函数)

**Step 1: 在 GetSettings 函数的默认值初始化中添加**

找到 `response := SettingsResponse{` 的位置，在末尾添加：

```go
		ReasoningEffortMappingEnabled: true,  // 默认启用
		ReasoningEffortDefaultValue:   "low", // 默认值为 low
```

**Step 2: 在 switch 语句中添加读取逻辑**

找到 `switch setting.Key {` 的位置，在末尾添加：

```go
	case models.SettingKeyReasoningEffortMappingEnabled:
		response.ReasoningEffortMappingEnabled = setting.Value == "true"
	case models.SettingKeyReasoningEffortDefaultValue:
		response.ReasoningEffortDefaultValue = setting.Value
```

**Step 3: 验证语法**

```bash
go build -o /dev/null ./handler
```

Expected: 编译成功，无错误

**Step 4: 提交**

```bash
git add handler/api.go
git commit -m "feat(handler): GetSettings 添加 reasoning_effort 映射配置读取"
```

---

## Task 9: 修改 UpdateSettings 函数

**Files:**
- Modify: `handler/api.go` (UpdateSettings 函数)

**Step 1: 在 UpdateSettings 函数末尾添加更新逻辑**

找到其他设置更新的位置（如 auto_associate_on_add 等），在附近添加：

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

**Step 2: 验证语法**

```bash
go build -o /dev/null ./handler
```

Expected: 编译成功，无错误

**Step 3: 提交**

```bash
git add handler/api.go
git commit -m "feat(handler): UpdateSettings 添加 reasoning_effort 映射配置更新"
```

---

## Task 10: 修改前端类型定义

**Files:**
- Modify: `webui/src/lib/api.ts` (Settings 和 UpdateSettingsRequest 接口)

**Step 1: 在 Settings 接口末尾添加字段**

```typescript
  reasoning_effort_mapping_enabled: boolean;
  reasoning_effort_default_value: 'low' | 'medium' | 'high';
```

**Step 2: 在 UpdateSettingsRequest 接口末尾添加字段**

```typescript
  reasoning_effort_mapping_enabled?: boolean;
  reasoning_effort_default_value?: string;
```

**Step 3: 验证语法**

```bash
cd webui
pnpm run type-check
```

Expected: 类型检查通过，无错误

**Step 4: 提交**

```bash
git add webui/src/lib/api.ts
git commit -m "feat(frontend): 添加 reasoning_effort 映射类型定义"
```

---

## Task 11: 添加前端 UI 组件

**Files:**
- Modify: `webui/src/routes/settings/routing-settings.tsx` (在"格式转换" Card 之后添加)

**Step 1: 确认需要导入 Select 组件**

检查文件顶部是否已导入 Select 相关组件，如果没有则添加：

```typescript
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
```

**Step 2: 在"格式转换" Card 之后添加新 Card**

找到"格式转换" Card 的 `</Card>` 闭合标签（约第 381 行），在其后添加：

```tsx
      <Card>
        <CardHeader>
          <CardTitle>参数映射</CardTitle>
          <CardDescription>
            配置请求参数的自动映射与规范化
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="reasoning-effort-mapping-enabled" className="text-sm md:text-base font-medium">
                reasoning_effort 参数映射
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                自动将 'minimal' 映射为有效值，处理无效参数
              </p>
            </div>
            <Switch
              id="reasoning-effort-mapping-enabled"
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

          {localSettings?.reasoning_effort_mapping_enabled && (
            <div className="space-y-1.5">
              <Label htmlFor="reasoning-effort-default-value" className="text-sm">默认值</Label>
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
                <SelectTrigger id="reasoning-effort-default-value">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">low（低推理强度）</SelectItem>
                  <SelectItem value="medium">medium（中等推理强度）</SelectItem>
                  <SelectItem value="high">high（高推理强度）</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs md:text-sm text-muted-foreground">
                无效参数将被映射为此默认值
              </p>
            </div>
          )}
        </CardContent>
      </Card>
```

**Step 3: 验证语法**

```bash
cd webui
pnpm run type-check
```

Expected: 类型检查通过，无错误

**Step 4: 提交**

```bash
git add webui/src/routes/settings/routing-settings.tsx
git commit -m "feat(frontend): 添加 reasoning_effort 参数映射 UI 组件"
```

---

## Task 12: 编译和手动测试

**Files:**
- Test: 完整系统

**Step 1: 编译后端**

```bash
cd E:\a-2025-11-projects-git\llmio
go build -o llmio.exe
```

Expected: 编译成功，生成 llmio.exe

**Step 2: 编译前端**

```bash
cd webui
pnpm run build
```

Expected: 构建成功，生成 dist 目录

**Step 3: 启动服务器**

```bash
cd ..
./llmio.exe
```

Expected: 服务器启动成功

**Step 4: 手动测试前端设置页面**

1. 打开浏览器访问 http://localhost:8080/settings
2. 导航到"通用设置"页面
3. 验证"参数映射" Card 显示正常
4. 验证"reasoning_effort 参数映射"开关可以切换
5. 验证开关开启时显示"默认值"下拉选择
6. 验证可以选择 low/medium/high
7. 点击"保存"按钮，验证保存成功
8. 刷新页面，验证设置已保存

**Step 5: 测试 API 请求**

使用 curl 或 Postman 发送测试请求：

```bash
# 测试 minimal 映射为 low
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "messages": [{"role": "user", "content": "test"}],
    "reasoning_effort": "minimal"
  }'
```

Expected: 请求成功，不返回 400 错误

**Step 6: 检查日志**

查看服务器日志，验证 reasoning_effort 参数已被规范化为 "low"

**Step 7: 最终提交**

```bash
git add .
git commit -m "test: 验证 reasoning_effort 参数映射功能"
```

---

## 验收标准

完成以上所有任务后，验证以下标准：

- [x] 后端数据结构修改完成（Task 1-2）
- [x] 后端转换逻辑实现完成（Task 3-6）
- [x] 后端 API 接口修改完成（Task 7-9）
- [x] 前端类型定义添加完成（Task 10）
- [x] 前端 UI 组件实现完成（Task 11）
- [x] 编译测试通过（Task 12）
- [x] `minimal` 正确映射为 `low`
- [x] 无效值正确映射为配置的默认值
- [x] 映射开关关闭时直接透传
- [x] 前端设置页面功能正常

## 注意事项

1. **函数签名修改**：确保所有调用 `TransformOpenAIToUnified` 和 `TransformResponsesToUnified` 的地方都传递了 `context.Context` 参数
2. **Anthropic 转换**：不要在 `TransformUnifiedToAnthropic` 中输出 `reasoning_effort` 参数
3. **默认值**：所有配置读取失败时都应该有合理的默认值（映射开关默认 true，默认值默认 "low"）
4. **前端验证**：后端已经验证默认值合法性，前端只需要提供正确的选项

## 参考资料

- 设计文档: `docs/plans/2026-02-19-reasoning-effort-mapping-design.md`
- 现有转换架构: `service/transformer.go`
- 现有设置管理: `handler/api.go`
- 前端设置页面: `webui/src/routes/settings/routing-settings.tsx`
