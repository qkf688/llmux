package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

func TestApplyProviderSelectionResult_ReduceWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{ReduceWeight: true})

	if weightItems[1] != 6 {
		t.Fatalf("expected weight reduced to 6, got %d", weightItems[1])
	}
	if priorityItems[1] != 7 {
		t.Fatalf("expected priority unchanged, got %d", priorityItems[1])
	}
}

func TestApplyProviderSelectionResult_RemoveWeightAndPriority(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{
		RemoveWeight:   true,
		RemovePriority: true,
	})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; ok {
		t.Fatal("expected priority entry removed")
	}
}

func TestApplyProviderSelectionResult_RemoveOnlyWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{RemoveWeight: true})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; !ok {
		t.Fatal("expected priority entry kept")
	}
}

func TestBuildRequestBodyForProvider_OpenAI_MissingToolCallFunctionName_ReturnsHTTP400(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{
		"model":"m",
		"messages":[
			{
				"role":"assistant",
				"tool_calls":[
					{"type":"function","function":{"arguments":"{}"}}
				]
			}
		]
	}`)

	_, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            string(consts.StyleOpenAI),
		ProviderType:     providers.TypeOpenAI,
		Raw:              raw,
		MaxTokensLimit:   nil,
		SupportsThinking: true,
	})
	if skip {
		t.Fatalf("skipProvider = true, want false")
	}
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var statusCoder interface{ StatusCode() int }
	if !errors.As(err, &statusCoder) {
		t.Fatalf("expected status-codable error, got %T: %v", err, err)
	}
	if got := statusCoder.StatusCode(); got != http.StatusBadRequest {
		t.Fatalf("StatusCode() = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestBuildRequestBodyForProvider_OpenAI_ValidToolCall_Passes(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`{
		"model":"m",
		"messages":[
			{
				"role":"assistant",
				"tool_calls":[
					{"type":"function","function":{"name":"container_shell","arguments":"{}"}}
				]
			}
		]
	}`)

	got, skip, err := buildRequestBodyForProvider(ctx, ProviderRequestCaps{
		Style:            string(consts.StyleOpenAI),
		ProviderType:     providers.TypeOpenAI,
		Raw:              raw,
		MaxTokensLimit:   nil,
		SupportsThinking: true,
	})
	if skip {
		t.Fatalf("skipProvider = true, want false")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) == "" {
		t.Fatalf("expected non-empty body")
	}
}

func TestStripThinkingFields_Unsupported_RemovesFields(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"reasoning_effort":"high","messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["thinking"]; ok {
		t.Fatal("expected thinking field removed")
	}
	if _, ok := obj["reasoning_effort"]; ok {
		t.Fatal("expected reasoning_effort field removed")
	}
	if _, ok := obj["messages"]; !ok {
		t.Fatal("expected messages field kept")
	}
}

func TestStripThinkingFields_Supported_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"messages":[]}`)
	got := stripThinkingFields(body, true)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged when supportsThinking=true, got %s", got)
	}
}

func TestStripThinkingFields_NoThinkingFields_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","messages":[]}`)
	got := stripThinkingFields(body, false)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged when no thinking fields, got %s", got)
	}
}

func TestStripThinkingFields_InvalidJSON_Unchanged(t *testing.T) {
	body := []byte(`not-json`)
	got := stripThinkingFields(body, false)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged for invalid JSON, got %s", got)
	}
}

