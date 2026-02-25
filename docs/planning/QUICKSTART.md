# UnifiedRequest 扩展 - 快速开始指南

## 概览

本指南帮助你快速开始实施 UnifiedRequest 结构扩展。建议按照阶段顺序实施，每个阶段独立可用。

---

## 准备工作

### 1. 环境检查
```bash
# 检查 Go 版本
go version  # 需要 1.25.0+

# 检查依赖
go mod tidy

# 运行现有测试，确保基线正常
cd service
go test -v
```

### 2. 创建功能分支
```bash
git checkout -b feature/unified-request-extension
```

### 3. 阅读规划文档
- ✅ `task_plan.md` - 7 阶段实施计划
- ✅ `findings.md` - 技术发现和洞察
- ✅ `implementation_checklist.md` - 详细清单
- ✅ 本文件 - 快速开始指南

---

## 阶段 1: 基础字段扩展 (推荐首先实施)

### 为什么从阶段 1 开始？
- ✅ 影响面小，风险低
- ✅ 不需要重构现有结构
- ✅ 常用功能，收益明显
- ✅ 为后续阶段打好基础

### 实施步骤

#### 步骤 1: 添加辅助函数 (15 分钟)

**文件**: `service/transform_openai.go`

在现有辅助函数后添加 (约 2500 行后):

```go
// getInt 安全获取 int 值
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// getInt64 安全获取 int64 值
func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// getFloat64Ptr 安全获取 *float64 值
func getFloat64Ptr(m map[string]interface{}, key string) *float64 {
	if v, ok := m[key].(float64); ok {
		return &v
	}
	return nil
}

// getStringPtr 安全获取 *string 值
func getStringPtr(m map[string]interface{}, key string) *string {
	if v, ok := m[key].(string); ok {
		return &v
	}
	return nil
}

// getInt64Ptr 安全获取 *int64 值
func getInt64Ptr(m map[string]interface{}, key string) *int64 {
	if v, ok := m[key].(float64); ok {
		val := int64(v)
		return &val
	}
	return nil
}

// getBoolPtr 安全获取 *bool 值
func getBoolPtr(m map[string]interface{}, key string) *bool {
	if v, ok := m[key].(bool); ok {
		return &v
	}
	return nil
}

// getStringArray 安全获取 []string 值
func getStringArray(m map[string]interface{}, key string) []string {
	if arr, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// getIntMap 安全获取 map[string]int64 值
func getIntMap(m map[string]interface{}, key string) map[string]int64 {
	if mapVal, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]int64, len(mapVal))
		for k, v := range mapVal {
			if num, ok := v.(float64); ok {
				result[k] = int64(num)
			}
		}
		return result
	}
	return nil
}

// getStringMap 安全获取 map[string]string 值
func getStringMap(m map[string]interface{}, key string) map[string]string {
	if mapVal, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]string, len(mapVal))
		for k, v := range mapVal {
			if str, ok := v.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return nil
}
```

#### 步骤 2: 添加新类型定义 (10 分钟)

**文件**: `service/transformer.go`

在 `UnifiedRequest` 定义前添加:

```go
// UnifiedStop 停止序列 (支持 string 或 []string)
type UnifiedStop struct {
	Single   *string
	Multiple []string
}
```

在 `UnifiedRequest` 结构体末尾添加新字段:

