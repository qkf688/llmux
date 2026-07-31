package virtualmodels

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

func TestBatchDeleteVirtualModelMapping_EmptyIDs(t *testing.T) {
	testsupport.InitTestDB(t)

	vm := models.VirtualModel{Name: "vm1"}
	if err := models.DB.Create(&vm).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/virtual-models/1/mappings/batch", strings.NewReader(`{"ids":[]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: strconvUint(vm.ID)}}

	BatchDeleteVirtualModelMapping(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 400 {
		t.Fatalf("payload code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

func TestBatchDeleteVirtualModelMapping_DeletesOnlyWithinVirtualModel(t *testing.T) {
	testsupport.InitTestDB(t)

	realModels := []models.Model{
		{Name: "m1"},
		{Name: "m2"},
		{Name: "m3"},
	}
	for i := range realModels {
		if err := models.DB.Create(&realModels[i]).Error; err != nil {
			t.Fatalf("create real model %d: %v", i, err)
		}
	}

	vm1 := models.VirtualModel{Name: "vm1"}
	if err := models.DB.Create(&vm1).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}
	vm2 := models.VirtualModel{Name: "vm2"}
	if err := models.DB.Create(&vm2).Error; err != nil {
		t.Fatalf("create virtual model 2: %v", err)
	}

	enabled := true
	m1 := models.VirtualModelMapping{VirtualModelID: vm1.ID, RealModelID: realModels[0].ID, Enabled: &enabled}
	m2 := models.VirtualModelMapping{VirtualModelID: vm1.ID, RealModelID: realModels[1].ID, Enabled: &enabled}
	mOther := models.VirtualModelMapping{VirtualModelID: vm2.ID, RealModelID: realModels[2].ID, Enabled: &enabled}
	if err := models.DB.Create(&m1).Error; err != nil {
		t.Fatalf("create mapping 1: %v", err)
	}
	if err := models.DB.Create(&m2).Error; err != nil {
		t.Fatalf("create mapping 2: %v", err)
	}
	if err := models.DB.Create(&mOther).Error; err != nil {
		t.Fatalf("create other mapping: %v", err)
	}

	reqBody := `{"ids":[` + strconvUint(m1.ID) + `,` + strconvUint(mOther.ID) + `]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/virtual-models/1/mappings/batch", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: strconvUint(vm1.ID)}}

	BatchDeleteVirtualModelMapping(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[struct {
		Deleted int64 `json:"deleted"`
	}]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if payload.Data.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", payload.Data.Deleted)
	}

	var remainingVM1 int64
	if err := models.DB.Model(&models.VirtualModelMapping{}).
		Where("virtual_model_id = ?", vm1.ID).
		Count(&remainingVM1).Error; err != nil {
		t.Fatalf("count remaining vm1 mappings: %v", err)
	}
	if remainingVM1 != 1 {
		t.Fatalf("remaining vm1 = %d, want 1", remainingVM1)
	}

	var remainingVM2 int64
	if err := models.DB.Model(&models.VirtualModelMapping{}).
		Where("virtual_model_id = ?", vm2.ID).
		Count(&remainingVM2).Error; err != nil {
		t.Fatalf("count remaining vm2 mappings: %v", err)
	}
	if remainingVM2 != 1 {
		t.Fatalf("remaining vm2 = %d, want 1", remainingVM2)
	}
}
