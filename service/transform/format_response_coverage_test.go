package transform

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// intPtr 是 test_helpers_test.go 里那组指针 helper 的补充：UnifiedMessage.MessageIndex
// 是 *int，现有 helper 只有 int64Ptr。
func intPtr(i int) *int {
	return &i
}

// fullUnifiedResponse 构造一个「所有可序列化字段都非零」的统一响应，
// 用于把「统一响应每个字段在各协议出站时 emit 成什么 / 有没有被吃掉」冻结成 golden。
// 这是 fullUnifiedRequest 的响应侧对偶。
//
// 两条边界，改这个 fixture 前先读：
//  1. Content 保持最简（单条 string）。消息内部多模态结构（image_url / input_audio）
//     的出站映射已由 TestTransformProviderResponse_OpenAIToAnthropic_MapsMultimodalImage
//     等测试覆盖，本测试只关心响应级字段的去向，不重复覆盖 content part 变体。
//  2. Error 与 Choices 同时填。真实响应里两者通常互斥（错误响应没有 choices），
//     但本 fixture 是覆盖探针不是拟真样本：三 style 的错误信封分支守卫都是
//     `Error != nil && len(Choices) == 0`，所以三 style 在
//     full 里都会走正常路径、Error 不出现在 body。纯错误路径由 errorOnlyUnifiedResponse
//     单独覆盖。
func fullUnifiedResponse(t *testing.T) *models.UnifiedResponse {
	t.Helper()

	content := "Sure, here is the weather."
	return &models.UnifiedResponse{
		ID:      "resp-coverage",
		Object:  "chat.completion",
		Created: 1_700_000_000,
		Model:   "coverage-model",
		Choices: []models.UnifiedChoice{{
			Index:        0, // 见 exempt：序号 0 是合法首值
			FinishReason: "tool_calls",
			Message: &models.UnifiedMessage{
				Role:    "assistant",
				Content: content,
				ToolCalls: []models.UnifiedToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: models.UnifiedToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"SF"}`,
					},
					Index:        0, // 见 exempt
					CacheControl: &models.CacheControl{Type: "ephemeral", TTL: "5m"},
				}},
				ToolCallID:           "call_prev",
				CacheControl:         &models.CacheControl{Type: "ephemeral", TTL: "5m"},
				ReasoningContent:     ptr("let me think"),
				Reasoning:            ptr("still thinking"),
				ReasoningSignature:   ptr("sig-abc"),
				RedactedThinkingData: ptr("redacted-xyz"),
				MessageIndex:         intPtr(1),
				ToolCallName:         ptr("get_weather"),
				ToolCallIsError:      boolPtr(true),
			},
			// Delta 见 exempt：流式增量专用
			Logprobs: &models.LogprobsContent{
				Content: []models.TokenLogprob{{
					Token:   "Sure",
					Logprob: -0.25,
					Bytes:   []int{83, 117},
					TopLogprobs: []models.TopLogprob{{
						Token:   "Sure",
						Logprob: -0.25,
						Bytes:   []int{83},
					}},
				}},
			},
		}},
		Usage: &models.Usage{
			PromptTokens:     11,
			CompletionTokens: 22,
			TotalTokens:      33,
			PromptTokensDetails: models.PromptTokensDetails{
				CachedTokens: 3,
				AudioTokens:  1,
			},
			CompletionTokensDetails: models.CompletionTokensDetails{
				ReasoningTokens:      7,
				ReasoningTokensKnown: true,
				AudioTokens:          2,
			},
		},
		SystemFingerprint: "fp_coverage",
		ServiceTier:       "default",
		Error: &models.ResponseError{
			StatusCode: 400,
			Detail: models.ErrorDetail{
				Code:      "bad_request",
				Message:   "coverage error message",
				Type:      "invalid_request_error",
				Param:     "model",
				RequestID: "req-coverage",
			},
		},
	}
}

// TestFullUnifiedResponse_CoversAllSerializableFields 防止 fixture 随统一模型演进而腐化。
// 给 UnifiedResponse 或其嵌套类型新增字段却忘了填进 fullUnifiedResponse 时，此测试失败——
// 否则新字段会静默游离在 golden 覆盖之外，等于覆盖矩阵有洞而没人知道。
//
// 与请求侧 TestFullUnifiedRequest_CoversAllSerializableFields 的**刻意偏离：递归下降**。
// UnifiedRequest 有 41 个顶层字段，单层 IsZero 已是实质约束；UnifiedResponse 只有 9 个
// 顶层字段，实质内容全在嵌套层（Choices[].Message.ToolCalls、Usage.*Details、Logprobs 三层），
// 单层 IsZero 只要 Choices 非 nil 就通过，等于没有守卫。所以这里必须递归穿透。
//
// 下降规则见 descendCoverage：struct / *struct / slice 逐层进；interface{}（Content）
// 是叶子边界不再下降——它背后是 text/image_url/input_audio 互斥的 sum type，单个 fixture
// 值无法同时填满所有变体，其出站映射另由多模态专项测试覆盖。
func TestFullUnifiedResponse_CoversAllSerializableFields(t *testing.T) {
	t.Parallel()

	// 豁免字段必须写明理由，且必须保持零值（否则说明豁免理由已失效）。
	// key 是从 UnifiedResponse 根算起的字段路径，slice 元素用 [] 标记。
	exempt := map[string]string{
		"Choices[].Delta":                     "流式增量专用，非流式响应恒为 nil；FormatResponse 三 style 全不读它",
		"Choices[].Index":                     "序号 int，0 是合法首选值，无法用非零表达；不影响新增字段检测",
		"Choices[].Message.ToolCalls[].Index": "同上，工具调用序号 0 合法",
	}

	resp := fullUnifiedResponse(t)
	checkCoverage(t, "", reflect.ValueOf(*resp), exempt)
}

// checkCoverage 遍历 struct 的导出字段，对每个字段：豁免字段反向锁定为零值，
// 其余字段要求非零，非零的复合字段再交给 descendCoverage 决定是否继续下降。
func checkCoverage(t *testing.T, prefix string, structVal reflect.Value, exempt map[string]string) {
	t.Helper()

	typ := structVal.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			// 未导出字段：reflect 无法 Interface()，且不参与序列化，跳过。
			continue
		}

		path := prefix + field.Name
		fieldVal := structVal.Field(i)

		if reason, ok := exempt[path]; ok {
			if !fieldVal.IsZero() {
				t.Errorf("字段 %s 在豁免表里（%s）却被填了值，请更新豁免表或移除该值", path, reason)
			}
			continue
		}

		if fieldVal.IsZero() {
			t.Errorf("字段 %s 未填非零值：新增字段必须纳入 fullUnifiedResponse，否则 golden 覆盖不到它", path)
			continue
		}

		descendCoverage(t, path, fieldVal, exempt)
	}
}

// descendCoverage 对已确认非零的值决定是否继续下降检查嵌套字段。
func descendCoverage(t *testing.T, path string, v reflect.Value, exempt map[string]string) {
	t.Helper()

	switch v.Kind() {
	case reflect.Ptr:
		descendCoverage(t, path, v.Elem(), exempt)
	case reflect.Slice, reflect.Array:
		// 非零 slice 至少有一个元素，取第 0 个作为代表继续下降。
		descendCoverage(t, path+"[]", v.Index(0), exempt)
	case reflect.Struct:
		checkCoverage(t, path+".", v, exempt)
	case reflect.Interface:
		// 叶子边界：Content 是 interface{}，背后是互斥的 content part sum type，不下降。
		return
	default:
		// 基础类型 / map：叶子，非零即可。
		return
	}
}

// formatResponseErrorJSON 把 FormatResponse 的错误包成确定的 JSON，用于冻进 golden。
// 与 fromUnifiedErrorJSON 同理：不引入「哪个 style 期望 error」的期望表，那会是 adapter
// 逻辑的第二份副本，必然漂移。
func formatResponseErrorJSON(err error) []byte {
	b, _ := json.Marshal(map[string]string{"__format_response_error__": err.Error()})
	return b
}

// TestFormatResponseCoverage_Golden 把统一响应在各协议下的出站 body 冻结成 golden。
//
// 两个维度：fixture（响应形态）× wire format（目标协议形状），golden 落在
// testdata/golden/format_response/{fixture}/{format}.json。这是请求侧
// TestFromUnifiedCoverage_Golden 的响应侧对偶。
//
// 直调 adapter.FormatResponse 而非 TransformProviderResponse：FormatResponse 是纯函数
// （无 ctx、不读设置），测的是纯出站字段映射终态。TransformProviderResponse 会先跑
// ParseResponse，任一侧的字段丢失会被另一侧掩盖，「字段去向」不可读。
//
// formats 刻意硬编码而非 range formatAdapters：新协议注册后应当人工审一遍它的
// 出站形状再纳入基线，自动全跑会静默接受未审输出。
func TestFormatResponseCoverage_Golden(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		name  string
		build func(*testing.T) *models.UnifiedResponse
	}{
		{name: "full", build: fullUnifiedResponse},
		{name: "error_only", build: errorOnlyUnifiedResponse},
		{name: "multi_choice", build: multiChoiceUnifiedResponse},
	}

	formats := []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatOpenAIResponses, consts.FormatAnthropic}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			for _, format := range formats {
				t.Run(string(format), func(t *testing.T) {
					t.Parallel()

					// 每个 subtest 独立构造：Anthropic 出站会 snapshot/restore 消息级 reasoning，
					// 共享同一实例可能跨 subtest 干扰。
					resp := fixture.build(t)

					adapter, err := getAdapter(format)
					if err != nil {
						t.Fatalf("getAdapter(%s) failed: %v", format, err)
					}

					out, err := adapter.FormatResponse(resp)
					// FormatResponse 返错时不 Fatalf，而是把错误也冻进 golden：
					// 某些 fixture/style 组合的当前行为就是返错，这本身是要被覆盖矩阵记录的现状。
					if err != nil {
						out = formatResponseErrorJSON(err)
					}

					wantPath := filepath.Join("testdata", "golden", "format_response", fixture.name, string(format)+".json")
					assertGoldenJSON(t, wantPath, out)
				})
			}
		})
	}
}

// errorOnlyUnifiedResponse 构造一个纯错误响应（Error 非空、Choices 为空）。
//
// 为什么需要独立 fixture：三个 style 的错误信封分支守卫都是
// `Error != nil && len(Choices) == 0`，无法塞进 fullUnifiedResponse（那里 Choices 非空）。
// 这份 fixture 冻结的是错误响应的出站形状差异：openai / responses 是 `{"error":{...}}`
// 五字段信封，anthropic 是官方的 `{"type":"error","error":{type,message},"request_id":...}`
// ——同一份错误在三协议下的键名与字段集都不同，golden 记录这个差异。
func errorOnlyUnifiedResponse(t *testing.T) *models.UnifiedResponse {
	t.Helper()

	return &models.UnifiedResponse{
		ID:    "resp-error",
		Model: "coverage-model",
		Error: &models.ResponseError{
			StatusCode: 429,
			Detail: models.ErrorDetail{
				Code:      "rate_limited",
				Message:   "too many requests",
				Type:      "rate_limit_error",
				Param:     "",
				RequestID: "req-error",
			},
		},
	}
}

// multiChoiceUnifiedResponse 构造一个含两个 Choice 的响应。
//
// 为什么需要独立 fixture：fullUnifiedResponse 是单 choice 基线，多 choice 并进去会让
// 满配 golden 噪声翻倍且掩盖单 choice 形态。这份 fixture 冻结的是「多 choice 时各 style
// 怎么处理」——anthropic 出站只取 Choices[0]、丢弃其余，golden 会记录这个现状。
func multiChoiceUnifiedResponse(t *testing.T) *models.UnifiedResponse {
	t.Helper()

	first := "first choice"
	second := "second choice"
	return &models.UnifiedResponse{
		ID:      "resp-multi",
		Object:  "chat.completion",
		Created: 1_700_000_001,
		Model:   "coverage-model",
		Choices: []models.UnifiedChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message:      &models.UnifiedMessage{Role: "assistant", Content: first},
			},
			{
				Index:        1,
				FinishReason: "length",
				Message:      &models.UnifiedMessage{Role: "assistant", Content: second},
			},
		},
		Usage: &models.Usage{
			PromptTokens:     5,
			CompletionTokens: 8,
			TotalTokens:      13,
		},
	}
}
