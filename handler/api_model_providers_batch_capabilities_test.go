package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atopos31/llmio/handler/associations"
	"github.com/atopos31/llmio/handler/testsupport"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

func TestBatchUpdateModelProvidersCapabilities_NoIDs_ReturnsEnvelope400(t *testing.T) {
	testsupport.InitTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PATCH", "/model-providers/batch/capabilities", strings.NewReader(`{"ids":[],"tool_call":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	associations.BatchUpdateModelProvidersCapabilities(c)

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
	if payload.Message != "No IDs provided" {
		t.Fatalf("payload message = %q, want %q, body=%s", payload.Message, "No IDs provided", w.Body.String())
	}
}

func TestBatchUpdateModelProvidersCapabilities_NoFields_ReturnsEnvelope400(t *testing.T) {
	testsupport.InitTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PATCH", "/model-providers/batch/capabilities", strings.NewReader(`{"ids":[1]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	associations.BatchUpdateModelProvidersCapabilities(c)

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
	if payload.Message != "No capability fields provided" {
		t.Fatalf("payload message = %q, want %q, body=%s", payload.Message, "No capability fields provided", w.Body.String())
	}
}

func TestBatchUpdateModelProvidersCapabilities_UpdatesOnlyProvidedFields(t *testing.T) {
	testsupport.InitTestDB(t)

	toolCallTrue := true
	structuredTrue := true
	imageTrue := true

	mp1 := models.ModelWithProvider{
		ModelID:          1,
		ProviderID:       1,
		ProviderModel:    "pm-1",
		ToolCall:         &toolCallTrue,
		StructuredOutput: &structuredTrue,
		Image:            &imageTrue,
		Weight:           1,
		Priority:         100,
	}
	if err := models.DB.Create(&mp1).Error; err != nil {
		t.Fatalf("create mp1: %v", err)
	}

	mp2 := models.ModelWithProvider{
		ModelID:          1,
		ProviderID:       2,
		ProviderModel:    "pm-2",
		ToolCall:         &toolCallTrue,
		StructuredOutput: &structuredTrue,
		Image:            &imageTrue,
		Weight:           1,
		Priority:         100,
	}
	if err := models.DB.Create(&mp2).Error; err != nil {
		t.Fatalf("create mp2: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		"PATCH",
		"/model-providers/batch/capabilities",
		strings.NewReader(`{"ids":[`+strconvUint(mp1.ID)+`,`+strconvUint(mp2.ID)+`],"tool_call":false}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	associations.BatchUpdateModelProvidersCapabilities(c)

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

	var updated1 models.ModelWithProvider
	if err := models.DB.Where("id = ?", mp1.ID).First(&updated1).Error; err != nil {
		t.Fatalf("find updated mp1: %v", err)
	}
	if updated1.ToolCall == nil || *updated1.ToolCall != false {
		t.Fatalf("updated mp1 ToolCall = %v, want false", updated1.ToolCall)
	}
	if updated1.StructuredOutput == nil || *updated1.StructuredOutput != true {
		t.Fatalf("updated mp1 StructuredOutput = %v, want true", updated1.StructuredOutput)
	}
	if updated1.Image == nil || *updated1.Image != true {
		t.Fatalf("updated mp1 Image = %v, want true", updated1.Image)
	}

	var updated2 models.ModelWithProvider
	if err := models.DB.Where("id = ?", mp2.ID).First(&updated2).Error; err != nil {
		t.Fatalf("find updated mp2: %v", err)
	}
	if updated2.ToolCall == nil || *updated2.ToolCall != false {
		t.Fatalf("updated mp2 ToolCall = %v, want false", updated2.ToolCall)
	}
	if updated2.StructuredOutput == nil || *updated2.StructuredOutput != true {
		t.Fatalf("updated mp2 StructuredOutput = %v, want true", updated2.StructuredOutput)
	}
	if updated2.Image == nil || *updated2.Image != true {
		t.Fatalf("updated mp2 Image = %v, want true", updated2.Image)
	}
}
