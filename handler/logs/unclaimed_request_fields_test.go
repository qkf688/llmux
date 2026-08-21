package logs

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// unclaimed_request_fields 与 mismatched_request_fields 是两类读时计算的诊断字段：
// 前者指出客户端原始请求体里有哪些顶层键本网关根本没解析（转换后静默消失），后者指出
// 哪些键被认领了、但值类型不符而被宽容容器当成「没传」丢掉。
//
// 断言全部打在响应体 JSON 上而不是 buildChatLogResponse 的返回值上：这两个字段的价值
// 完全取决于前端能否区分四种 status，而键名、null 与空数组的差别只在序列化后才成立。

func detailResponseUnclaimed(t *testing.T, log *models.ChatLog) (map[string]any, bool) {
	t.Helper()
	return detailResponseDiagnostics(t, log, "unclaimed_request_fields")
}

func detailResponseDiagnostics(t *testing.T, log *models.ChatLog, key string) (map[string]any, bool) {
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

	raw, present := payload.Data[key]
	if !present {
		return nil, false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %v", key, raw)
	}
	return obj, true
}

func TestGetRequestLogDetail_UnclaimedRequestFieldsStatuses(t *testing.T) {
	cases := []struct {
		name       string
		style      consts.Style
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
				Style:          string(tc.style),
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
		Style:          string(consts.StyleOpenAI),
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

// mismatched_request_fields 是另一类静默：键被认领了、但值类型与入站 DTO 不符，被宽容
// 容器当成「没传」丢掉。断言同样打在响应体 JSON 上，理由同上。
func TestGetRequestLogDetail_MismatchedRequestFieldsStatuses(t *testing.T) {
	cases := []struct {
		name       string
		style      consts.Style
		rawBody    string
		wantStatus string
		wantFields []string
		wantDetail bool
	}{
		{
			// temperature 传字符串：openai 的 OptionalNumber 静默吞掉，请求 200 通过但参数不生效。
			name:       "openai_with_type_mismatch",
			style:      consts.StyleOpenAI,
			rawBody:    `{"model":"gpt-5","messages":[],"temperature":"0.5"}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{"temperature"},
		},
		{
			// 类型全对时 ok + 空数组。top_k 是未认领键，归 unclaimed_request_fields 报，不在这里。
			name:       "openai_all_types_match",
			style:      consts.StyleOpenAI,
			rawBody:    `{"model":"gpt-5","messages":[],"temperature":0.7,"top_k":40}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{},
		},
		{
			name:       "anthropic_with_type_mismatch",
			style:      consts.StyleAnthropic,
			rawBody:    `{"model":"claude","messages":[],"max_tokens":"lots"}`,
			wantStatus: unclaimedStatusOK,
			wantFields: []string{"max_tokens"},
		},
		{
			// openai-res 的入站 DTO 用裸类型/指针字段，类型不对时整条请求解析失败、
			// 不存在静默丢弃，故该项检测**不支持**它。这是本状态在生产的唯一可达路径。
			name:       "openai_res_unsupported",
			style:      consts.StyleOpenAIRes,
			rawBody:    `{"model":"gpt-5","input":[],"temperature":"0.5"}`,
			wantStatus: unclaimedStatusStyleUnsupported,
			wantFields: []string{},
		},
		{
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
				Style:          string(tc.style),
				RawRequestBody: tc.rawBody,
			}
			if err := models.DB.Create(&log).Error; err != nil {
				t.Fatalf("create log: %v", err)
			}

			obj, present := detailResponseDiagnostics(t, &log, "mismatched_request_fields")
			if !present {
				t.Fatal("detail response must include mismatched_request_fields")
			}

			if obj["status"] != tc.wantStatus {
				t.Fatalf("status = %v, want %v", obj["status"], tc.wantStatus)
			}

			rawFields, ok := obj["fields"]
			if !ok {
				t.Fatal("fields key must always be present")
			}
			// 空结果必须是 [] 而非 null，理由同 unclaimed_request_fields。
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

// 两块诊断必须彼此独立：同一份 body 里既有未认领键又有类型不匹配键时，各报各的，
// 不许混进对方的列表——用户据此判断该改网关还是改客户端。
func TestGetRequestLogDetail_UnclaimedAndMismatchedAreReportedSeparately(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		Status:         "success",
		Style:          string(consts.StyleOpenAI),
		RawRequestBody: `{"model":"gpt-5","messages":[],"temperature":"0.5","top_k":40}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	unclaimed, ok := detailResponseDiagnostics(t, &log, "unclaimed_request_fields")
	if !ok {
		t.Fatal("detail response must include unclaimed_request_fields")
	}
	if got := unclaimed["fields"]; !reflect.DeepEqual(got, []any{"top_k"}) {
		t.Fatalf("unclaimed fields = %v, want [top_k]", got)
	}

	mismatched, ok := detailResponseDiagnostics(t, &log, "mismatched_request_fields")
	if !ok {
		t.Fatal("detail response must include mismatched_request_fields")
	}
	if got := mismatched["fields"]; !reflect.DeepEqual(got, []any{"temperature"}) {
		t.Fatalf("mismatched fields = %v, want [temperature]", got)
	}
}

// mismatched 与 unclaimed 同一个 include_raw 门控：输入都是 RawRequestBody。
func TestGetRequestLogs_MismatchedRequestFieldsFollowsIncludeRawGate(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		Status:         "success",
		Style:          string(consts.StyleOpenAI),
		RawRequestBody: `{"model":"gpt-5","messages":[],"temperature":"0.5"}`,
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

	if _, ok := listItem("/logs?page=1&page_size=20")["mismatched_request_fields"]; ok {
		t.Fatal("mismatched_request_fields must be omitted when include_raw is not set")
	}

	item := listItem("/logs?page=1&page_size=20&include_raw=true")
	obj, ok := item["mismatched_request_fields"].(map[string]any)
	if !ok {
		t.Fatalf("mismatched_request_fields missing or not an object with include_raw=true: %v", item["mismatched_request_fields"])
	}
	if obj["status"] != unclaimedStatusOK {
		t.Fatalf("status = %v, want %v", obj["status"], unclaimedStatusOK)
	}
}
