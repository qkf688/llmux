package logs

import (
	"strconv"

	"github.com/atopos31/llmio/httpresp"
	"github.com/gin-gonic/gin"
)

// GetChatIO 查询指定日志的输入输出记录。
func GetChatIO(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httpresp.NotFound(c, "ChatIO not found")
		return
	}

	chatIO, err := repos().ChatIO.GetByLogID(c.Request.Context(), uint(id))
	if err != nil {
		httpresp.NotFound(c, "ChatIO not found")
		return
	}

	httpresp.Success(c, chatIO)
}

// GetUserAgents 获取所有不重复的用户代理种类。
func GetUserAgents(c *gin.Context) {
	userAgents, err := repos().ChatLog.DistinctUserAgents(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, "Failed to query user agents: "+err.Error())
		return
	}
	httpresp.Success(c, userAgents)
}
