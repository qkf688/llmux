package pools

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// 池作用域守卫：凭据提为第一等资源后（handler/credentials），号池域只剩两个仍需
// 「池作用域」的动作——批量导入（往这个池里灌 key）与按凭据测试（测试模型是池级配置
// `Pool.TestModel`）。这两个入口的入参是池，故保留池存在性与归属校验。

// requirePool 校验号池存在并带出池实体：不存在写 404、查询异常写 500，返回 (nil, false)
// （响应已写出，调用方直接 return）；存在返回 (*Pool, true)。
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

// getCredentialInPool 获取指定号池下的凭据，统一「池校验 + 凭据获取 + 越池判断」：
// 池不存在 404、凭据不存在 404、越池 404（不暴露凭据存在性）。
// 返回 false 时响应已写出，调用方直接 return。池实体随校验一并带出（按凭据测试
// 要读池级 test_model，免二次查询）。
//
// 越池为何仍判 404 而不是改成按凭据 ID 直取：入口本身就是池详情的「按凭据测试」，
// 入参是 (池, 凭据) 一对——URL 里带了池就该按池校验；凭据级的读/写走 handler/credentials，
// 那里没有归属概念。
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
