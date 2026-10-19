package associations

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// GetModelProviders 获取模型的提供商关联列表。
func GetModelProviders(c *gin.Context) {
	modelIDStr := c.Query("model_id")
	if modelIDStr == "" {
		httpresp.BadRequest(c, "model_id query parameter is required")
		return
	}

	modelID, err := strconv.ParseUint(modelIDStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid model_id format")
		return
	}

	modelProviders, err := repos().ModelWithProvider.ListByModelID(c.Request.Context(), uint(modelID))
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	httpresp.Success(c, modelProviders)
}

// CreateModelProvider 创建模型提供商关联。
func CreateModelProvider(c *gin.Context) {
	var req ModelWithProviderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()

	// 写入闸：上游模型名是关联的定义性字段，直接进出站 body 的 model，空必被上游 400
	// （失败记在供应商头上、极端情况自动停用该关联），故不给「不指定上游名」兜底。
	// 故意不用 binding:"required"：它只认空串，`"   "` 能过——而空格恰恰是 Update 侧
	// 唯一能真正写进库的形态（见 types.go 的三态说明）。
	if req.ProviderModel == nil || strings.TrimSpace(*req.ProviderModel) == "" {
		httpresp.BadRequest(c, "provider_model is required")
		return
	}

	// 校验 model 存在（手动关联始终允许，与现网一致）。
	if _, err := repos().Model.Get(ctx, req.ModelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.BadRequest(c, "Model not found")
			return
		}
		httpresp.InternalErrorMessage(c, "Failed to get model", err)
		return
	}
	// 校验 provider 存在，避免悬空关联（与 Update 对齐）。
	if req.ProviderID > 0 {
		if _, err := repos().Provider.Get(ctx, req.ProviderID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpresp.BadRequest(c, "Provider not found")
				return
			}
			httpresp.InternalErrorMessage(c, "Failed to get provider", err)
			return
		}
	}

	protocols, primaryProtocol, err := validateProtocolOverride(ctx, repos(), req.ProviderID, req.Protocols, req.PrimaryProtocol)
	if err != nil {
		writeProtocolOverrideError(c, err)
		return
	}

	// 写入闸与 apikeys 同一份净化器（common 下沉，两域口径单源，4a）：
	// 非法名/控制字符值/规范化撞名当场 400，杜绝脏值入库后出站非确定或
	// Client.Do 失败误伤凭据冷却/权重。
	cleanHeaders, err := common.SanitizeOutboundHeaders(req.CustomerHeaders)
	if err != nil {
		httpresp.BadRequest(c, "invalid headers: "+err.Error())
		return
	}
	customerHeaders := cleanHeaders
	if customerHeaders == nil {
		customerHeaders = map[string]string{}
	}

	priority := req.Priority
	if priority == 0 {
		// 与 settings.GetAutoPriorityDecayDefault 默认值一致
		priority = models.GetSettingInt(ctx, models.SettingKeyAutoPriorityDecayDefault, 100, 0)
	}

	modelProvider := models.ModelWithProvider{
		ModelID:       req.ModelID,
		ProviderModel: *req.ProviderModel,
		ProviderID:    req.ProviderID,
		// 能力三态：nil 落库为 NULL = 继承 Model 能力真值（新建关联的默认态）。
		ToolCall:         req.ToolCall,
		StructuredOutput: req.StructuredOutput,
		Image:            req.Image,
		WithHeader:       &req.WithHeader,
		CustomerHeaders:  customerHeaders,
		Weight:           req.Weight,
		Priority:         priority,
		MaxTokens:        req.MaxTokens,
		// 三态：nil=继承 model，true/false=override
		SupportsThinking: req.SupportsThinking,
		// 三态：nil=继承 model，空切片=显式不约束，非空=override
		ThinkingLevels: req.ThinkingLevels,
		// 协议覆盖三态：nil=继承供应商那层；非 nil=覆盖（可含供应商未配置的协议）
		Protocols:       protocols,
		PrimaryProtocol: primaryProtocol,
	}

	defaultStatus := true
	modelProvider.Status = &defaultStatus

	if err := repos().ModelWithProvider.Create(ctx, &modelProvider); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to create model-provider association", err)
		return
	}

	if models.GetSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate, false) {
		if err := SaveProviderModelToTemplate(ctx, modelProvider.ModelID, modelProvider.ProviderModel); err != nil {
			slog.Warn("failed to save provider model to template on create",
				"error", err,
				"model_id", modelProvider.ModelID,
				"provider_model", modelProvider.ProviderModel)
		}
	}

	httpresp.Success(c, modelProvider)
}