func TestStripThinkingFields_Reasoning_Removed(t *testing.T) {
	body := []byte(`{"model":"m","reasoning":{"effort":"high"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["reasoning"]; ok {
		t.Fatal("expected reasoning field removed")
	}
}

// TestStripThinkingFields_PartialFields 真实场景：请求只带部分 thinking 键
// （Anthropic 只带 thinking、OpenAI 只带 reasoning_effort），缺失键应原样跳过。
func TestStripThinkingFields_PartialFields(t *testing.T) {
	body := []byte(`{"model":"m","thinking":{"type":"enabled","budget_tokens":20000},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["thinking"]; ok {
		t.Fatal("expected thinking field removed")
	}
	if _, ok := obj["reasoning_effort"]; ok {
		t.Fatal("unexpected reasoning_effort present")
	}
	if _, ok := obj["messages"]; !ok {
		t.Fatal("expected messages field kept")
	}
}

// TestStripThinkingFields_OutputConfigEffort_RemovedAndCleaned：
// Anthropic output_config.effort 是 adaptive thinking 字段，不支持 thinking 时必须删除，
// 删完后若 output_config 变空对象再删整个 output_config（避免残留空对象）。
func TestStripThinkingFields_OutputConfigEffort_RemovedAndCleaned(t *testing.T) {
	body := []byte(`{"model":"m","output_config":{"effort":"high"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := obj["output_config"]; ok {
		t.Fatal("expected output_config removed (was {effort:high}, after strip effort only key left → empty object → removed)")
	}
}

// TestStripThinkingFields_MessagesThinkingBlock_Removed：
// Anthropic 扩展思考多轮会话里 assistant 轮 content 数组内嵌 type=thinking 块，
// 路由到不支持 thinking 的上游时同样触发 400，需在裁剪顶层字段时一并剥离。
// 断言：thinking 块删除、同轮 text/tool_use 块保留、其他轮次不动。
func TestStripThinkingFields_MessagesThinkingBlock_Removed(t *testing.T) {
	body := []byte(`{` +
		`"model":"m",` +
		`"messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"let me think","signature":"sig-1"},` +
		`{"type":"text","text":"answer"},` +
		`{"type":"tool_use","id":"tu_1","name":"foo","input":{}}` +
		`]},` +
		`{"role":"user","content":"next"}` +
		`]}`)

	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	msgs := obj["messages"].([]any)
	asst := msgs[1].(map[string]any)
	content := asst["content"].([]any)

	for _, blk := range content {
		bm := blk.(map[string]any)
		if bm["type"] == "thinking" {
			t.Fatalf("expected thinking block removed from assistant content, got %v", content)
		}
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 blocks left (text+tool_use), got %d: %v", len(content), content)
	}
	if content[0].(map[string]any)["type"] != "text" {
		t.Fatalf("expected first remaining block = text, got %v", content[0])
	}
	if content[1].(map[string]any)["type"] != "tool_use" {
		t.Fatalf("expected second remaining block = tool_use, got %v", content[1])
	}
	// 其他轮次不动
	if msgs[0].(map[string]any)["content"] != "hi" {
		t.Fatalf("expected user content unchanged, got %v", msgs[0])
	}
	if msgs[2].(map[string]any)["content"] != "next" {
		t.Fatalf("expected last user content unchanged, got %v", msgs[2])
	}
}

// TestStripThinkingFields_MessagesRedactedThinkingBlock_Removed：
// redacted_thinking 块（Anthropic 不透明思考密文）同样需剥离。
func TestStripThinkingFields_MessagesRedactedThinkingBlock_Removed(t *testing.T) {
	body := []byte(`{` +
		`"model":"m",` +
		`"messages":[` +
		`{"role":"assistant","content":[` +
		`{"type":"redacted_thinking","data":"opaque-1"},` +
		`{"type":"redacted_thinking","data":"opaque-2"},` +
		`{"type":"text","text":"ok"}` +
		`]}` +
		`]}`)

	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	content := obj["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 block left (text), got %d: %v", len(content), content)
	}
	if content[0].(map[string]any)["type"] != "text" {
		t.Fatalf("expected remaining block = text, got %v", content[0])
	}
}

// TestStripThinkingFields_MessagesThinkingBlock_Supported_Unchanged：
// supportsThinking=true 时 messages 内嵌 thinking 块不裁剪（与顶层字段一致）。
func TestStripThinkingFields_MessagesThinkingBlock_Supported_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","messages":[` +
		`{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"x","signature":"s"},` +
		`{"type":"text","text":"y"}` +
		`]}]}`)

	got := stripThinkingFields(body, true)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged when supportsThinking=true, got %s", got)
	}
}

