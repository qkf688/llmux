package database

import (
	"log/slog"
	"time"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

// VacuumDatabase 执行数据库真空操作。
func VacuumDatabase(c *gin.Context) {
	if err := models.DB.Exec("VACUUM").Error; err != nil {
		slog.Error("执行数据库真空操作失败", "error", err)
		httpresp.InternalServerError(c, "执行数据库真空操作失败: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"message":   "数据库真空操作执行成功",
		"timestamp": time.Now(),
	})
}