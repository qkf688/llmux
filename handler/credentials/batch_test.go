package credentials

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// TestBatchUpdateCredentialStatus_AcrossPools 批量启停按 ID 生效，不分归属：
// 不同号池的凭据混在一批里一次恢复——这是「凭据是第一等资源」的直接后果，
// 也是被判停后最省事的人工恢复路径。
func TestBatchUpdateCredentialStatus_AcrossPools(t *testing.T) {
	initCredentialsTestDB(t)
	_, poolCred := seedPoolCredential(t, "批量池", "sk-batch-pool-ABCD1234", models.CredentialStatusTempUnsched)
	poolB := seedPool(t, "批量池B")
	otherCred := seedCredential(t, poolB, "sk-batch-other-ABCD1234",
		models.CredentialStatusTempUnsched, 3, "auth_fail")
	// 池凭据也补上连败痕迹（恢复须一并清掉）
	if _, err := repository.Default().Credential.UpdateFields(t.Context(), poolCred,
		map[string]any{"fail_count": 2, "cooldown_reason": "auth_fail"}); err != nil {
		t.Fatal(err)
	}

	payload := `{"ids":[` + itoa(poolCred) + `,` + itoa(otherCred) + `],"status":"active"}`
	body, _ := invoke(t, "PATCH", "/credentials/status", nil, payload, BatchUpdateCredentialStatus)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("批量恢复应 200 body=%s", body)
	}
	if got := gjson.Get(body, "data.updated").Int(); got != 2 {
		t.Fatalf("updated=%d want 2 body=%s", got, body)
	}
	for _, id := range []uint{poolCred, otherCred} {
		stored := reloadCredential(t, id)
		if stored.Status != models.CredentialStatusActive || stored.FailCount != 0 || stored.CooldownReason != "" {
			t.Fatalf("凭据 %d 恢复不完整: %+v", id, stored)
		}
	}
}

// TestBatchUpdateCredentialStatus_Validation 空 ids 400、非法状态 400。
func TestBatchUpdateCredentialStatus_Validation(t *testing.T) {
	initCredentialsTestDB(t)
	_, credID := seedPoolCredential(t, "批量校验池", "sk-batch-v-ABCD1234", models.CredentialStatusActive)

	if body, _ := invoke(t, "PATCH", "/credentials/status", nil, `{"ids":[],"status":"active"}`, BatchUpdateCredentialStatus); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("空 ids 应 400 body=%s", body)
	}
	if body, _ := invoke(t, "PATCH", "/credentials/status", nil, `{"ids":[`+itoa(credID)+`],"status":"bogus"}`, BatchUpdateCredentialStatus); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("非法状态应 400 body=%s", body)
	}
}

// TestBatchUpdateCredentialStatus_RowsAffectedCountsHits 记录 SQLite 下「状态已相同」的
// 重复批量更新语义：命中行全部计入 RowsAffected（updated = 命中数，不是变化数）。
func TestBatchUpdateCredentialStatus_RowsAffectedCountsHits(t *testing.T) {
	initCredentialsTestDB(t)
	poolID := seedPool(t, "幂等池")
	ids := []uint{
		seedPoolMember(t, poolID, "sk-idem-0-ABCD1234", models.CredentialStatusDisabled),
		seedPoolMember(t, poolID, "sk-idem-1-ABCD1234", models.CredentialStatusActive),
		seedPoolMember(t, poolID, "sk-idem-2-ABCD1234", models.CredentialStatusActive),
	}

	body, _ := invoke(t, "PATCH", "/credentials/status", nil,
		`{"ids":[`+itoa(ids[0])+`,`+itoa(ids[1])+`,`+itoa(ids[2])+`],"status":"disabled"}`, BatchUpdateCredentialStatus)
	if got := gjson.Get(body, "data.updated").Int(); got != 3 {
		t.Fatalf("updated=%d want 3（命中计数）body=%s", got, body)
	}
}

// TestBatchDeleteCredentials_ByIDs 批量软删按 ID 生效，跨池一并删；空 ids 400。
func TestBatchDeleteCredentials_ByIDs(t *testing.T) {
	initCredentialsTestDB(t)
	poolID, poolCred := seedPoolCredential(t, "批量删除池", "sk-bdel-pool-ABCD1234", models.CredentialStatusActive)
	poolB := seedPool(t, "批量删除池B")
	otherCred := seedPoolMember(t, poolB, "sk-bdel-other-ABCD1234", models.CredentialStatusActive)
	survivor := seedPoolMember(t, poolID, "sk-bdel-survivor-ABCD1234", models.CredentialStatusActive)

	body, _ := invoke(t, "DELETE", "/credentials/batch", nil,
		`{"ids":[`+itoa(poolCred)+`,`+itoa(otherCred)+`]}`, BatchDeleteCredentials)
	if got := gjson.Get(body, "data.deleted").Int(); got != 2 {
		t.Fatalf("deleted=%d want 2 body=%s", got, body)
	}
	for _, id := range []uint{poolCred, otherCred} {
		if after, _ := invoke(t, "GET", "/credentials/"+itoa(id), credentialParams(id), "", GetCredential); gjson.Get(after, "code").Int() != 404 {
			t.Fatalf("凭据 %d 未被软删 body=%s", id, after)
		}
	}
	if after, _ := invoke(t, "GET", "/credentials/"+itoa(survivor), credentialParams(survivor), "", GetCredential); gjson.Get(after, "code").Int() != 200 {
		t.Fatalf("未列出的凭据不应被删 body=%s", after)
	}

	if body, _ := invoke(t, "DELETE", "/credentials/batch", nil, `{"ids":[]}`, BatchDeleteCredentials); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("空 ids 应 400 body=%s", body)
	}
}

// TestRegisterRouteOrder_StaticSegmentsBeforeID 锁注册顺序契约：/credentials/status 与
// /credentials/batch 固定段必须优先于 /credentials/:id，否则 "status"/"batch" 会被当成
// 凭据 ID 解析（表现为 400 Invalid ID format），批量端点被单条路径劫持。
//
// 判别式：批量端点先做入参校验，空 ids → 400 "No IDs provided"；被 :id 劫持则先解析
// 路径参数 → 400 "Invalid ID format"。
func TestRegisterRouteOrder_StaticSegmentsBeforeID(t *testing.T) {
	initCredentialsTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"status 固定段优先", "PATCH", "/credentials/status", `{"ids":[],"status":"active"}`},
		{"batch 固定段优先", "DELETE", "/credentials/batch", `{"ids":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := serveJSON(t, r, tc.method, tc.path, tc.body).Body.String()
			if strings.Contains(body, "Invalid ID format") {
				t.Fatalf("固定段被 :id 劫持 body=%s", body)
			}
			if !strings.Contains(body, "No IDs provided") {
				t.Fatalf("未命中批量端点 body=%s", body)
			}
		})
	}
}