```go
type UnifiedRequest struct {
	// ... 现有字段 ...
	ReasoningEffort *string          `json:"reasoning_effort,omitempty"` // 推理强度参数

	// 阶段 1: 基础高级参数
	FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"` // 频率惩罚 (-2.0 到 2.0)
	PresencePenalty  *float64          `json:"presence_penalty,omitempty"`  // 存在惩罚 (-2.0 到 2.0)
	Seed             *int64            `json:"seed,omitempty"`              // 随机种子
	LogitBias        map[string]int64  `json:"logit_bias,omitempty"`        // Token 偏置
	Stop             *UnifiedStop      `json:"stop,omitempty"`              // 停止序列
	User             *string           `json:"user,omitempty"`              // 用户标识符
	Metadata         map[string]string `json:"metadata,omitempty"`          // 元数据
}
```

#### 步骤 3: 更新 OpenAI → Unified 转换 (20 分钟)

**文件**: `service/transform_openai.go`

在 `TransformOpenAIToUnified` 函数中，现有字段解析后添加:

```go
func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error) {
	// ... 现有代码 ...

	// 阶段 1: 基础高级参数
	unified.FrequencyPenalty = getFloat64Ptr(req, "frequency_penalty")
	unified.PresencePenalty = getFloat64Ptr(req, "presence_penalty")
	unified.Seed = getInt64Ptr(req, "seed")
	unified.LogitBias = getIntMap(req, "logit_bias")
	unified.User = getStringPtr(req, "user")
	unified.Metadata = getStringMap(req, "metadata")

	// 处理 stop (可能是 string 或 []string)
	if stopVal, ok := req["stop"]; ok && stopVal != nil {
		unified.Stop = &UnifiedStop{}
		switch v := stopVal.(type) {
		case string:
			unified.Stop.Single = &v
		case []interface{}:
			unified.Stop.Multiple = getStringArray(req, "stop")
		}
	}

	return unified, nil
}
```

#### 步骤 4: 更新 Unified → OpenAI 转换 (20 分钟)

**文件**: `service/transform_openai.go`

在 `TransformUnifiedToOpenAI` 函数中，现有字段输出后添加:

```go
func TransformUnifiedToOpenAI(unified *UnifiedRequest) ([]byte, error) {
	req := map[string]interface{}{
		// ... 现有字段 ...
	}

	// 阶段 1: 基础高级参数
	if unified.FrequencyPenalty != nil {
		req["frequency_penalty"] = *unified.FrequencyPenalty
	}
	if unified.PresencePenalty != nil {
		req["presence_penalty"] = *unified.PresencePenalty
	}
	if unified.Seed != nil {
		req["seed"] = *unified.Seed
	}
	if unified.LogitBias != nil && len(unified.LogitBias) > 0 {
		req["logit_bias"] = unified.LogitBias
	}
	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop"] = *unified.Stop.Single
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop"] = unified.Stop.Multiple
		}
	}
	if unified.User != nil {
		req["user"] = *unified.User
	}
	if unified.Metadata != nil && len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}

	return json.Marshal(req)
}
```

#### 步骤 5: 更新 Unified → Anthropic 转换 (15 分钟)

**文件**: `service/transform_anthropic.go`

在 `TransformUnifiedToAnthropic` 函数中添加:

```go
func TransformUnifiedToAnthropic(unified *UnifiedRequest) ([]byte, error) {
	req := map[string]interface{}{
		// ... 现有字段 ...
	}

	// 阶段 1: 映射兼容字段
	// Anthropic 支持 stop_sequences (只支持数组)
	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop_sequences"] = []string{*unified.Stop.Single}
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop_sequences"] = unified.Stop.Multiple
		}
	}

	// Anthropic 支持 metadata
	if unified.Metadata != nil && len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}

	// 注意: Anthropic 不支持以下字段，静默忽略
	// - frequency_penalty
	// - presence_penalty
	// - seed
	// - logit_bias
	// - user

	return json.Marshal(req)
}
```

#### 步骤 6: 更新 Anthropic → Unified 转换 (10 分钟)

**文件**: `service/transform_anthropic.go`

在 `TransformAnthropicToUnified` 函数中添加:

```go
func TransformAnthropicToUnified(rawBody []byte) (*UnifiedRequest, error) {
	// ... 现有代码 ...

	// 阶段 1: 解析 Anthropic 特有字段
	if stopSeqs := getStringArray(req, "stop_sequences"); len(stopSeqs) > 0 {
		unified.Stop = &UnifiedStop{Multiple: stopSeqs}
	}

	if metadata := getStringMap(req, "metadata"); len(metadata) > 0 {
		unified.Metadata = metadata
	}

	return unified, nil
}
```

#### 步骤 7: 更新 Responses API 转换 (15 分钟)

**文件**: `service/transform_responses.go`

在 `TransformResponsesToUnified` 和 `TransformUnifiedToResponses` 中添加类似逻辑。

#### 步骤 8: 编写测试 (30 分钟)

**文件**: `service/transformer_test.go`

添加表驱动测试:

```go
func TestTransformOpenAIToUnified_AdvancedParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "frequency_penalty",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"frequency_penalty": 0.5
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.FrequencyPenalty == nil {
					t.Fatal("FrequencyPenalty is nil")
				}
				if *u.FrequencyPenalty != 0.5 {
					t.Errorf("expected 0.5, got %v", *u.FrequencyPenalty)
				}
			},
		},
		{
			name: "presence_penalty",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"presence_penalty": -0.3
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.PresencePenalty == nil {
					t.Fatal("PresencePenalty is nil")
				}
				if *u.PresencePenalty != -0.3 {
					t.Errorf("expected -0.3, got %v", *u.PresencePenalty)
				}
			},
		},
		{
			name: "seed",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"seed": 12345
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Seed == nil {
					t.Fatal("Seed is nil")
				}
				if *u.Seed != 12345 {
					t.Errorf("expected 12345, got %v", *u.Seed)
				}
			},
		},
		{
			name: "logit_bias",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"logit_bias": {"50256": -100, "50257": 100}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.LogitBias == nil {
					t.Fatal("LogitBias is nil")
				}
				if u.LogitBias["50256"] != -100 {
					t.Errorf("expected -100, got %v", u.LogitBias["50256"])
				}
				if u.LogitBias["50257"] != 100 {
					t.Errorf("expected 100, got %v", u.LogitBias["50257"])
				}
			},
		},
		{
			name: "stop_string",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stop": "END"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Stop == nil {
					t.Fatal("Stop is nil")
				}
				if u.Stop.Single == nil {
					t.Fatal("Stop.Single is nil")
				}
				if *u.Stop.Single != "END" {
					t.Errorf("expected 'END', got %v", *u.Stop.Single)
				}
			},
		},
		{
			name: "stop_array",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stop": ["END", "STOP"]
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Stop == nil {
					t.Fatal("Stop is nil")
				}
				if len(u.Stop.Multiple) != 2 {
					t.Fatalf("expected 2 stops, got %d", len(u.Stop.Multiple))
				}
				if u.Stop.Multiple[0] != "END" || u.Stop.Multiple[1] != "STOP" {
					t.Errorf("unexpected stop values: %v", u.Stop.Multiple)
				}
			},
		},
		{
			name: "user",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"user": "user-123"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.User == nil {
					t.Fatal("User is nil")
				}
				if *u.User != "user-123" {
					t.Errorf("expected 'user-123', got %v", *u.User)
				}
			},
		},
		{
			name: "metadata",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"metadata": {"key1": "value1", "key2": "value2"}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if u.Metadata["key1"] != "value1" {
					t.Errorf("expected 'value1', got %v", u.Metadata["key1"])
				}
				if u.Metadata["key2"] != "value2" {
					t.Errorf("expected 'value2', got %v", u.Metadata["key2"])
				}
			},
		},
		{
			name: "all_fields",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"frequency_penalty": 0.5,
				"presence_penalty": -0.3,
				"seed": 12345,
				"stop": ["END"],
				"user": "user-123"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.FrequencyPenalty == nil || *u.FrequencyPenalty != 0.5 {
					t.Error("FrequencyPenalty mismatch")
				}
				if u.PresencePenalty == nil || *u.PresencePenalty != -0.3 {
					t.Error("PresencePenalty mismatch")
				}
				if u.Seed == nil || *u.Seed != 12345 {
					t.Error("Seed mismatch")
				}
				if u.Stop == nil || len(u.Stop.Multiple) != 1 {
					t.Error("Stop mismatch")
				}
				if u.User == nil || *u.User != "user-123" {
					t.Error("User mismatch")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformOpenAIToUnified(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, unified)
		})
	}
}

