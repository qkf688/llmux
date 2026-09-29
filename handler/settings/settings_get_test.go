package settings

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
)

// TestBuildSettingsResponse_HealthKeys_NoWarn 通用设置页拼装不得对已知归属边界刷警告（Stage 36）：
// 健康检测系设置走独立域（health.go），在通用设置页没有位置。此前每读到一个这样的键就
// 记一条「failed to set setting field」，一次 GET /settings 固定刷 7 条——把真警告淹掉、
// 也把人往「组装出错」的错方向带。
func TestBuildSettingsResponse_HealthKeys_NoWarn(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rows := []models.Setting{
		{Key: models.SettingKeyHealthCheckEnabled, Value: "true"},
		{Key: models.SettingKeyHealthCheckInterval, Value: "30"},
		{Key: models.SettingKeyHealthCheckFailureThreshold, Value: "5"},
		{Key: models.SettingKeyHealthCheckFailureDisableEnabled, Value: "false"},
		{Key: models.SettingKeyHealthCheckAutoEnable, Value: "true"},
		{Key: models.SettingKeyHealthCheckLogRetentionCount, Value: "500"},
		{Key: models.SettingKeyHealthCheckCheckDisabledOnly, Value: "true"},
	}

	if _, err := buildSettingsResponse(rows); err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}
	if strings.Contains(logs.String(), "failed to set setting field") {
		t.Errorf("已知归属边界被当成组装失败上报，每次打开设置页都会刷噪声:\n%s", logs.String())
	}
}

// TestBuildSettingsResponse_SharedHealthKeys_StillApplied 反向锁定：健康检测域里
// **同时属于通用设置页**的两个开关必须照旧被填上（按域粗筛会误伤它们）。
func TestBuildSettingsResponse_SharedHealthKeys_StillApplied(t *testing.T) {
	rows := []models.Setting{
		{Key: models.SettingKeyHealthCheckCountAsSuccess, Value: "false"},
		{Key: models.SettingKeyHealthCheckCountAsFailure, Value: "true"},
	}

	response, err := buildSettingsResponse(rows)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}
	if response.CountHealthCheckAsSuccess {
		t.Error("count_health_check_as_success = true, want false（通用设置页共享的键不得被分流误伤）")
	}
	if !response.CountHealthCheckAsFailure {
		t.Error("count_health_check_as_failure = false, want true（通用设置页共享的键不得被分流误伤）")
	}
}

// TestGenericSettingsResponseFieldContract 通用设置响应字段契约（Stage 36 机械门）：
// ① 每个非 health 域的设置项都必须在通用响应结构里有位置——否则它在设置页上永远显示默认值，
// 而这种「漏字段」正是分流静默跳过的副作用，必须由门挡住，不靠人盯；
// ② 该字段还必须在**响应体 JSON 里真的出现**（按 json tag 取键名，禁 `json:"-"` / 空 tag）——
// 只断 Go 字段存在会让「加了字段忘写 tag、或写成不序列化」双绿，而前端是按 snake_case 键读的；
// ③ 反向锁定 health 域里被通用页复用的那一组（集合相等），防按域粗筛时误伤。
func TestGenericSettingsResponseFieldContract(t *testing.T) {
	// 通用设置页共享的 health 键（有意为之的一份清单，改动必须显式过这里）
	expectedShared := map[string]bool{
		models.SettingKeyHealthCheckCountAsSuccess: true,
		models.SettingKeyHealthCheckCountAsFailure: true,
	}
	actualShared := map[string]bool{}
	responseType := reflect.TypeOf(SettingsResponse{})

	raw, err := json.Marshal(SettingsResponse{})
	if err != nil {
		t.Fatalf("marshal SettingsResponse: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal SettingsResponse: %v", err)
	}

	for _, schema := range models.SettingSchemas() {
		field, present := responseType.FieldByName(schema.Field)

		if schema.Category == "health" {
			if present {
				actualShared[schema.Key] = true
			}
			continue
		}

		if !present {
			t.Errorf("设置项 %s（字段 %s）在通用设置响应里没有位置：要么补字段，要么把它的归属改成独立域（静默跳过的代价必须由本门兜住）",
				schema.Key, schema.Field)
			continue
		}

		jsonKey := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonKey == "" || jsonKey == "-" {
			t.Errorf("设置项 %s 的字段 %s 没有可用的 json 键（tag=%q）：前端按 snake_case 键读，加字段必须同时写 tag",
				schema.Key, schema.Field, field.Tag.Get("json"))
			continue
		}
		if _, ok := wire[jsonKey]; !ok {
			t.Errorf("设置项 %s 的 json 键 %q 没出现在响应体里（前端读不到）", schema.Key, jsonKey)
		}
	}

	if !reflect.DeepEqual(actualShared, expectedShared) {
		t.Errorf("通用设置页共享的 health 键集合 = %v, want %v", actualShared, expectedShared)
	}
}

