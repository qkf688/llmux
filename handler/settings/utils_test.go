package settings

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/settings"
	"github.com/gin-gonic/gin"
)

type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type resetWeightsResponse struct {
	Updated       int64 `json:"updated"`
	DefaultWeight int   `json:"default_weight"`
}

type resetPrioritiesResponse struct {
	Updated         int64 `json:"updated"`
	DefaultPriority int   `json:"default_priority"`
}

type enableAssociationsResponse struct {
	Updated int64 `json:"updated"`
}

func initTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	// 与 models.DB 同步默认 Store，避免 settings.Default() 缓存上一个用例的连接
	settings.SetDefault(settings.NewStore(models.DB))
	t.Cleanup(func() {
		settings.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func newTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

func TestResetModelWeights(t *testing.T) {
	initTestDB(t)

	provider := models.Provider{Name: "p1", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	falseVal := false
	active := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm1",
		Status:        &falseVal,
		Weight:        1,
		Priority:      1,
	}
	deleted := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm2",
		Status:        &falseVal,
		Weight:        2,
		Priority:      2,
	}
	if err := models.DB.Create(&active).Error; err != nil {
		t.Fatalf("create active association: %v", err)
	}
	if err := models.DB.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted association: %v", err)
	}
	if err := models.DB.Delete(&deleted).Error; err != nil {
		t.Fatalf("soft delete association: %v", err)
	}

	c, w := newTestContext("POST", "/settings/reset-weights")
	ResetModelWeights(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[resetWeightsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if payload.Data.DefaultWeight != 5 {
		t.Fatalf("default_weight = %d, want 5", payload.Data.DefaultWeight)
	}
	if payload.Data.Updated != 1 {
		t.Fatalf("updated = %d, want 1", payload.Data.Updated)
	}

	var gotActive models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotActive, active.ID).Error; err != nil {
		t.Fatalf("load active association: %v", err)
	}
	if gotActive.Weight != 5 {
		t.Fatalf("active weight = %d, want 5", gotActive.Weight)
	}

	var gotDeleted models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotDeleted, deleted.ID).Error; err != nil {
		t.Fatalf("load deleted association: %v", err)
	}
	if gotDeleted.Weight != 2 {
		t.Fatalf("deleted weight = %d, want 2", gotDeleted.Weight)
	}
}

func TestResetModelPriorities(t *testing.T) {
	initTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		Update("value", "42").Error; err != nil {
		t.Fatalf("set auto priority default: %v", err)
	}

	provider := models.Provider{Name: "p1", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	falseVal := false
	active := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm1",
		Status:        &falseVal,
		Weight:        1,
		Priority:      1,
	}
	deleted := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm2",
		Status:        &falseVal,
		Weight:        2,
		Priority:      2,
	}
	if err := models.DB.Create(&active).Error; err != nil {
		t.Fatalf("create active association: %v", err)
	}
	if err := models.DB.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted association: %v", err)
	}
	if err := models.DB.Delete(&deleted).Error; err != nil {
		t.Fatalf("soft delete association: %v", err)
	}

	c, w := newTestContext("POST", "/settings/reset-priorities")
	ResetModelPriorities(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[resetPrioritiesResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if payload.Data.DefaultPriority != 42 {
		t.Fatalf("default_priority = %d, want 42", payload.Data.DefaultPriority)
	}
	if payload.Data.Updated != 1 {
		t.Fatalf("updated = %d, want 1", payload.Data.Updated)
	}

	var gotActive models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotActive, active.ID).Error; err != nil {
		t.Fatalf("load active association: %v", err)
	}
	if gotActive.Priority != 42 {
		t.Fatalf("active priority = %d, want 42", gotActive.Priority)
	}

	var gotDeleted models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotDeleted, deleted.ID).Error; err != nil {
		t.Fatalf("load deleted association: %v", err)
	}
	if gotDeleted.Priority != 2 {
		t.Fatalf("deleted priority = %d, want 2", gotDeleted.Priority)
	}
}

