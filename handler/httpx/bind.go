// Package httpx 提供 handler 层共用的请求绑定辅助（分页 / 路径 ID / JSON）。
// 依赖 gin，不放入 common，避免 gin 进一步渗入公共包。
package httpx

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
)

// ParsePaginationStrict 解析 page / page_size。
// 缺省：page=1, page_size=20；非法值写 BadRequest 并返回 ok=false。
// 与 logs / healthcheck 现网行为一致。
func ParsePaginationStrict(c *gin.Context) (page, pageSize int, ok bool) {
	page = 1
	if pageStr := c.Query("page"); pageStr != "" {
		parsed, err := strconv.Atoi(pageStr)
		if err != nil || parsed < 1 {
			httpresp.BadRequest(c, "Invalid page parameter")
			return 0, 0, false
		}
		page = parsed
	}

	pageSize = 20
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		parsed, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsed < 1 || parsed > 100 {
			httpresp.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return 0, 0, false
		}
		pageSize = parsed
	}
	return page, pageSize, true
}

// ParsePaginationLoose 解析 page / page_size（modelsync 风格）。
// 缺省 DefaultQuery；非法 page 回落 1；非法 page_size 回落 20（含 >100）。
// 注意：List 查询使用解析后的原始值，响应中的 page/page_size 才做回落展示——保持现网顺序由调用方决定。
func ParsePaginationLoose(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return page, pageSize
}

// NormalizePaginationLoose 将 loose 分页归一化到响应展示值。
func NormalizePaginationLoose(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// ParseUintParam 解析路径参数为 uint；失败时 BadRequest 并返回 ok=false。
// id==0 也视为非法（与部分 detail 接口一致）。
func ParseUintParam(c *gin.Context, name string) (id uint, ok bool) {
	raw := strings.TrimSpace(c.Param(name))
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		httpresp.BadRequest(c, "Invalid ID format")
		return 0, false
	}
	return uint(v), true
}

// ParseUintParamAllowZero 解析路径参数；允许 0 以外的解析失败才 400；v 可为任意 uint（含 0 若能解析）。
// 多数 CRUD 使用：ParseUint 失败即 400，不额外校验 id==0。
func ParseUintParamAllowZero(c *gin.Context, name string) (id uint, ok bool) {
	raw := strings.TrimSpace(c.Param(name))
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return 0, false
	}
	return uint(v), true
}

// BindJSON 绑定 JSON，失败写 BadRequest。
func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return false
	}
	return true
}

// BindJSONAllowEOF 绑定 JSON，允许空 body（EOF）；其它错误 BadRequest。
func BindJSONAllowEOF(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil && !errors.Is(err, io.EOF) {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return false
	}
	return true
}