func TestDefaultSettingsResponse(t *testing.T) {
	response, err := buildSettingsResponse(nil)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}

	if !response.StrictCapabilityMatch {
		t.Fatalf("expected StrictCapabilityMatch default to be true")
	}
	if response.AutoWeightDecayDefault != 100 {
		t.Fatalf("expected AutoWeightDecayDefault default to be 100, got %d", response.AutoWeightDecayDefault)
	}
	if response.ModelSyncInterval != 12 {
		t.Fatalf("expected ModelSyncInterval default to be 12, got %d", response.ModelSyncInterval)
	}
	if response.CredHealthCooldown429Sec != 60 {
		t.Fatalf("expected CredHealthCooldown429Sec default to be 60, got %d", response.CredHealthCooldown429Sec)
	}
	if response.CredHealthCooldownServerSec != 60 {
		t.Fatalf("expected CredHealthCooldownServerSec default to be 60, got %d", response.CredHealthCooldownServerSec)
	}
	if response.CredHealthAuthFailThreshold != 3 {
		t.Fatalf("expected CredHealthAuthFailThreshold default to be 3, got %d", response.CredHealthAuthFailThreshold)
	}
	if response.CredHealthProbeIntervalSec != 60 {
		t.Fatalf("expected CredHealthProbeIntervalSec default to be 60, got %d", response.CredHealthProbeIntervalSec)
	}
	if response.ReasoningEffortDefaultValue != "low" {
		t.Fatalf("expected ReasoningEffortDefaultValue default to be low, got %s", response.ReasoningEffortDefaultValue)
	}
	if response.LogRawRequestResponseErrorsOnly {
		t.Fatalf("expected LogRawRequestResponseErrorsOnly default to be false")
	}
	// Stage 2-C：上游最大响应体默认 32 MiB（文生图 base64 合法场景 ×4 余量，见 schema 注释）。
	if response.UpstreamMaxResponseMiB != 32 {
		t.Fatalf("expected UpstreamMaxResponseMiB default to be 32, got %d", response.UpstreamMaxResponseMiB)
	}
	// Stage 2-D：原始采集字段上限默认 256 KiB（普通响应完整、异常大响应截尾）。
	if response.RawLogFieldMaxKiB != 256 {
		t.Fatalf("expected RawLogFieldMaxKiB default to be 256, got %d", response.RawLogFieldMaxKiB)
	}
	// Stage 23：回复日志单条上限默认 2 MiB（「看全文」口径，调试开关场景）。
	if response.ResponseLogMaxKiB != 2048 {
		t.Fatalf("expected ResponseLogMaxKiB default to be 2048, got %d", response.ResponseLogMaxKiB)
	}
	// 缓存亲和默认开启：粘性选路是纯优化（失效无害退化为现状选择），默认开让缓存省钱即刻生效。
	if !response.CacheAffinityEnabled {
		t.Fatalf("expected CacheAffinityEnabled default to be true")
	}
}

func TestApplySettingToResponse(t *testing.T) {
	settings := []models.Setting{
		{Key: models.SettingKeyAutoWeightDecay, Value: "true"},
		{Key: models.SettingKeyAutoWeightDecayDefault, Value: "88"},
		{Key: models.SettingKeyLogRawRequestResponseErrorsOnly, Value: "true"},
		{Key: models.SettingKeyModelSyncFilterRules, Value: `["gpt","claude"]`},
		{Key: models.SettingKeyReasoningEffortDefaultValue, Value: "high"},
	}

	response, err := buildSettingsResponse(settings)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}

	if !response.AutoWeightDecay {
		t.Fatalf("expected AutoWeightDecay to be true")
	}
	if response.AutoWeightDecayDefault != 88 {
		t.Fatalf("expected AutoWeightDecayDefault to be 88, got %d", response.AutoWeightDecayDefault)
	}
	if len(response.ModelSyncFilterRules) != 2 {
		t.Fatalf("expected ModelSyncFilterRules length to be 2, got %d", len(response.ModelSyncFilterRules))
	}
	if response.ModelSyncFilterRules[0] != "gpt" || response.ModelSyncFilterRules[1] != "claude" {
		t.Fatalf("unexpected ModelSyncFilterRules value: %#v", response.ModelSyncFilterRules)
	}
	if response.ReasoningEffortDefaultValue != "high" {
		t.Fatalf("expected ReasoningEffortDefaultValue to be high, got %s", response.ReasoningEffortDefaultValue)
	}
	if !response.LogRawRequestResponseErrorsOnly {
		t.Fatalf("expected LogRawRequestResponseErrorsOnly to be true")
	}
}

func TestApplySettingToResponseKeepsDefaultsOnInvalidValues(t *testing.T) {
	settings := []models.Setting{
		{Key: models.SettingKeyAutoWeightDecayDefault, Value: "invalid-int"},
		{Key: models.SettingKeyTemplateFuzzyMatchSeparators, Value: "invalid-json"},
	}

	response, err := buildSettingsResponse(settings)
	if err != nil {
		t.Fatalf("buildSettingsResponse error: %v", err)
	}

	if response.AutoWeightDecayDefault != 100 {
		t.Fatalf("expected AutoWeightDecayDefault to keep default 100, got %d", response.AutoWeightDecayDefault)
	}
	if len(response.TemplateFuzzyMatchSeparators) != 2 {
		t.Fatalf("expected TemplateFuzzyMatchSeparators to keep default length 2, got %d", len(response.TemplateFuzzyMatchSeparators))
	}
	if response.TemplateFuzzyMatchSeparators[0] != ":" || response.TemplateFuzzyMatchSeparators[1] != "-" {
		t.Fatalf("unexpected TemplateFuzzyMatchSeparators value: %#v", response.TemplateFuzzyMatchSeparators)
	}
}
