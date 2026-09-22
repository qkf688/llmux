package pools

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// credentialToListItem 将 models.Credential 转为 CredentialListItem（掩码形态，不泄漏明文/密文/KeyHash）。
func credentialToListItem(cred models.Credential) CredentialListItem {
	// 掩码与降级策略（cipher 未配置/解密失败 → ****）统一走 credentialcrypto.MaskStored，
	// 与供应商内联 key 详情同源，避免两域各自拼装漂移。
	masked := credentialcrypto.MaskStored(credentialcrypto.Default(), cred.Key, "credential_id", cred.ID)
	return CredentialListItem{
		ID:             cred.ID,
		PoolID:         cred.PoolID,
		GroupID:        cred.GroupID,
		Status:         cred.Status,
		Note:           cred.Note,
		KeyMasked:      masked,
		CooldownUntil:  cred.CooldownUntil,
		CooldownReason: cred.CooldownReason,
		FailCount:      cred.FailCount,
		LastUsedAt:     cred.LastUsedAt,
		LastProbeAt:    cred.LastProbeAt,
		TotalRequests:  cred.TotalRequests,
		TotalErrors:    cred.TotalErrors,
		TotalTokens:    cred.TotalTokens,
		CreatedAt:      cred.CreatedAt,
		UpdatedAt:      cred.UpdatedAt,
	}
}

// CreateCredential 创建单条凭据（池内 KeyHash 去重 + AES-GCM 加密落库）。
// POST /api/pools/:id/credentials
func CreateCredential(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	if _, ok := requirePool(c, ctx, poolID); !ok {
		return
	}

	var req CreateCredentialRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	plain := strings.TrimSpace(req.Key)
	if plain == "" {
		httpresp.BadRequest(c, "Key is required")
		return
	}

	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalErrorMessage(c, "encryption not configured", nil)
		return
	}

	hash := cipher.Hash(plain)
	// 池内 KeyHash 去重（跨池同 key 允许复用）
	existing, err := repos().Credential.List(ctx, repository.CredentialFilter{PoolID: &poolID, KeyHash: hash})
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	if len(existing) > 0 {
		httpresp.BadRequest(c, "Key already exists in this pool")
		return
	}

	enc, err := cipher.Encrypt(plain)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to encrypt key", err)
		return
	}

	cred := &models.Credential{
		Key:     enc,
		KeyHash: hash,
		PoolID:  &poolID,
		Status:  models.CredentialStatusActive,
		Note:    req.Note,
	}
	if err := repos().Credential.Create(ctx, cred); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to create credential", err)
		return
	}

	// 重新取回以获得完整 GORM 字段（CreatedAt 等），但 KeyMasked 基于明文直接掩码避免二次解密
	saved, err := repos().Credential.Get(ctx, cred.ID)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	item := credentialToListItem(*saved)
	// 用明文掩码覆盖（避免解密失败场景的 **** 与新建掩码不一致）
	item.KeyMasked = credentialcrypto.MaskKey(plain)
	httpresp.Success(c, item)
}

// getCredentialInPool 获取指定号池下的凭据，统一「池校验 + 凭据获取 + 越池判断」：
// 池不存在 404、凭据不存在 404、越池 404（不暴露凭据存在性）。
// 返回 false 时响应已写出，调用方直接 return。池实体随校验一并带出（按凭据测试
// 要读池级 test_model，免二次查询）；不需要的调用方忽略第二个返回值。
func getCredentialInPool(c *gin.Context, ctx context.Context, poolID, credID uint) (*models.Credential, *models.Pool, bool) {
	pool, ok := requirePool(c, ctx, poolID)
	if !ok {
		return nil, nil, false
	}
	cred, err := repos().Credential.Get(ctx, credID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Credential not found")
			return nil, nil, false
		}
		httpresp.InternalServerError(c, err)
		return nil, nil, false
	}
	if cred.PoolID == nil || *cred.PoolID != poolID {
		httpresp.NotFound(c, "Credential not found")
		return nil, nil, false
	}
	return cred, pool, true
}

// GetCredential 获取单条凭据（掩码形态，池内限界防越池）。
// GET /api/pools/:id/credentials/:credId
func GetCredential(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	credID, ok := httpx.ParseUintParamAllowZero(c, "credId")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	cred, _, ok := getCredentialInPool(c, ctx, poolID, credID)
	if !ok {
		return
	}
	httpresp.Success(c, credentialToListItem(*cred))
}

// GetCredentialRaw 明文查看（受控端点，解密返回明文 key）。响应带 `Cache-Control: no-store`：
// 明文落进浏览器磁盘缓存或中间代理就绕过了前端「翻页即作废 / 隐藏即删」的生命周期守卫。
// GET /api/pools/:id/credentials/:credId/raw
func GetCredentialRaw(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	credID, ok := httpx.ParseUintParamAllowZero(c, "credId")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	cred, _, ok := getCredentialInPool(c, ctx, poolID, credID)
	if !ok {
		return
	}
	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalErrorMessage(c, "encryption not configured", nil)
		return
	}
	plain, err := cipher.Decrypt(cred.Key)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to decrypt key", err, "credential_id", cred.ID)
		return
	}
	httpresp.SuccessNoStore(c, CredentialRawData{Key: plain})
}

// UpdateCredential 更新单条凭据（note 可清空，status 需合法）。
// PATCH /api/pools/:id/credentials/:credId
func UpdateCredential(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	credID, ok := httpx.ParseUintParamAllowZero(c, "credId")
	if !ok {
		return
	}

	var req UpdateCredentialRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if req.Note == nil && req.Status == nil {
		httpresp.BadRequest(c, "No fields to update")
		return
	}
	if req.Status != nil && !models.IsValidCredentialStatus(*req.Status) {
		httpresp.BadRequest(c, "Invalid status")
		return
	}

	ctx := c.Request.Context()
	_, _, ok = getCredentialInPool(c, ctx, poolID, credID)
	if !ok {
		return
	}

	fields := map[string]any{}
	if req.Note != nil {
		fields["note"] = *req.Note
	}
	if req.Status != nil {
		fields["status"] = *req.Status
		// 恢复 active 时按状态机约定连带重置鉴权失败计数（单一来源 models.CredentialRecoveryFields）
		for k, v := range models.CredentialRecoveryFields(*req.Status) {
			fields[k] = v
		}
	}
	if _, err := repos().Credential.UpdateFields(ctx, credID, fields); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update credential", err)
		return
	}
	updated, err := repos().Credential.Get(ctx, credID)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	httpresp.Success(c, credentialToListItem(*updated))
}

// DeleteCredential 删除单条凭据（池内限界软删）。
// DELETE /api/pools/:id/credentials/:credId
func DeleteCredential(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	credID, ok := httpx.ParseUintParamAllowZero(c, "credId")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	_, _, ok = getCredentialInPool(c, ctx, poolID, credID)
	if !ok {
		return
	}
	if _, err := repos().Credential.Delete(ctx, credID); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to delete credential", err)
		return
	}
	httpresp.Success(c, nil)
}
