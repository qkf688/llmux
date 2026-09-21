package logs

import (
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// Stage 17 契约（回显侧）：rid 来源标记落库后，/api/logs 与 /api/logs/:id 响应体的
// request_id 键**恒在**——值 = 建行时定格的带前缀编号（自生成 gw: / 沿用 in:），
// 存量行/无归因路径为空串（键在值空，前端可区分「无编号」与「后端漏返」）。
// 断言打在响应体 JSON 上（AGENTS §5）；写侧填充契约在 service/chat 建行点测试。

func TestGetRequestLogs_RequestIDSnapshot(t *testing.T) {
	testsupport.InitTestDB(t)

	withRID := models.ChatLog{Name: "m-rid", Status: "success", Style: "openai", RequestID: "in:trace-0921"}
	legacy := models.ChatLog{Name: "m-legacy", Status: "error", Style: "openai"} // 存量行：建行时无 rid 概念
	for _, l := range []*models.ChatLog{&withRID, &legacy} {
		if err := models.DB.Create(l).Error; err != nil {
			t.Fatalf("create log %s: %v", l.Name, err)
		}
	}

	// 列表路径：两行的 request_id 键都必须在（信封 data 内 handler 自包一层
	// {data,total,page,…} 分页结构，数组在 data.data）。
	c, w := testsupport.NewTestContext("GET", "/logs")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("list status = %d, body=%s", w.Code, w.Body.String())
	}
	const missing = "\u0000missing"
	found := map[string]string{}
	gjson.Get(w.Body.String(), "data.data").ForEach(func(_, item gjson.Result) bool {
		name := item.Get("name").String()
		if name != "m-rid" && name != "m-legacy" {
			return true
		}
		rid := item.Get("request_id")
		if !rid.Exists() {
			t.Errorf("列表行 %s 的 request_id 键缺失；body=%s", name, w.Body.String())
			found[name] = missing
			return true
		}
		found[name] = rid.String()
		return true
	})
	if got := found["m-rid"]; got != "in:trace-0921" {
		t.Errorf("列表行 m-rid 的 request_id = %q, want in:trace-0921", got)
	}
	if got := found["m-legacy"]; got != "" {
		t.Errorf("列表行 m-legacy 的 request_id = %q, want 键在值空", got)
	}

	// 详情路径
	for _, tc := range []struct {
		log  models.ChatLog
		want string
	}{{withRID, "in:trace-0921"}, {legacy, ""}} {
		id := strconv.FormatUint(uint64(tc.log.ID), 10)
		cd, wd := testsupport.NewTestContext("GET", "/logs/"+id)
		cd.Params = gin.Params{{Key: "id", Value: id}}
		GetRequestLogDetail(cd)
		if wd.Code != 200 {
			t.Fatalf("detail status = %d, body=%s", wd.Code, wd.Body.String())
		}
		res := gjson.Get(wd.Body.String(), "data.request_id")
		if !res.Exists() {
			t.Errorf("详情行 %s 的 request_id 键缺失；body=%s", tc.log.Name, wd.Body.String())
			continue
		}
		if res.String() != tc.want {
			t.Errorf("详情行 %s 的 request_id = %q, want %q", tc.log.Name, res.String(), tc.want)
		}
	}
}
