package pools

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// maskKey 将明文 key 掩码为 sk-****ab12 形态（前缀 3 + **** + 后缀 4），
// 过短时退化为 ****，避免泄漏有效长度信息。
func maskKey(plain string) string {
	if len(plain) <= 4 {
		return "****"
	}
	if len(plain) <= 8 {
		return plain[:1] + "****" + plain[len(plain)-4:]
	}
	return plain[:3] + "****" + plain[len(plain)-4:]
}

// ListCredentials 获取指定号池的凭据分页列表（分页/状态筛选/关键词搜索+掩码）。
// GET /api/pools/:id/credentials?page=&page_size=&status=&q=
func ListCredentials(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	// 号池存在性校验：不存在直接 404
	ctx := c.Request.Context()
	if _, err := repos().Pool.Get(ctx, poolID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}

	page, pageSize, ok := httpx.ParsePaginationStrict(c)
	if !ok {
		return
	}

	// 状态筛选：空/all 不过滤，非法值 400（OCP 收敛至 models.IsValidCredentialStatus）
	status := c.Query("status")
	if status == "all" {
		status = ""
	}
	if status != "" {
		if !models.IsValidCredentialStatus(status) {
			httpresp.BadRequest(c, "Invalid status")
			return
		}
	}

	q := c.Query("q")

	creds, total, err := repos().Credential.ListPaged(ctx, repository.CredentialFilter{
		PoolID: &poolID,
		Status: status,
		Q:      q,
	}, page, pageSize)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	items := make([]CredentialListItem, 0, len(creds))
	cipher := credentialcrypto.Default()
	for _, cred := range creds {
		var masked string
		if cipher != nil {
			if dec, decErr := cipher.Decrypt(cred.Key); decErr == nil {
				masked = maskKey(dec)
			} else {
				// 解密失败不回退掩码密文，避免泄漏密文片段（密钥轮换/损坏场景）
				slog.Warn("credential decrypt failed, masking as ****", "credential_id", cred.ID, "error", decErr)
				masked = "****"
			}
		} else {
			// 无 cipher（如测试明文种子）直接掩码原文
			masked = maskKey(cred.Key)
		}
		items = append(items, CredentialListItem{
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
			TotalRequests:  cred.TotalRequests,
			TotalErrors:    cred.TotalErrors,
			TotalTokens:    cred.TotalTokens,
			CreatedAt:      cred.CreatedAt,
			UpdatedAt:      cred.UpdatedAt,
		})
	}
	if items == nil {
		items = []CredentialListItem{}
	}

	httpresp.Success(c, CredentialListData{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
