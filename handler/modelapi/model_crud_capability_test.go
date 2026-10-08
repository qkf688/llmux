package modelapi

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

// 模型层能力真值的写入契约：Update 必须能开，也能关，且缺省字段不得被静默重置。
//
// 缺省保护是这组用例的重点：模型能力是所有「继承」关联的共同上游，一旦某个不带该字段的
// PUT 把已开启的模型重置成 false，所有继承它的关联会同时失去该能力——故障面从一行放大到整个模型。

func updateModelForCaps(t *testing.T, id uint, body string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rawID := strconv.FormatUint(uint64(id), 10)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: rawID}}
	c.Request = httptest.NewRequest("PUT", "/models/"+rawID, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModel(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

// TestUpdateModel_CapabilitiesRoundTrip 覆盖「开 → 关 → 缺省保留」三步。
func TestUpdateModel_CapabilitiesRoundTrip(t *testing.T) {
	testsupport.InitTestDB(t)
	m := createModelForUpdateTest(t)

	// 1) 显式开启三项
	updateModelForCaps(t, m.ID, `{"name":"test-model-update","io_log":false,`+
		`"tool_call":true,"structured_output":true,"image":true}`)
	got := reloadModel(t, m.ID)
	if !got.ToolCall || !got.StructuredOutput || !got.Image {
		t.Fatalf("case 1: caps = %v/%v/%v, want all true", got.ToolCall, got.StructuredOutput, got.Image)
	}

	// 2) 显式关闭其中一项：逐字段独立生效，不得连坐
	updateModelForCaps(t, m.ID, `{"name":"test-model-update","io_log":false,"image":false}`)
	got = reloadModel(t, m.ID)
	if got.Image {
		t.Errorf("case 2: Image = true, want false")
	}
	if !got.ToolCall || !got.StructuredOutput {
		t.Errorf("case 2: ToolCall/StructuredOutput = %v/%v, want true/true（未传的字段应保留）",
			got.ToolCall, got.StructuredOutput)
	}

	// 3) 完全不带能力字段：必须保留存量，不得被重置
	updateModelForCaps(t, m.ID, `{"name":"test-model-update","io_log":false}`)
	got = reloadModel(t, m.ID)
	if !got.ToolCall || !got.StructuredOutput || got.Image {
		t.Errorf("case 3: caps = %v/%v/%v, want true/true/false（缺省不改）",
			got.ToolCall, got.StructuredOutput, got.Image)
	}
}

// TestCreateModel_CapabilitiesDefaultFalse 建模型不带能力字段时落 false（无「未设置」语义）。
func TestCreateModel_CapabilitiesDefaultFalse(t *testing.T) {
	testsupport.InitTestDB(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/models", strings.NewReader(`{"name":"m-new","io_log":false}`))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateModel(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	created := reloadModel(t, 1)
	if created.ToolCall || created.StructuredOutput || created.Image {
		t.Errorf("新建模型能力 = %v/%v/%v, want 全 false",
			created.ToolCall, created.StructuredOutput, created.Image)
	}
}

// TestCreateModel_AutoAssociate_DefaultsTrueExplicitFalseSticks 锁「自动关联」二态开关的建行契约。
//
// 两条方向都要断：缺省 = true（前端表单默认值，也是原 gorm default:true 的口径），
// 显式 false 必须真的存住 false。这条是本字段收回二态的**行为分界**——改前字段带
// `gorm:"default:true"`，Create 传显式 false 会被 default tag 当零值吞掉、落库成 true，
// 于是「新建时不勾自动关联」在界面上根本做不到（勾不勾都一样）。
func TestCreateModel_AutoAssociate_DefaultsTrueExplicitFalseSticks(t *testing.T) {
	testsupport.InitTestDB(t)
	gin.SetMode(gin.TestMode)

	createViaHandler := func(name, body string) *models.Model {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/models", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		CreateModel(c)
		if w.Code != 200 {
			t.Fatalf("[%s] status code = %d, want 200, body=%s", name, w.Code, w.Body.String())
		}
		got, err := repository.Default().Model.GetByName(t.Context(), name)
		if err != nil {
			t.Fatalf("[%s] reload model: %v", name, err)
		}
		return got
	}

	// 缺省：DTO 不带 auto_associate → 落 true（与前端表单默认值一致）
	if got := createViaHandler("m-default", `{"name":"m-default","io_log":false}`); !got.AutoAssociate {
		t.Errorf("缺省建行 AutoAssociate = false, want true（缺省=允许自动关联）")
	}

	// 显式 false：必须存住 false，不得被默认值吞回 true
	if got := createViaHandler("m-off", `{"name":"m-off","io_log":false,"auto_associate":false}`); got.AutoAssociate {
		t.Errorf("显式 false 建行 AutoAssociate = true, want false（显式关闭必须存得住）")
	}

	// 显式 true：同上，闭合两端
	if got := createViaHandler("m-on", `{"name":"m-on","io_log":false,"auto_associate":true}`); !got.AutoAssociate {
		t.Errorf("显式 true 建行 AutoAssociate = false, want true")
	}
}
