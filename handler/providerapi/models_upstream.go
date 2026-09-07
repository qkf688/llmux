package providerapi

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service/modelsync"
	"gorm.io/gorm"
)

// GetProviderModels 拉取供应商上游模型列表（管理端「上游模型」弹窗数据源）。
//
// 选路委托 modelsync.ListUpstreamModels——与落库同步同一条链路（协议端点 + 分组凭据 +
// base_url 继承链）。禁止在此直接用 Provider.Config 组装客户端：S6 起 config 不再承载
// api_key（凭据只存在于 credentials 表，运行时由 channel.BuildConfig 注入明文），
// 绕过选路等于不带 Authorization 打上游，号池/内联凭据的供应商恒 401。
func GetProviderModels(c *gin.Context) {
	providerID, err := parseUintParam(c.Param("id"))
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	modelList, err := modelsync.ListUpstreamModels(c.Request.Context(), repos(), providerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Provider not found")
			return
		}
		// 信封 code=404 + HTTP 200：前端 apiRequest 按 code!=200 抛出 message，
		// 弹窗直接把上游原文（含 request_id）展示给用户，沿用既有契约不改状态码。
		httpresp.NotFound(c, "Failed to get models: "+err.Error())
		return
	}
	// ListUpstreamModels 恒返回非 nil 切片（空列表序列化为 [] 而非 null，避免前端白屏）
	httpresp.Success(c, modelList)
}