func TestTransformUnifiedToOpenAI_AdvancedParams(t *testing.T) {
	freq := 0.5
	pres := -0.3
	seed := int64(12345)
	user := "user-123"

	unified := &UnifiedRequest{
		Model:            "gpt-4",
		Messages:         []UnifiedMessage{{Role: "user", Content: "hi"}},
		FrequencyPenalty: &freq,
		PresencePenalty:  &pres,
		Seed:             &seed,
		LogitBias:        map[string]int64{"50256": -100},
		Stop:             &UnifiedStop{Multiple: []string{"END"}},
		User:             &user,
		Metadata:         map[string]string{"key": "value"},
	}

	result, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// 验证字段存在
	if parsed["frequency_penalty"] != 0.5 {
		t.Errorf("frequency_penalty mismatch")
	}
	if parsed["presence_penalty"] != -0.3 {
		t.Errorf("presence_penalty mismatch")
	}
	if parsed["seed"] != float64(12345) {
		t.Errorf("seed mismatch")
	}
	if parsed["user"] != "user-123" {
		t.Errorf("user mismatch")
	}
}
```

#### 步骤 9: 运行测试 (5 分钟)

```bash
cd service
go test -v -run TestTransformOpenAIToUnified_AdvancedParams
go test -v -run TestTransformUnifiedToOpenAI_AdvancedParams
go test -v  # 运行所有测试
```

#### 步骤 10: 提交代码 (5 分钟)

```bash
git add service/transformer.go service/transform_openai.go service/transform_anthropic.go service/transformer_test.go
git commit -m "feat(阶段1): 添加基础高级参数支持

