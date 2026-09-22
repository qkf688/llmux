package database

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// VacuumDatabase 执行数据库真空操作。
func VacuumDatabase(c *gin.Context) {
	if err := models.DB.Exec("VACUUM").Error; err != nil {
		httpresp.InternalErrorMessage(c, "执行数据库真空操作失败", err)
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"message":   "数据库真空操作执行成功",
		"timestamp": time.Now(),
	})
}
