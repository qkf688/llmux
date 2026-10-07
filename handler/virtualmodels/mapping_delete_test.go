package virtualmodels

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

func TestDeleteVirtualModelMapping_HardDeleteAllowsRecreate(t *testing.T) {
	testsupport.InitTestDB(t)

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

		var payload testsupport.APIEnvelope[models.VirtualModelMapping]
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

		var payload testsupport.APIEnvelope[any]
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
		var payload testsupport.APIEnvelope[models.VirtualModelMapping]
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal recreate response: %v, body=%s", err, w.Body.String())
		}
		if payload.Code != 200 {
			t.Fatalf("recreate payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
		}
	}
}

func strconvUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
