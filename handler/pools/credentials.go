package pools

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// maskKey 将明文 key 掩码为 sk-****ab12 形态（前缀 3 + **** + 后缀 4），
// 过短时（≤8）一律退化为 ****：1+4 组合在 len 5-8 时几乎完整还原明文。
func maskKey(plain string) string {
	if len(plain) <= 8 {
		return "****"
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
	for _, cred := range creds {
		items = append(items, credentialToListItem(cred))
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
