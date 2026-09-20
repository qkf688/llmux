package logs

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// Stage 16 契约（回显侧）：chat_logs 快照脱敏后，/api/logs/:id 响应体的
// request_headers 键恒在、敏感头键保留值为 ****、非敏感头原样、不含明文凭据。
// 断言打在响应体 JSON 上（AGENTS §5）；写侧剥除实现在 common.MaskSensitiveLogHeaders
// （接线契约测试在 service/chat），存量清洗在 repository 层，三段各锁一段。
// 种子用脱敏形态字面量而非 sanitizer 产物——避免断言自证。

func TestGetRequestLogDetail_RequestHeadersMaskedSnapshot(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:   "m-mask",
		Status: "success",
		Style:  "openai",
		RequestHeaders: `{"Authorization":["****"],"Cookie":["****"],"X-Api-Key":["****"],` +
			`"Proxy-Authorization":["****"],"User-Agent":["cli/1.0"]}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	id := strconv.FormatUint(uint64(log.ID), 10)
	c, w := testsupport.NewTestContext("GET", "/logs/"+id)
	c.Params = gin.Params{{Key: "id", Value: id}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	rh := gjson.Get(body, "data.request_headers")
	if !rh.Exists() {
		t.Fatalf("request_headers 键缺失（前端无法区分「未开启记录」与「后端没返字段」）；body=%s", body)
	}
	snapshot := rh.String()
	for _, key := range []string{`"Authorization":["****"]`, `"Cookie":["****"]`, `"X-Api-Key":["****"]`, `"Proxy-Authorization":["****"]`} {
		if !strings.Contains(snapshot, key) {
			t.Errorf("快照缺 %s（敏感头键保留、值掩码契约）；request_headers=%s", key, snapshot)
		}
	}
	if !strings.Contains(snapshot, `"User-Agent":["cli/1.0"]`) {
		t.Errorf("非敏感头被改动；request_headers=%s", snapshot)
	}
	for _, leak := range []string{"Bearer", "session=", "sk-"} {
		if strings.Contains(snapshot, leak) {
			t.Errorf("响应体快照明文泄露凭据痕迹 %q；request_headers=%s", leak, snapshot)
		}
	}
}