// UpdateModelProvider 更新模型提供商关联。
func UpdateModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelWithProviderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	slog.Debug("UpdateModelProvider", "id", id, "model_id", req.ModelID, "provider_id", req.ProviderID) // 不打印 req：CustomerHeaders 可能含敏感 header 值

	// customer_headers 保持原样：nil 时 struct Updates 跳过该字段（部分更新语义，保留旧值）。
	// 注意与 Create 路径不同：Create 需要空 map 兜底（新行无旧值可保留）。
	// 过闸与 Create 同一份净化器；nil 经 Sanitize 仍为 nil，部分更新语义不受影响。
	customerHeaders, err := common.SanitizeOutboundHeaders(req.CustomerHeaders)
	if err != nil {
		httpresp.BadRequest(c, "invalid headers: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 写入闸（与 Create 同一口径，差别只在缺省）：传了就不得为空，判 TrimSpace
	// ——全空格非零值会被 struct Updates 真正写进库，只判空串会漏掉这条路径。
	// 缺省（nil）=不改：部分更新语义，原值保留（与其余字段一致）。
	if req.ProviderModel != nil && strings.TrimSpace(*req.ProviderModel) == "" {
		httpresp.BadRequest(c, "provider_model is required")
		return
	}

	existing, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Model-provider association not found")
			return
		}
		httpresp.InternalErrorMessage(c, "Database error", err)
		return
	}

	// 校验改写目标存在，避免悬空关联（与 Create 对齐）。
	// GORM struct Updates 跳过零值，零值=不改，只校验非零 ID。
	if req.ModelID > 0 {
		if _, err := repos().Model.Get(ctx, req.ModelID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpresp.BadRequest(c, "Model not found")
				return
			}
			httpresp.InternalErrorMessage(c, "Failed to get model", err)
			return
		}
	}
	if req.ProviderID > 0 {
		if _, err := repos().Provider.Get(ctx, req.ProviderID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpresp.BadRequest(c, "Provider not found")
				return
			}
			httpresp.InternalErrorMessage(c, "Failed to get provider", err)
			return
		}
	}

	// 覆盖校验用「本次请求生效的供应商」：没传 provider_id 时沿用存量关联那家。
	effectiveProviderID := req.ProviderID
	if effectiveProviderID == 0 {
		effectiveProviderID = existing.ProviderID
	}
	protocols, primaryProtocol, err := validateProtocolOverride(ctx, repos(), effectiveProviderID, req.Protocols, req.PrimaryProtocol)
	if err != nil {
		writeProtocolOverrideError(c, err)
		return
	}

	// 五项三态字段（supports_thinking / thinking_levels / tool_call / structured_output / image）
	// 需要 nil 显式写 NULL（struct Updates 会跳过 nil 指针），其余字段保持 struct 部分更新
	// 语义（跳过零值，缺省不改）。两步在同一事务内执行（RunInTx），避免非原子：失败整体回滚。
	updates := models.ModelWithProvider{
		ModelID:         req.ModelID,
		ProviderID:      req.ProviderID,
		WithHeader:      &req.WithHeader,
		CustomerHeaders: customerHeaders,
		Weight:          req.Weight,
		Priority:        req.Priority,
		MaxTokens:       req.MaxTokens,
	}
	// provider_model 是 *string 三态，不能直接进 struct（Updates 期待 string）：nil 时
	// 保持零值让 GORM 跳过（缺省=不改），非 nil 时取解引用值（上面已判过非空）。
	if req.ProviderModel != nil {
		updates.ProviderModel = *req.ProviderModel
	}

	if err := repos().RunInTx(ctx, func(txRepos *repository.Repositories) error {
		if err := txRepos.ModelWithProvider.Update(ctx, uint(id), updates); err != nil {
			return err
		}
		// 非 nil 写 override；nil 写 NULL（改回"继承"）
		// supports_thinking + thinking_levels 都走 UpdateFields 显式写 NULL。
		// thinking_levels 需手动 JSON 序列化：GORM serializer:json 只对 struct Updates 生效，
		// map-based UpdateFields 不走 serializer，直接传 *[]string 会报 unsupported type。
		thinkingLevelsVal, err := models.SerializeStringSlicePtrForUpdate(req.ThinkingLevels)
		if err != nil {
			return err
		}
		// protocols 与 thinking_levels 同族：*[]string 三态，map 更新不走 serializer，
		// 需手动转 JSON；nil 写 NULL（= 继承供应商那层）。
		protocolsVal, err := models.SerializeStringSlicePtrForUpdate(protocols)
		if err != nil {
			return err
		}
		_, err = txRepos.ModelWithProvider.UpdateFields(ctx, uint(id), map[string]any{
			"supports_thinking": req.SupportsThinking,
			"thinking_levels":   thinkingLevelsVal,
			// 能力三兄弟同 supports_thinking：缺省 nil = 显式清回继承。
			"tool_call":         req.ToolCall,
			"structured_output": req.StructuredOutput,
			"image":             req.Image,
			// 协议覆盖同族：缺省 nil = 清回继承。
			"protocols":        protocolsVal,
			"primary_protocol": primaryProtocol,
		})
		return err
	}); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update model-provider association", err)
		return
	}

	updatedModelProvider, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to retrieve updated model-provider association", err)
		return
	}

	httpresp.Success(c, updatedModelProvider)
}

// DeleteModelProvider 删除模型提供商关联。
func DeleteModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := repos().ModelWithProvider.Delete(c.Request.Context(), uint(id))
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to delete model-provider association", err)
		return
	}

	if result == 0 {
		httpresp.NotFound(c, "Model-provider association not found")
		return
	}

	httpresp.Success(c, nil)
}

// BatchDeleteModelProviders 批量删除模型提供商关联。
func BatchDeleteModelProviders(c *gin.Context) {
	var req BatchDeleteModelProvidersRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	result, err := repos().ModelWithProvider.DeleteByIDs(c.Request.Context(), req.IDs)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to delete model-provider associations", err)
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": result,
	})
}
