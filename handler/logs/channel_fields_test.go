package logs

import (
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// 调度明细四键（endpoint_protocol / endpoint_url / key_group_name / credential_note）
// 是 S3-3 起的响应契约：前端 logs 展示层（S0 原型）已按同名 snake_case 键消费，
// 后端 DTO 漏写任意一键都会让该列静默空白——与 usage_source 同理，用契约测试锁死。
func TestGetRequestLogs_ReturnsChannelSelectionFields(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:             "m1",
		ProviderName:     "p1",
		ProviderModel:    "pm1",
		Status:           "success",
		Style:            "openai",
		EndpointProtocol: "anthropic",
		EndpointURL:      "https://upstream.example/claude/v1",
		KeyGroupName:     "低价组",
		CredentialNote:   "#a1b2c3d4",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]
	for key, want := range map[string]string{
		"endpoint_protocol": "anthropic",
		"endpoint_url":      "https://upstream.example/claude/v1",
		"key_group_name":    "低价组",
		"credential_note":   "#a1b2c3d4",
	} {
		if item[key] != want {
			t.Fatalf("%s = %v, want %v（调度明细键缺失或值不符，前端展示列将静默空白）", key, item[key], want)
		}
	}
}

// 转换标签必须按**命中端点协议**（ChatLog.EndpointProtocol）回放，而非 Provider.Type：
// 多协议供应商（type=openai + 勾选 anthropic 端点）的请求实际按端点协议透传/转换，
// 按 Provider.Type 回放会给出与请求实际行为相反的标签（enrich.go 原分叉标注，S3-3 关闭）。
func TestGetRequestLogs_FormatConversionByEndpointProtocol(t *testing.T) {
	testsupport.InitTestDB(t)

	// type=openai 的供应商，但该请求命中 anthropic 端点（入站 openai → 出站 anthropic）。
	// 按 Provider.Type 回放会误判「同形无转换」；按端点协议判必须有转换。
	provider := models.Provider{Name: "dual", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	log := models.ChatLog{
		Name:             "m1",
		ProviderName:     "dual",
		Status:           "success",
		Style:            "openai",
		EndpointProtocol: "anthropic",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]
	if val, ok := item["has_format_conversion"].(bool); !ok || !val {
		t.Fatalf("has_format_conversion = %v, want true（入站 openai × 端点 anthropic = 有转换）", item["has_format_conversion"])
	}
	if item["source_format"] != "openai" {
		t.Fatalf("source_format = %v, want openai", item["source_format"])
	}
	if item["target_format"] != "anthropic" {
		t.Fatalf("target_format = %v, want anthropic（按端点协议，而非 Provider.Type=openai）", item["target_format"])
	}
}

// EndpointProtocol 为空（S3-3 之前的存量行）时判无转换：宁少报不误报——
// 回退 Provider.Type 回放会让多协议供应商的存量行拿到与实际行为相反的标签。
func TestGetRequestLogs_EmptyEndpointProtocolNoConversion(t *testing.T) {
	testsupport.InitTestDB(t)

	// Style=openai vs Provider.Type=anthropic：旧取数路径会判「有转换」；
	// EndpointProtocol 缺失时新语义必须判 false。
	provider := models.Provider{Name: "legacy", Type: "anthropic"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	log := models.ChatLog{
		Name:         "m1",
		ProviderName: "legacy",
		Status:       "success",
		Style:        "openai",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]
	if val, ok := item["has_format_conversion"].(bool); !ok || val {
		t.Fatalf("has_format_conversion = %v, want false（EndpointProtocol 缺失宁少报，不回退 Provider.Type 猜测）", item["has_format_conversion"])
	}
}
