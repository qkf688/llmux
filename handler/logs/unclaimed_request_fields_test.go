package logs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// unclaimed_request_fields 是读时计算的诊断字段：指出客户端原始请求体里有哪些顶层键
// 本网关根本没解析（转换后会静默消失）。
//
// 断言全部打在响应体 JSON 上而不是 buildChatLogResponse 的返回值上：这个字段的价值
// 完全取决于前端能否区分四种 status，而键名、null 与空数组的差别只在序列化后才成立。

func detailResponseUnclaimed(t *testing.T, log *models.ChatLog) (map[string]any, bool) {
	t.Helper()

	id := strconv.FormatUint(uint64(log.ID), 10)
	c, w := testsupport.NewTestContext("GET", "/logs/"+id)
	c.Params = gin.Params{{Key: "id", Value: id}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}

	raw, present := payload.Data["unclaimed_request_fields"]
	if !present {
		return nil, false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("unclaimed_request_fields is not an object: %v", raw)
	}
	return obj, true
}

func TestGetRequestLogDetail_UnclaimedRequestFieldsStatuses(t *testing.T) {
	cases := []struct {
		name       string
		style      string
		rawBody    string
		wantStatus string
		wantFields []string
		wantDetail bool
	}{
		{
			// top_k 是真实场景：部分客户端会带它，OpenAI Chat Completions DTO 不认领，
			// 转换后就悄悄没了。这类键正是本字段要暴露的对象。
			name:       "openai_with_unclaimed_key",
			style:      consts.StyleOpenAI,
			rawBody:    `{"model":"gpt-5","messages":[],"top_k":40}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{"top_k"},
		},
		{
			// 全部键都被认领时必须是 ok + 空数组，而不是把「无问题」和「查不了」混为一谈。
			name:       "openai_fully_claimed",
			style:      consts.StyleOpenAI,
			rawBody:    `{"model":"gpt-5","messages":[],"temperature":0.7}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{},
		},
		{
			// Responses 协议把 messages 叫 input，所以同一份 body 在这边 messages 也未认领。
			name:       "openai_res_uses_its_own_key_set",
			style:      consts.StyleOpenAIRes,
			rawBody:    `{"model":"gpt-5","messages":[],"input":[]}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{"messages"},
		},
		{
			// anthropic 入站已有请求 DTO（service/anthropic/request_dto.go）：
			// system / messages 被认领，top_k 是 Anthropic 真实 API 有而网关未解析的键。
			name:       "anthropic_uses_its_own_key_set",
			style:      consts.StyleAnthropic,
			rawBody:    `{"model":"claude","system":"s","messages":[],"top_k":40}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{"top_k"},
		},
		{
			// 未注册的 style 必须显式报不支持——返回空数组会被读成「已检查、无未知字段」，
			// 即假阴性。当前三个生产 style 均已注册，故这里用一个不存在的 style 覆盖该分支。
			name:       "unregistered_style_unsupported",
			style:      "some-future-style",
			rawBody:    `{"model":"m","system":"s"}`,
			wantStatus: unclaimedStatusStyleUnsupported,
			wantFields: []string{},
		},
		{
			// raw_request_body 开关默认全关，且 errors_only 会在成功时清空它。
			// 前端必须能把这种「没数据可查」与「查过没问题」分开。
			name:       "raw_not_recorded",
			style:      consts.StyleOpenAI,
			rawBody:    "",
			wantStatus: unclaimedStatusRawNotRecorded,
			wantFields: []string{},
		},
		{
			name:       "parse_error",
			style:      consts.StyleOpenAI,
			rawBody:    `{"model":`,
			wantStatus: unclaimedStatusParseError,
			wantFields: []string{},
			wantDetail: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.InitTestDB(t)

			log := models.ChatLog{
				Name:           "m1",
				ProviderName:   "p1",
				Status:         "success",
				Style:          tc.style,
				RawRequestBody: tc.rawBody,
			}
			if err := models.DB.Create(&log).Error; err != nil {
				t.Fatalf("create log: %v", err)
			}

			obj, present := detailResponseUnclaimed(t, &log)
			if !present {
				t.Fatal("detail response must include unclaimed_request_fields")
			}

			if obj["status"] != tc.wantStatus {
				t.Fatalf("status = %v, want %v", obj["status"], tc.wantStatus)
			}

			rawFields, ok := obj["fields"]
			if !ok {
				t.Fatal("fields key must always be present")
			}
			// 空结果必须序列化成 [] 而非 null：null 会让前端多一种需要特判的状态。
			if rawFields == nil {
				t.Fatal("fields must be [] rather than null when empty")
			}
			fields, ok := rawFields.([]any)
			if !ok {
				t.Fatalf("fields is not an array: %v", rawFields)
			}
			if len(fields) != len(tc.wantFields) {
				t.Fatalf("fields = %v, want %v", fields, tc.wantFields)
			}
			for i, want := range tc.wantFields {
				if fields[i] != want {
					t.Fatalf("fields[%d] = %v, want %v", i, fields[i], want)
				}
			}

			if _, hasDetail := obj["detail"]; hasDetail != tc.wantDetail {
				t.Fatalf("detail present = %v, want %v (obj=%v)", hasDetail, tc.wantDetail, obj)
			}
		})
	}
}

// 列表接口默认不返回该字段：它的输入 RawRequestBody 在 include_raw=false 时压根没从库里
// 读出来，此时算出来的只会是假的 raw_not_recorded。必须与 raw 组同进同出。
func TestGetRequestLogs_UnclaimedRequestFieldsFollowsIncludeRawGate(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		Status:         "success",
		Style:          consts.StyleOpenAI,
		RawRequestBody: `{"model":"gpt-5","messages":[],"top_k":40}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	listItem := func(query string) map[string]any {
		c, w := testsupport.NewTestContext("GET", query)
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
		return payload.Data.Data[0]
	}

	if _, ok := listItem("/logs?page=1&page_size=20")["unclaimed_request_fields"]; ok {
		t.Fatal("unclaimed_request_fields must be omitted when include_raw is not set")
	}

	item := listItem("/logs?page=1&page_size=20&include_raw=true")
	obj, ok := item["unclaimed_request_fields"].(map[string]any)
	if !ok {
		t.Fatalf("unclaimed_request_fields missing or not an object with include_raw=true: %v", item["unclaimed_request_fields"])
	}
	if obj["status"] != unclaimedStatusOK {
		t.Fatalf("status = %v, want %v", obj["status"], unclaimedStatusOK)
	}
}