func TestEnableAllAssociations(t *testing.T) {
	initTestDB(t)

	provider := models.Provider{Name: "p1", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	falseVal := false
	active := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm1",
		Status:        &falseVal,
		Weight:        1,
		Priority:      1,
	}
	deleted := models.ModelWithProvider{
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		ProviderModel: "pm2",
		Status:        &falseVal,
		Weight:        2,
		Priority:      2,
	}
	if err := models.DB.Create(&active).Error; err != nil {
		t.Fatalf("create active association: %v", err)
	}
	if err := models.DB.Create(&deleted).Error; err != nil {
		t.Fatalf("create deleted association: %v", err)
	}
	if err := models.DB.Delete(&deleted).Error; err != nil {
		t.Fatalf("soft delete association: %v", err)
	}

	c, w := newTestContext("POST", "/settings/enable-all-associations")
	EnableAllAssociations(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload apiEnvelope[enableAssociationsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if payload.Data.Updated != 1 {
		t.Fatalf("updated = %d, want 1", payload.Data.Updated)
	}

	var gotActive models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotActive, active.ID).Error; err != nil {
		t.Fatalf("load active association: %v", err)
	}
	if gotActive.Status == nil || !*gotActive.Status {
		t.Fatalf("active status = %#v, want true", gotActive.Status)
	}

	var gotDeleted models.ModelWithProvider
	if err := models.DB.Unscoped().First(&gotDeleted, deleted.ID).Error; err != nil {
		t.Fatalf("load deleted association: %v", err)
	}
	if gotDeleted.Status == nil || *gotDeleted.Status != false {
		t.Fatalf("deleted status = %#v, want false", gotDeleted.Status)
	}
}

func TestCleanupExcessLogs_HardDeletesSoftDeletedBeyondRetention(t *testing.T) {
	initTestDB(t)

	logs := make([]models.ChatLog, 200)
	for i := range logs {
		logs[i] = models.ChatLog{
			Name:          "m1",
			ProviderModel: "pm1",
			ProviderName:  "p1",
			Status:        "success",
		}
		if err := models.DB.Create(&logs[i]).Error; err != nil {
			t.Fatalf("create log %d: %v", i, err)
		}
		if err := models.DB.Create(&models.ChatIO{
			LogId: logs[i].ID,
			Input: "in",
			OutputUnion: models.OutputUnion{
				OfString: "out",
			},
		}).Error; err != nil {
			t.Fatalf("create chat io %d: %v", i, err)
		}
	}

	idsToSoftDelete := make([]uint, 100)
	for i := 0; i < 100; i++ {
		idsToSoftDelete[i] = logs[i].ID
	}
	if err := models.DB.Where("id IN ?", idsToSoftDelete).Delete(&models.ChatLog{}).Error; err != nil {
		t.Fatalf("soft delete logs: %v", err)
	}

	var visible int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&visible).Error; err != nil {
		t.Fatalf("count visible logs: %v", err)
	}
	if visible != 100 {
		t.Fatalf("visible logs = %d, want 100", visible)
	}

	var total int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		t.Fatalf("count total logs: %v", err)
	}
	if total != 200 {
		t.Fatalf("total logs = %d, want 200", total)
	}

	cleanupExcessLogs(100)

	var afterTotal int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Count(&afterTotal).Error; err != nil {
		t.Fatalf("count total logs after cleanup: %v", err)
	}
	if afterTotal != 100 {
		t.Fatalf("total logs after cleanup = %d, want 100", afterTotal)
	}

	var softDeleted int64
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Where("deleted_at IS NOT NULL").Count(&softDeleted).Error; err != nil {
		t.Fatalf("count soft deleted logs: %v", err)
	}
	if softDeleted != 0 {
		t.Fatalf("soft deleted logs = %d, want 0", softDeleted)
	}

	var ioTotal int64
	if err := models.DB.Unscoped().Model(&models.ChatIO{}).Count(&ioTotal).Error; err != nil {
		t.Fatalf("count chat io: %v", err)
	}
	if ioTotal != 100 {
		t.Fatalf("chat io total = %d, want 100", ioTotal)
	}
}
