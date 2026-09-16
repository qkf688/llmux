package pools

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// requirePool 校验号池存在并带出池实体：不存在写 404、查询异常写 500，返回 (nil, false)
// （响应已写出，调用方直接 return）；存在返回 (*Pool, true)。池存在性判定在 pools 域
// 所有凭据端点统一走此 helper（防 404/500 语义在各 handler 分裂）。
// 池实体随返回值带出：存在性判定与加载本是同一动作，按凭据测试（probe）直接消费，
// 避免同请求对同一行重复查询。
func requirePool(c *gin.Context, ctx context.Context, poolID uint) (*models.Pool, bool) {
	pool, err := repos().Pool.Get(ctx, poolID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return nil, false
		}
		httpresp.InternalServerError(c, err)
		return nil, false
	}
	return pool, true
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
	if _, ok := requirePool(c, ctx, poolID); !ok {
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
		httpresp.InternalServerError(c, err)
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
