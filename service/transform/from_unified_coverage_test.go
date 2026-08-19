package transform

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/qkf688/llmux/models"
)

// fullUnifiedRequest 构造一个「所有可序列化字段都非零」的统一请求，
// 用于把「统一模型每个字段在各协议出站时 emit 成什么 / 有没有被吃掉」冻结成 golden。
//
// 为什么不用 baseUnifiedRequest：那个 fixture 只填 Messages，覆盖不到出站字段映射。
//
// 三条边界，改这个 fixture 前先读：
//  1. Messages 保持最简（单条 user 文本）。消息内部结构（多模态 / tool_calls）已由
//     TestGolden_RequestConversions 的 request/in/*.json 覆盖，本测试只关心请求级字段。
//  2. EmbeddingInput 留空：UnifiedRequest.Validate 规定它与 Messages 互斥，
//     无法在同一 fixture 里共存。embedding 出站路径需要独立 fixture，不在本测试范围。
//  3. SystemParts 留空：它与 System 语义重叠且 Anthropic 会优先取 SystemParts，
//     两者同时填会掩盖 System 字段的真实去向。
//
// 帮助字段（RawRequest / ExtraBody / Include / TransformerMetadata / Query）刻意填了值，
// 用于让 golden 证明它们不会泄漏到上游 body（SanitizedForProvider 会清掉）。
// 这里不额外写「输出不含某键」的断言——目标协议可能存在同名键（如 Responses 的 include），
// 那种断言会变成误报源。
func fullUnifiedRequest(t *testing.T) *models.UnifiedRequest {
	t.Helper()

	return &models.UnifiedRequest{
		Model:    "coverage-model",
		Messages: []models.UnifiedMessage{{Role: "user", Content: "hello"}},
		Stream:   true,

		MaxTokens:   1024,
		Temperature: floatPtr(0.7),
		TopP:        floatPtr(0.9),

		FrequencyPenalty:    floatPtr(0.1),
		PresencePenalty:     floatPtr(0.2),
		Seed:                int64Ptr(42),
		LogitBias:           map[string]int64{"1234": -5},
		Stop:                &models.UnifiedStop{Multiple: []string{"STOP"}},
		User:                ptr("coverage-user"),
		Metadata:            map[string]string{"trace": "coverage"},
		Logprobs:            boolPtr(true),
		TopLogprobs:         int64Ptr(3),
		MaxCompletionTokens: int64Ptr(2048),
		Store:               boolPtr(true),

		ResponseFormat: &models.UnifiedResponseFormat{
			Type:       "json_schema",
			JSONSchema: json.RawMessage(`{"name":"coverage","schema":{"type":"object"}}`),
		},
		ToolChoice:        &models.UnifiedToolChoice{StringValue: ptr("auto")},
		ParallelToolCalls: boolPtr(true),
		StreamOptions:     &models.UnifiedStreamOptions{IncludeUsage: true},

		Modalities: []string{"text"},
		Audio:      &models.UnifiedAudio{Voice: "alloy", Format: "mp3"},

		Tools: []models.UnifiedTool{{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        "get_weather",
				Description: "Get weather by city",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]any{"type": "string"}},
					"required":   []string{"city"},
				},
				Strict: boolPtr(true),
			},
		}},

		System: "You are a coverage probe.",

		ReasoningEffort: ptr("high"),
		ReasoningBudget: int64Ptr(4096),

		EmbeddingDimensions:     int64Ptr(1536),
		EmbeddingEncodingFormat: ptr("float"),

		PromptCacheKey:   ptr("coverage-cache-key"),
		SafetyIdentifier: ptr("coverage-safety"),
		ServiceTier:      ptr("auto"),

		EnableThinking: boolPtr(true),

		// 帮助字段：应被 SanitizedForProvider 清除，不进上游 body
		RawRequest:          []byte(`{"probe":"raw"}`),
		ExtraBody:           json.RawMessage(`{"probe":"extra"}`),
		Include:             []string{"probe"},
		TransformerMetadata: map[string]string{"probe": "meta"},
		TransformOptions:    models.TransformOptions{ArrayInputs: boolPtr(true)},
		Query:               map[string][]string{"probe": {"query"}},
	}
}

// TestFullUnifiedRequest_CoversAllSerializableFields 防止 fixture 随统一模型演进而腐化。
// 给 UnifiedRequest 新增可序列化字段却忘了填进 fullUnifiedRequest 时，此测试失败——
// 否则新字段会静默游离在 golden 覆盖之外，等于覆盖矩阵有洞而没人知道。
func TestFullUnifiedRequest_CoversAllSerializableFields(t *testing.T) {
	t.Parallel()

	// 豁免字段必须写明理由，且必须保持零值（否则说明豁免理由已失效）
	exempt := map[string]string{
		"EmbeddingInput": "与 Messages 互斥，见 UnifiedRequest.Validate",
		"SystemParts":    "与 System 语义重叠且 Anthropic 优先取它，会掩盖 System 去向",
	}

	req := fullUnifiedRequest(t)
	value := reflect.ValueOf(*req)
	typ := value.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if reason, ok := exempt[field.Name]; ok {
			if !value.Field(i).IsZero() {
				t.Errorf("字段 %s 在豁免表里（%s）却被填了值，请更新豁免表或移除该值", field.Name, reason)
			}
			continue
		}

		if value.Field(i).IsZero() {
			t.Errorf("字段 %s 未填非零值：新增字段必须纳入 fullUnifiedRequest，否则 golden 覆盖不到它", field.Name)
		}
	}
}

// TestFromUnifiedCoverage_Golden 把满配统一请求在三个协议下的出站 body 冻结成 golden。
//
// 直调 adapter.FromUnified 而非 ProcessRequest：FromUnified 是纯函数（无 ctx、不读设置），
// 测的是纯字段映射终态。ProcessRequest 会先跑 clampUnifiedReasoning，
// 那是另一层语义，混进来会让「字段去向」不可读。
//
// styles 刻意硬编码而非 range formatAdapters：新协议注册后应当人工审一遍它的
// 出站形状再纳入基线，自动全跑会静默接受未审输出。
func TestFromUnifiedCoverage_Golden(t *testing.T) {
	t.Parallel()

	styles := []string{"openai", "openai-res", "anthropic"}
	for _, style := range styles {
		style := style
		t.Run(style, func(t *testing.T) {
			t.Parallel()

			// 每个 subtest 独立构造：Anthropic 出站会 snapshot/restore 消息级 reasoning，
			// 共享同一实例可能跨 subtest 干扰。
			req := fullUnifiedRequest(t)
			if err := req.Validate(); err != nil {
				t.Fatalf("fixture 未通过 Validate: %v", err)
			}

			adapter, err := getAdapterOrDefault(style)
			if err != nil {
				t.Fatalf("getAdapterOrDefault(%s) failed: %v", style, err)
			}

			out, err := adapter.FromUnified(req)
			if err != nil {
				t.Fatalf("FromUnified(%s) failed: %v", style, err)
			}

			wantPath := filepath.Join("testdata", "golden", "from_unified", style+".json")
			assertGoldenJSON(t, wantPath, out)
		})
	}
}