// TestStripThinkingFields_MessagesStringContent_Unchanged：
// content 为 string（非数组）时不应触发数组遍历逻辑，原样返回。
func TestStripThinkingFields_MessagesStringContent_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}]}`)
	got := stripThinkingFields(body, false)
	if string(got) != string(body) {
		t.Fatalf("expected unchanged for string content, got %s", got)
	}
}

// TestStripThinkingFields_OutputConfigWithOtherFields_KeepsShell：
// output_config 含 effort 之外的字段时，删 effort 后 output_config 不空，应保留。
func TestStripThinkingFields_OutputConfigWithOtherFields_KeepsShell(t *testing.T) {
	body := []byte(`{"model":"m","output_config":{"effort":"high","format":"json"},"messages":[]}`)
	got := stripThinkingFields(body, false)

	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	oc, ok := obj["output_config"].(map[string]any)
	if !ok {
		t.Fatal("expected output_config kept (has format field left)")
	}
	if _, ok := oc["effort"]; ok {
		t.Fatal("expected output_config.effort removed")
	}
	if oc["format"] != "json" {
		t.Fatalf("expected output_config.format=json kept, got %v", oc["format"])
	}
}

func TestClampMaxTokens_OverLimit_Clamps(t *testing.T) {
	limit := 8192
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, _ := obj["max_tokens"].(float64); int(v) != 8192 {
		t.Fatalf("expected max_tokens=8192, got %v", obj["max_tokens"])
	}
}

func TestClampMaxTokens_UnderLimit_Unchanged(t *testing.T) {
	limit := 8192
	body := []byte(`{"model":"m","max_tokens":100,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_NilLimit_Unchanged(t *testing.T) {
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_ZeroLimit_Unchanged(t *testing.T) {
	zero := 0
	body := []byte(`{"model":"m","max_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &zero)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestClampMaxTokens_MaxCompletionTokens_Clamped(t *testing.T) {
	limit := 4096
	body := []byte(`{"model":"m","max_completion_tokens":1048576,"messages":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, _ := obj["max_completion_tokens"].(float64); int(v) != 4096 {
		t.Fatalf("expected max_completion_tokens=4096, got %v", obj["max_completion_tokens"])
	}
}

// openai-res 出站体的输出上限键是 max_output_tokens（`service/responses/transform_to_responses.go`
// 把 unified.MaxTokens 写进 ResponsesRequest.MaxOutputTokens）。该键曾不在钳制字段表里，
// 使 MaxTokensLimit 对 Responses 上游完全空转、且无任何报错可察觉。
func TestClampMaxTokens_MaxOutputTokens_Clamped(t *testing.T) {
	limit := 2048
	body := []byte(`{"model":"m","max_output_tokens":1048576,"input":[]}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, _ := obj["max_output_tokens"].(float64); int(v) != 2048 {
		t.Fatalf("expected max_output_tokens=2048, got %v", obj["max_output_tokens"])
	}
}

// 客户端同时传多个上限键时每个都要压住：只钳第一个命中的会留下超限值直通上游。
func TestClampMaxTokens_AllOutputLimitFields_Clamped(t *testing.T) {
	limit := 512
	body := []byte(`{"model":"m","max_tokens":9000,"max_completion_tokens":9000,"max_output_tokens":9000}`)
	got, err := clampMaxTokens(body, &limit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	for _, field := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
		if v, _ := obj[field].(float64); int(v) != 512 {
			t.Fatalf("expected %s=512, got %v", field, obj[field])
		}
	}
}

// TestExecuteSingleProviderAttempt_NonOKResponse_BackfillsProxyTime：
// 上游返回非 200（handleNonOKProviderResponse 分支）时，错误日志的 ProxyTime 必须
// 回填端到端耗时，而不是留在建行瞬间的近零快照（建行发生在 Client.Do 之前）。
// 构造要点：上游 sleep 300ms 再回 502——用真实耗时区分「回填值」与「建行快照」。
// 注意不能回拨 Start：回拨会让建行快照本身≈回拨量，测试变成假绿（自证循环）。
func TestExecuteSingleProviderAttempt_NonOKResponse_BackfillsProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
	}))
	defer srv.Close()

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error", got.Status)
	}
	if got.ProxyTime < 250*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≈ 300ms（非 200 响应回填端到端耗时），当前仍是建行近零快照", got.ProxyTime)
	}
}

// TestExecuteSingleProviderAttempt_DoError_BackfillsProxyTime：
// 上游连接阶段失败（Client.Do 返回错误）时同样回填 ProxyTime。
// 构造要点：上游 sleep 500ms 而客户端 Timeout=200ms → Do 在 200ms 处返回超时错误，
// 真实 elapsed ≈200ms 可测；若只凭回拨 Start，建行快照会吸收回拨量导致假绿。
func TestExecuteSingleProviderAttempt_DoError_BackfillsProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		ChatModel:         chatModel,
		Client:            &http.Client{Timeout: 200 * time.Millisecond},
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error", got.Status)
	}
	if got.ProxyTime < 150*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≈ 200ms（连接超时回填端到端耗时），当前仍是建行近零快照", got.ProxyTime)
	}
}

// TestExecuteSingleProviderAttempt_TransformResponseError_BackfillsProxyTime：
// 协议转换失败（transform response err 分支）时错误日志同样回填 ProxyTime。
// 构造：style=openai × 上游 type=anthropic（wire format 不同 → 强制转换）+
// 上游 sleep 300ms 后返回 200 + 非 JSON body → ProcessResponse 解析必失败。
// 该出口的回填与建行之间隔着真实上游耗时，删除回填后落库值跌回建行近零快照、
// 断言变红——与 Do err / 非 200 用例同构，真正锁定回填行为。
func TestExecuteSingleProviderAttempt_TransformResponseError_BackfillsProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeAnthropic}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	chatModel, err := providers.New(providers.TypeAnthropic, fmt.Sprintf(`{"base_url":%q,"api_key":"k","version":"2023-06-01","auth_type":"x-api-key"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error（转换失败应落 error 日志）", got.Status)
	}
	if got.ProxyTime < 250*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≈ 300ms（转换失败回填端到端耗时），当前仍是建行近零快照", got.ProxyTime)
	}
}

// buildReqErrProvider 是 BuildReq 必失败的 providers.Provider stub。
type buildReqErrProvider struct{}

func (buildReqErrProvider) BuildReq(ctx context.Context, header http.Header, model string, rawData []byte) (*http.Request, error) {
	return nil, errors.New("build req failed")
}

func (buildReqErrProvider) Models(ctx context.Context) ([]providers.Model, error) { return nil, nil }

func (buildReqErrProvider) GetProxy() string { return "" }

// TestExecuteSingleProviderAttempt_BuildReqError_RetryLogCarriesProxyTime：
// BuildReq 失败走 retryLog 通道（recordRetryLog 侧 Create 落库），断言通道产物为
// error 状态且 ProxyTime 非零。
// 局限说明（有意如此）：该出口的「回填」与建行占位只差一次 BuildReq 调用（微秒级），
// 断言无法区分回填语句是否被删除——回拨量会被建行占位吸收，删了同样绿。本用例锁定
// 的是「此出口经通道产出 error 日志且耗时值非零」的链路完整性；回填语句本身由
// chat_attempt.go 的强制约定注释 + 代码审阅保证。
func TestExecuteSingleProviderAttempt_BuildReqError_RetryLogCarriesProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	retryChan := make(chan models.ChatLog, 1)
	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now().Add(-300 * time.Millisecond),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		ChatModel:         buildReqErrProvider{},
		Client:            &http.Client{},
	}, retryChan)

	var got models.ChatLog
	select {
	case got = <-retryChan:
	case <-time.After(time.Second):
		t.Fatal("retryLog channel got no entry")
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error", got.Status)
	}
	if got.ProxyTime < 250*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want 非近零（回拨 300ms 下占位/回填均应为该量级）", got.ProxyTime)
	}
}
