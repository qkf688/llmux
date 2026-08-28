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

// GetPools 获取号池列表（名称搜索），每项附健康概览统计
// （key 数 / 状态分布 / 被分组引用数）。
func GetPools(c *gin.Context) {
	ctx := c.Request.Context()
	pools, err := repos().Pool.List(ctx, repository.PoolFilter{Name: c.Query("name")})
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	ids := make([]uint, 0, len(pools))
	for _, p := range pools {
		ids = append(ids, p.ID)
	}
	stats, err := repos().Pool.StatsByIDs(ctx, ids)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	refs, err := repos().KeyGroup.CountByPoolIDs(ctx, ids)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	items := make([]PoolListItem, 0, len(pools))
	for _, p := range pools {
		s := stats[p.ID]
		item := PoolListItem{
			Pool:         p,
			KeyCount:     s.KeyCount,
			StatusCounts: s.StatusCount,
			ReferencedBy: refs[p.ID],
		}
		// 无凭据/无引用的号池聚合不产生条目，序列化为 null 会让前端读不到 0，
		// 显式置空 map/零值保证键存在且为 0。
		if item.StatusCounts == nil {
			item.StatusCounts = map[string]int64{}
		}
		items = append(items, item)
	}
	httpresp.Success(c, items)
}

// CreatePool 创建号池（name 必填且唯一）。
func CreatePool(c *gin.Context) {
	var req PoolRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if req.Name == "" {
		httpresp.BadRequest(c, "Pool name is required")
		return
	}

	ctx := c.Request.Context()
	exists, err := repos().Pool.ExistsByName(ctx, req.Name)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	if exists {
		httpresp.BadRequest(c, "Pool name already exists")
		return
	}

	pool := &models.Pool{Name: req.Name, Note: req.Note}
	if err := repos().Pool.Create(ctx, pool); err != nil {
		httpresp.InternalServerError(c, "Failed to create pool: "+err.Error())
		return
	}
	httpresp.Success(c, pool)
}

// UpdatePool 更新号池（name 必填且唯一；note 可清空，走 UpdateFields）。
func UpdatePool(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	var req PoolRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if req.Name == "" {
		httpresp.BadRequest(c, "Pool name is required")
		return
	}

	ctx := c.Request.Context()
	pool, err := repos().Pool.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}

	if req.Name != pool.Name {
		exists, err := repos().Pool.ExistsByName(ctx, req.Name)
		if err != nil {
			httpresp.InternalServerError(c, err.Error())
			return
		}
		if exists {
			httpresp.BadRequest(c, "Pool name already exists")
			return
		}
	}

	if _, err := repos().Pool.UpdateFields(ctx, id, map[string]any{"name": req.Name, "note": req.Note}); err != nil {
		httpresp.InternalServerError(c, "Failed to update pool: "+err.Error())
		return
	}

	updated, err := repos().Pool.Get(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve updated pool: "+err.Error())
		return
	}
	httpresp.Success(c, updated)
}

// DeletePool 删除号池。被分组引用（key_groups.PoolID）时禁止删除；
// 未引用时在单事务中级联删除其下全部凭据 + 号池本体（软删）。
func DeletePool(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	pool, err := repos().Pool.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}

	refs, err := repos().KeyGroup.CountByPoolIDs(ctx, []uint{pool.ID})
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	if refs[pool.ID] > 0 {
		httpresp.BadRequest(c, "Pool is referenced by key groups; unbind them before deleting")
		return
	}

	err = repos().RunInTx(ctx, func(tx *repository.Repositories) error {
		if _, err := tx.Credential.DeleteByPoolID(ctx, pool.ID); err != nil {
			return err
		}
		if _, err := tx.Pool.Delete(ctx, pool.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete pool: "+err.Error())
		return
	}

	httpresp.Success(c, nil)
}
