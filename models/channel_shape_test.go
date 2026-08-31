package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tidwall/gjson"
)

// channelResponseKeyContract 描述四表实体（直返实体，无 json tag 走 Go 字段名）
// 对外暴露的一个响应键的契约。断言打在 json.Marshal 输出上：
// handler 的 c.JSON 与 json.Marshal 共用 encoding/json，形状等价
// （S1 无 handler，S2 起 HTTP 契约测试在此基础上补全）。
type channelResponseKeyContract struct {
	key         string
	wantSetType gjson.Type
	// nilable 标记该键承载「未设置」这一有效语义：未设置时必须序列化成 JSON null
	// 且键仍存在，禁止 omitempty 省略整键（省略会让前端把继承/未设置误判成 override）。
	nilable bool
}

var (
	poolResponseKeys = []channelResponseKeyContract{
		{key: "Name", wantSetType: gjson.String},
		{key: "Note", wantSetType: gjson.String},
	}
	credentialResponseKeys = []channelResponseKeyContract{
		{key: "Key", wantSetType: gjson.String},
		{key: "KeyHash", wantSetType: gjson.String},
		{key: "Note", wantSetType: gjson.String},
		{key: "GroupID", wantSetType: gjson.Number, nilable: true},
		{key: "PoolID", wantSetType: gjson.Number, nilable: true},
		{key: "Status", wantSetType: gjson.String},
		{key: "CooldownUntil", wantSetType: gjson.String, nilable: true},
		{key: "CooldownReason", wantSetType: gjson.String},
		{key: "FailCount", wantSetType: gjson.Number},
		{key: "LastUsedAt", wantSetType: gjson.String, nilable: true},
		{key: "TotalRequests", wantSetType: gjson.Number},
		{key: "TotalErrors", wantSetType: gjson.Number},
		{key: "TotalTokens", wantSetType: gjson.Number},
	}
	endpointResponseKeys = []channelResponseKeyContract{
		{key: "ProviderID", wantSetType: gjson.Number},
		{key: "Protocol", wantSetType: gjson.String},
		{key: "URL", wantSetType: gjson.String},
		{key: "Enabled", wantSetType: gjson.True},
	}
	keyGroupResponseKeys = []channelResponseKeyContract{
		{key: "ProviderID", wantSetType: gjson.Number},
		{key: "Name", wantSetType: gjson.String},
		{key: "Weight", wantSetType: gjson.Number},
		{key: "Models", wantSetType: gjson.String},
		{key: "PoolID", wantSetType: gjson.Number, nilable: true},
	}
	providerProtocolsKeys = []channelResponseKeyContract{
		{key: "Protocols", wantSetType: gjson.JSON, nilable: true},
	}
)

func marshalShape(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return string(data)
}

// assertShapeKey 断言序列化输出指定路径上的键存在且类型符合预期。
// 用 Exists() 而非取值判空：键缺失与「存在且为 null」在取值上无法区分，
// 只断取值会让 omitempty 类断层全部漏过（与 handler/associations 的断言同源）。
func assertShapeKey(t *testing.T, raw, path string, wantType gjson.Type) {
	t.Helper()
	got := gjson.Get(raw, path)
	if !got.Exists() {
		t.Fatalf("%s 键缺失：直返实体响应键必须始终存在（三态字段禁 omitempty，nil 序列化为 null）。raw=%s", path, raw)
	}
	if got.Type != wantType {
		t.Fatalf("%s type = %v, want %v, raw=%s", path, got.Type, wantType, got.Raw)
	}
}

func checkShapeTable(t *testing.T, raw, prefix string, keys []channelResponseKeyContract, set bool) {
	for _, kc := range keys {
		want := kc.wantSetType
		if !set && kc.nilable {
			want = gjson.Null
		}
		assertShapeKey(t, raw, prefix+kc.key, want)
	}
}

// TestChannelEntities_KeysPresent_WhenSet 锁定四表实体所有业务字段在「有值态」下
// 键存在且类型正确（PascalCase 直返形状）。
func TestChannelEntities_KeysPresent_WhenSet(t *testing.T) {
	groupID, poolID := uint(11), uint(12)
	now := time.Now()
	creds := uint(3)

	pool := Pool{Name: "主池", Note: "生产密钥池"}
	credential := Credential{
		Key:            "hex-ciphertext",
		KeyHash:        "sha256-hex",
		Note:           "备注",
		GroupID:        &groupID,
		PoolID:         &poolID,
		Status:         CredentialStatusActive,
		CooldownUntil:  &now,
		CooldownReason: "429 限流",
		FailCount:      3,
		LastUsedAt:     &now,
		TotalRequests:  100,
		TotalErrors:    2,
		TotalTokens:    5000,
	}
	endpoint := Endpoint{ProviderID: 1, Protocol: "openai", URL: "https://x.example.com", Enabled: true}
	keyGroup := KeyGroup{ProviderID: 2, Name: "低价组", Weight: 3, Models: "gpt-4o-mini", PoolID: &creds}
	provider := Provider{Protocols: []string{"openai", "responses"}}

	checkShapeTable(t, marshalShape(t, pool), "", poolResponseKeys, true)
	checkShapeTable(t, marshalShape(t, credential), "", credentialResponseKeys, true)
	checkShapeTable(t, marshalShape(t, endpoint), "", endpointResponseKeys, true)
	checkShapeTable(t, marshalShape(t, keyGroup), "", keyGroupResponseKeys, true)
	checkShapeTable(t, marshalShape(t, provider), "", providerProtocolsKeys, true)
}

// TestChannelEntities_KeysPresent_WhenUnset 是 omitempty 类断层的主防线：
// 三态字段全为 nil 时，键必须仍在且为 JSON null（GroupID/PoolID/CooldownUntil/
// LastUsedAt/Provider.Protocols）。
func TestChannelEntities_KeysPresent_WhenUnset(t *testing.T) {
	credential := Credential{Key: "hex", KeyHash: "h", Status: CredentialStatusActive}
	keyGroup := KeyGroup{ProviderID: 2, Name: "默认组"}
	provider := Provider{}

	checkShapeTable(t, marshalShape(t, credential), "", credentialResponseKeys, false)
	checkShapeTable(t, marshalShape(t, keyGroup), "", keyGroupResponseKeys, false)
	checkShapeTable(t, marshalShape(t, provider), "", providerProtocolsKeys, false)
}

// TestIsValidCredentialStatus 锁凭据状态枚举的合法值集合（OCP 收敛点，
// #6-2 起 temp_unsched 入列）：handler 的 CRUD/筛选经此白名单反射放行，
// 新增状态只改 models 侧（含本表），handler 不另设白名单。
func TestIsValidCredentialStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{CredentialStatusActive, true},
		{CredentialStatusDisabled, true},
		{CredentialStatusError, true},
		{CredentialStatusTempUnsched, true},
		{"cooldown", false}, // 冷却不占状态位（CooldownUntil 表达），防枚举误收
		{"", false},
		{"ACTIVE", false},
		{"bogus", false},
	}
	for _, tc := range cases {
		if got := IsValidCredentialStatus(tc.status); got != tc.want {
			t.Fatalf("IsValidCredentialStatus(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