- 新增字段: FrequencyPenalty, PresencePenalty, Seed, LogitBias, Stop, User, Metadata
- 新增辅助函数: getInt64Ptr, getStringPtr, getStringArray, getIntMap 等
- 更新 OpenAI/Anthropic 转换逻辑
- 添加完整的单元测试
- 向后兼容，所有现有测试通过"
```

---

## 验证清单

### 功能验证
- [ ] 所有新字段都能正确解析
- [ ] 所有新字段都能正确输出
- [ ] Anthropic 不支持的字段被正确忽略
- [ ] 向后兼容，现有功能不受影响

### 测试验证
- [ ] 所有新测试通过
- [ ] 所有现有测试通过
- [ ] 测试覆盖所有新字段
- [ ] 测试覆盖边界情况

### 代码质量
- [ ] 代码通过 `go fmt`
- [ ] 代码通过 `go vet`
- [ ] 代码通过 `golangci-lint` (如果配置)
- [ ] 代码注释清晰

---

## 下一步

### 阶段 1 完成后
1. 更新 `task_plan.md` - 标记阶段 1 为 `complete`
2. 更新 `progress.md` - 记录完成情况
3. 考虑是否继续阶段 2，或先部署阶段 1

### 继续阶段 2
如果决定继续，阅读 `implementation_checklist.md` 中的阶段 2 清单。

---

## 常见问题

### Q: 为什么使用指针类型？
A: 指针类型可以区分"未设置"和"设置为零值"，确保 `omitempty` 正确工作。

### Q: Anthropic 不支持的字段怎么处理？
A: 静默忽略，不报错。这是最佳实践，避免破坏用户体验。

### Q: 如何测试流式响应？
A: 阶段 1 主要影响请求转换，流式响应转换在后续阶段处理。

### Q: 性能影响如何？
A: 阶段 1 添加的字段都是简单类型，性能影响 <1%。

---

## 获取帮助

- 查看 `findings.md` - 技术细节和洞察
- 查看 `task_plan.md` - 完整实施计划
- 查看 `implementation_checklist.md` - 详细清单
- 查看现有代码 - 参考现有实现模式

---

**创建日期**: 2026-02-25
**预计完成时间**: 2-3 小时 (阶段 1)
