package logs

import (
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// 密钥归因两键（api_key_id / api_key_name）是 /api/logs 列表与详情的响应契约。
// 断言必须打在响应体 JSON 上并区分「键存在且为 null」与「键缺失」
// （AGENTS §5：只测 Go 字段测不出 omitempty/键名类断层）。

func assertAttributionKeys(t *testing.T, res gjson.Result, wantID uint, wantName string) {
	t.Helper()
	idKey := res.Get("api_key_id")
	if !idKey.Exists() {
		t.Fatalf("api_key_id 键缺失（前端无法区分「无归因」与「后端没返字段」）")
	}
	if !res.Get("api_key_name").Exists() {
		t.Fatalf("api_key_name 键缺失（同上：缺键与空值必须可区分）")
	}
	if wantID == 0 {
		if idKey.Type != gjson.Null {
			t.Fatalf("api_key_id = %v, want null（无归因必须序列化成 null 而非省略/0）", idKey.Value())
		}
	} else if uint(idKey.Uint()) != wantID {
		t.Fatalf("api_key_id = %v, want %d", idKey.Value(), wantID)
	}
	if got := res.Get("api_key_name").String(); got != wantName {
		t.Fatalf("api_key_name = %q, want %q（键缺失或值不符）", got, wantName)
	}
}

func TestGetRequestLogs_ListCarriesAttributionKeys(t *testing.T) {
	testsupport.InitTestDB(t)

	callerID := uint(42)
	attributed := models.ChatLog{
		Name:         "m1",
		Status:       "success",
		Style:        "openai",
		ApiKeyID:     &callerID,
		ApiKeyName:   "codex-key",
		ProviderName: "p1",
	}
	unattributed := models.ChatLog{
		Name:         "m2",
		Status:       "error",
		Style:        "openai",
		ProviderName: "p1",
	}
	if err := models.DB.Create(&attributed).Error; err != nil {
		t.Fatalf("create attributed log: %v", err)
	}
	if err := models.DB.Create(&unattributed).Error; err != nil {
		t.Fatalf("create unattributed log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	byName := map[string]gjson.Result{}
	gjson.Get(w.Body.String(), "data.data").ForEach(func(_, v gjson.Result) bool {
		byName[v.Get("name").String()] = v
		return true
	})
	if len(byName) != 2 {
		t.Fatalf("logs length = %d, want 2", len(byName))
	}
	assertAttributionKeys(t, byName["m1"], callerID, "codex-key")
	assertAttributionKeys(t, byName["m2"], 0, "")
}

func TestGetRequestLogDetail_CarriesAttributionKeys(t *testing.T) {
	testsupport.InitTestDB(t)

	callerID := uint(7)
	log := models.ChatLog{
		Name:       "m1",
		Status:     "success",
		Style:      "openai",
		ApiKeyID:   &callerID,
		ApiKeyName: "claude-code",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10))
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	assertAttributionKeys(t, gjson.Get(w.Body.String(), "data"), callerID, "claude-code")
}

// 无归因行的详情同样必须带 null 态键（列表测过 null 不代表详情装配同路径）。
func TestGetRequestLogDetail_NullAttributionKeysPresent(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{Name: "m9", Status: "error", Style: "openai"}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10))
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	assertAttributionKeys(t, gjson.Get(w.Body.String(), "data"), 0, "")
}
