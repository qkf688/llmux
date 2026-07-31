package associations

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func createDisabledAssoc(t *testing.T, failures int) models.ModelWithProvider {
	t.Helper()
	disabled := false
	mp := models.ModelWithProvider{
		ModelID:             1,
		ProviderID:          1,
		ProviderModel:       "pm",
		Weight:              10,
		Priority:            10,
		Status:              &disabled,
		ConsecutiveFailures: failures,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}
	return mp
}

func assertCleared(t *testing.T, id uint) {
	t.Helper()
	got, err := repository.Default().ModelWithProvider.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Fatalf("status = %v, want enabled", got.Status)
	}
	if got.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive_failures = %d, want 0", got.ConsecutiveFailures)
	}
}

// TestUpdateModelProviderStatus_EnableClearsConsecutiveFailures 回归：手动启用需把连续失败计数清零。
// 曾用结构体 Updates 传 ConsecutiveFailures: 0，被 GORM 当零值跳过。
func TestUpdateModelProviderStatus_EnableClearsConsecutiveFailures(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createDisabledAssoc(t, 5)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mp.ID), 10)}}
	c.Request = httptest.NewRequest("PATCH", "/model-providers/"+strconv.FormatUint(uint64(mp.ID), 10)+"/status", strings.NewReader(`{"status":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModelProviderStatus(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	assertCleared(t, mp.ID)
}

// TestBatchUpdateModelProvidersStatus_EnableClearsConsecutiveFailures 批量启用同样需要清零。
func TestBatchUpdateModelProvidersStatus_EnableClearsConsecutiveFailures(t *testing.T) {
	testsupport.InitTestDB(t)
	mp1 := createDisabledAssoc(t, 4)
	mp2 := createDisabledAssoc(t, 7)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"ids":[` + strconv.FormatUint(uint64(mp1.ID), 10) + `,` + strconv.FormatUint(uint64(mp2.ID), 10) + `],"status":true}`
	c.Request = httptest.NewRequest("PATCH", "/model-providers/batch/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	BatchUpdateModelProvidersStatus(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	assertCleared(t, mp1.ID)
	assertCleared(t, mp2.ID)
}

// TestBatchUpdateModelProvidersStatus_DisableKeepsFailureCount 禁用路径不应改动计数。
func TestBatchUpdateModelProvidersStatus_DisableKeepsFailureCount(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createDisabledAssoc(t, 4)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PATCH", "/model-providers/batch/status", strings.NewReader(`{"ids":[`+strconv.FormatUint(uint64(mp.ID), 10)+`],"status":false}`))
	c.Request.Header.Set("Content-Type", "application/json")

	BatchUpdateModelProvidersStatus(c)

	got, err := repository.Default().ModelWithProvider.Get(t.Context(), mp.ID)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || *got.Status {
		t.Fatalf("status = %v, want disabled", got.Status)
	}
	if got.ConsecutiveFailures != 4 {
		t.Fatalf("consecutive_failures = %d, want 4 (unchanged)", got.ConsecutiveFailures)
	}
}
