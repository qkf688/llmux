package pools

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

func batchUpdateViaHandler(t *testing.T, poolID uint, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}}
	c.Request = httptest.NewRequest("PATCH", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/batch/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	BatchUpdateCredentialStatus(c)
	return w.Body.String(), w.Code
}

func batchDeleteViaHandler(t *testing.T, poolID uint, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}}
	c.Request = httptest.NewRequest("DELETE", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/batch", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	BatchDeleteCredentials(c)
	return w.Body.String(), w.Code
}

func createN(t *testing.T, poolID uint, n int, prefix string) []uint {
	t.Helper()
	ids := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		body, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-`+prefix+`-`+strconv.Itoa(i)+`-ABCD1234"}`)
		if gjson.Get(body, "code").Int() != 200 {
			t.Fatalf("createN失败 i=%d body=%s", i, body)
		}
		ids = append(ids, uint(gjson.Get(body, "data.ID").Uint()))
	}
	return ids
}

// TestBatchUpdateCredentialStatus 覆盖：批量启停仅计池内命中、状态非法400、空ids400、池不存在404
func TestBatchUpdateCredentialStatus(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolA := createPoolAndGetID(t, "批量启停池A")
	poolB := createPoolAndGetID(t, "批量启停池B")
	idsA := createN(t, poolA, 3, "batch")
	idsB := createN(t, poolB, 1, "other")

	// 批量停用池A的2条 + 1条越池ID（不应计）
	payload := `{"ids":[` + strconv.FormatUint(uint64(idsA[0]), 10) + `,` + strconv.FormatUint(uint64(idsA[1]), 10) + `,` + strconv.FormatUint(uint64(idsB[0]), 10) + `],"status":"disabled"}`
	body, _ := batchUpdateViaHandler(t, poolA, payload)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("批量启停应200 body=%s", body)
	}
	if got := gjson.Get(body, "data.updated").Int(); got != 2 {
		t.Fatalf("updated=%d want 2 body=%s", got, body)
	}
	// 校验池A内状态
	for _, id := range idsA[:2] {
		got, _ := getCredentialViaHandler(t, poolA, id)
		if gjson.Get(got, "data.Status").String() != models.CredentialStatusDisabled {
			t.Fatalf("id=%d Status=%q want disabled", id, gjson.Get(got, "data.Status").String())
		}
	}
	// 越池的那条在池B仍为active
	gotB, _ := getCredentialViaHandler(t, poolB, idsB[0])
	if gjson.Get(gotB, "data.Status").String() != models.CredentialStatusActive {
		t.Fatalf("越池不该被改 body=%s", gotB)
	}
	// 空ids 400
	body2, _ := batchUpdateViaHandler(t, poolA, `{"ids":[],"status":"active"}`)
	if gjson.Get(body2, "code").Int() != 400 {
		t.Fatalf("空ids应400 body=%s", body2)
	}
	// 非法 status 400
	body3, _ := batchUpdateViaHandler(t, poolA, `{"ids":[`+strconv.FormatUint(uint64(idsA[0]), 10)+`],"status":"invalid"}`)
	if gjson.Get(body3, "code").Int() != 400 {
		t.Fatalf("非法status应400 body=%s", body3)
	}
	// 不存在池 404
	body4, _ := batchUpdateViaHandler(t, 9999, `{"ids":[1],"status":"active"}`)
	if gjson.Get(body4, "code").Int() != 404 {
		t.Fatalf("不存在池应404 body=%s", body4)
	}
}

// TestBatchDeleteCredentials 覆盖：批量删除仅删池内、空IDs400、越池不计
func TestBatchDeleteCredentials(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolA := createPoolAndGetID(t, "批量删除池A")
	poolB := createPoolAndGetID(t, "批量删除池B")
	idsA := createN(t, poolA, 3, "bdel")
	idsB := createN(t, poolB, 1, "bother")

	payload := `{"ids":[` + strconv.FormatUint(uint64(idsA[0]), 10) + `,` + strconv.FormatUint(uint64(idsA[1]), 10) + `,` + strconv.FormatUint(uint64(idsB[0]), 10) + `]}`
	body, _ := batchDeleteViaHandler(t, poolA, payload)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("批量删除应200 body=%s", body)
	}
	if got := gjson.Get(body, "data.deleted").Int(); got != 2 {
		t.Fatalf("deleted=%d want 2 body=%s", got, body)
	}
	// 池A剩余1条
	bodyList, _ := listCredentialsViaHandler(t, poolA, "")
	remaining := gjson.Get(bodyList, "data.total").Int()
	if remaining != 1 {
		t.Fatalf("批量删后total=%d want 1", remaining)
	}
	// 池B那条还在
	gotB, _ := getCredentialViaHandler(t, poolB, idsB[0])
	if gjson.Get(gotB, "code").Int() != 200 {
		t.Fatalf("池B越池误删 body=%s", gotB)
	}
	// 空IDs 400
	body2, _ := batchDeleteViaHandler(t, poolA, `{"ids":[]}`)
	if gjson.Get(body2, "code").Int() != 400 {
		t.Fatalf("空ids应400 body=%s", body2)
	}
	// 不存在池 404
	body3, _ := batchDeleteViaHandler(t, 9999, `{"ids":[`+strconv.FormatUint(uint64(idsA[2]), 10)+`]}`)
	if gjson.Get(body3, "code").Int() != 404 {
		t.Fatalf("不存在池应404 body=%s", body3)
	}
}
