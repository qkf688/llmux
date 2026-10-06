package credentials

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/credview"
	"gorm.io/gorm"
)

// 凭据域（`/api/credentials`）：凭据是第一等资源，归属（pool_id）只是它的
// 字段与列表过滤条件。为什么独立成域：启停、备注、掩码、以后的凭据级设置（如重试次数）
// 只在这里实现一次，号池页与供应商分组页共用——按归属各写一份必然漂移
// （原先内联凭据就因为「启停只在号池域」而无法人工恢复，见设计定案第 5 节状态机出口）。

// ListCredentials 凭据列表：归属过滤（pool_id，可不带 = 全量）、状态筛选、
// 备注关键词 + 分页。归属过滤只存在于读路径。
// GET /api/credentials?pool_id=&status=&q=&page=&page_size=
func ListCredentials(c *gin.Context) {
	poolID, ok := parseUintQuery(c, "pool_id")
	if !ok {
		httpresp.BadRequest(c, "Invalid pool_id")
		return
	}
	page, pageSize, ok := httpx.ParsePaginationStrict(c)
	if !ok {
		return
	}

	// 状态筛选：空/all 不过滤，非法值 400（OCP 收敛至 models.IsValidCredentialStatus）
	status := strings.TrimSpace(c.Query("status"))
	if status == "all" {
		status = ""
	}
	if status != "" && !models.IsValidCredentialStatus(status) {
		httpresp.BadRequest(c, "Invalid status")
		return
	}

	creds, total, err := repos().Credential.ListPaged(c.Request.Context(), repository.CredentialFilter{
		PoolID: poolID,
		Status: status,
		Q:      c.Query("q"),
	}, page, pageSize)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	httpresp.Success(c, CredentialListData{
		Items:    credview.ViewsOf(creds),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetCredential 单条凭据（掩码视图）。
// GET /api/credentials/:id
func GetCredential(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	cred, ok := requireCredential(c, id)
	if !ok {
		return
	}
	httpresp.Success(c, credview.ViewOf(*cred))
}

// CreateCredential 往号池新增一条凭据（池内 KeyHash 去重 + AES-GCM 加密落库）。
// POST /api/credentials
func CreateCredential(c *gin.Context) {
	var req CreateCredentialRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if req.PoolID == 0 {
		httpresp.BadRequest(c, "pool_id is required")
		return
	}
	plain := strings.TrimSpace(req.Key)
	if plain == "" {
		httpresp.BadRequest(c, "Key is required")
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().Pool.Get(ctx, req.PoolID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err)
		return
	}

	cipher := credentialcrypto.Default()
	if cipher == nil {
		httpresp.InternalErrorMessage(c, "encryption not configured", nil)
		return
	}

	hash := cipher.Hash(plain)
	// 池内 KeyHash 去重（跨池同 key 允许复用）
	existing, err := repos().Credential.List(ctx, repository.CredentialFilter{PoolID: &req.PoolID, KeyHash: hash})
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
		PoolID:  req.PoolID,
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
	item := credview.ViewOf(*saved)
	// 用明文掩码覆盖（避免解密失败场景的 **** 与新建掩码不一致）
	item.KeyMasked = credentialcrypto.MaskKey(plain)
	httpresp.Success(c, item)
}

// GetCredentialRaw 明文查看（受控端点，解密返回明文 key）。响应带 `Cache-Control: no-store`：
// 明文落进浏览器磁盘缓存或中间代理就绕过了前端「翻页即作废 / 隐藏即删」的生命周期守卫。
// GET /api/credentials/:id/raw
func GetCredentialRaw(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	cred, ok := requireCredential(c, id)
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

// UpdateCredential 更新单条凭据（note 可清空，status 可切）。
// 切回 active 时按状态机约定连带重置鉴权连败计数（单一来源 models.CredentialRecoveryFields）——
// 这是「人工恢复」的落点：被判停（temp_unsched）或人工停用的凭据都经此回到生产。
// PATCH /api/credentials/:id
func UpdateCredential(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
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
	if _, ok := requireCredential(c, id); !ok {
		return
	}

	ctx := c.Request.Context()
	fields := map[string]any{}
	if req.Note != nil {
		fields["note"] = *req.Note
	}
	if req.Status != nil {
		fields["status"] = *req.Status
		for k, v := range models.CredentialRecoveryFields(*req.Status) {
			fields[k] = v
		}
	}
	if _, err := repos().Credential.UpdateFields(ctx, id, fields); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update credential", err)
		return
	}
	updated, err := repos().Credential.Get(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	httpresp.Success(c, credview.ViewOf(*updated))
}

// DeleteCredential 删除单条凭据（软删）。凭据只归属号池，供应商表单不持有凭据，
// 故此入口是凭据删除的唯一写路径（管理端 UI 在号池页提供）。
// DELETE /api/credentials/:id
func DeleteCredential(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	if _, ok := requireCredential(c, id); !ok {
		return
	}
	if _, err := repos().Credential.Delete(c.Request.Context(), id); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to delete credential", err)
		return
	}
	httpresp.Success(c, nil)
}

// requireCredential 按 ID 取凭据：不存在写 404（返回 false，响应已写出）。
// 凭据是第一等资源后，读路径不再有「越池/越组」概念——ID 就是资源标识，
// 归属校验只在列表过滤（pool_id）里需要。
func requireCredential(c *gin.Context, id uint) (*models.Credential, bool) {
	cred, err := repos().Credential.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Credential not found")
			return nil, false
		}
		httpresp.InternalServerError(c, err)
		return nil, false
	}
	return cred, true
}

// parseUintQuery 解析可选 uint 查询参数：缺省/空串 → (nil, true)；非法或 0 → (nil, false)，
// 调用方回 400。0 不是合法凭据归属 ID，显式拒绝好过静默按「不过滤」处理。
func parseUintQuery(c *gin.Context, name string) (*uint, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return nil, false
	}
	id := uint(v)
	return &id, true
}
