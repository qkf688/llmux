package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

func TestDeleteVirtualModelMapping_HardDeleteAllowsRecreate(t *testing.T) {
	initHandlerTestDB(t)

	realModel := models.Model{Name: "mimo-v2-flash"}
	if err := models.DB.Create(&realModel).Error; err != nil {
		t.Fatalf("create real model: %v", err)
	}

	virtualModel := models.VirtualModel{Name: "vm1"}
	if err := models.DB.Create(&virtualModel).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	gin.SetMode(gin.TestMode)

	createReqBody := `{"real_model_id":` + strconvUint(realModel.ID) + `,"priority":10,"weight":5,"enabled":true}`

	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/virtual-models/1/mappings", strings.NewReader(createReqBody))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: strconvUint(virtualModel.ID)}}

		CreateVirtualModelMapping(c)
		if w.Code != 200 {
			t.Fatalf("create mapping status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload apiEnvelope[models.VirtualModelMapping]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal create response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("create payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
	}

	var mapping models.VirtualModelMapping
	if err := models.DB.Where("virtual_model_id = ? AND real_model_id = ?", virtualModel.ID, realModel.ID).First(&mapping).Error; err != nil {
		t.Fatalf("find mapping: %v", err)
	}

	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/virtual-models/1/mappings/1", nil)
		c.Params = []gin.Param{
			{Key: "id", Value: strconvUint(virtualModel.ID)},
			{Key: "mapping_id", Value: strconvUint(mapping.ID)},
		}

		DeleteVirtualModelMapping(c)
		if w.Code != 200 {
			t.Fatalf("delete mapping status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}

		var payload apiEnvelope[any]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal delete response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("delete payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
	}

	{
		var count int64
		if err := models.DB.Unscoped().
			Model(&models.VirtualModelMapping{}).
			Where("virtual_model_id = ? AND real_model_id = ?", virtualModel.ID, realModel.ID).
			Count(&count).Error; err != nil {
			t.Fatalf("count mappings unscoped: %v", err)
		}
		if count != 0 {
			t.Fatalf("unscoped mappings count = %d, want 0", count)
		}
	}

	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/virtual-models/1/mappings", strings.NewReader(createReqBody))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = []gin.Param{{Key: "id", Value: strconvUint(virtualModel.ID)}}

		CreateVirtualModelMapping(c)
		if w.Code != 200 {
			t.Fatalf("recreate mapping status code = %d, want 200, body=%s", w.Code, w.Body.String())
		}
		var payload apiEnvelope[models.VirtualModelMapping]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal recreate response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("recreate payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
	}
}

func TestModelsInit_CleansSoftDeletedVirtualModelMappings(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "llmio-test.db")

	models.Init(context.Background(), dbPath)
	{
		sqlDB, err := models.DB.DB()
		if err != nil {
			t.Fatalf("get sql db: %v", err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
	}

	realModel := models.Model{Name: "mimo-v2-flash"}
	if err := models.DB.Create(&realModel).Error; err != nil {
		t.Fatalf("create real model: %v", err)
	}
	virtualModel := models.VirtualModel{Name: "vm1"}
	if err := models.DB.Create(&virtualModel).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}
	mapping := models.VirtualModelMapping{VirtualModelID: virtualModel.ID, RealModelID: realModel.ID}
	if err := models.DB.Create(&mapping).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}
	if err := models.DB.Delete(&mapping).Error; err != nil {
		t.Fatalf("soft delete mapping: %v", err)
	}

	// Close and re-init to simulate server restart with legacy soft-deleted rows present.
	{
		sqlDB, err := models.DB.DB()
		if err != nil {
			t.Fatalf("get sql db: %v", err)
		}
		_ = sqlDB.Close()
	}

	models.Init(context.Background(), dbPath)
	{
		sqlDB, err := models.DB.DB()
		if err != nil {
			t.Fatalf("get sql db: %v", err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
	}

	var deletedCount int64
	if err := models.DB.Unscoped().
		Model(&models.VirtualModelMapping{}).
		Where("deleted_at IS NOT NULL").
		Count(&deletedCount).Error; err != nil {
		t.Fatalf("count soft-deleted mappings: %v", err)
	}
	if deletedCount != 0 {
		t.Fatalf("soft-deleted mappings count = %d, want 0", deletedCount)
	}

	// Recreate should succeed after cleanup.
	if err := models.DB.Create(&models.VirtualModelMapping{VirtualModelID: virtualModel.ID, RealModelID: realModel.ID}).Error; err != nil {
		t.Fatalf("recreate mapping after cleanup: %v", err)
	}
}

func strconvUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
