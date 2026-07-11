package database

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

// ExportDatabase 导出完整数据库文件。
func ExportDatabase(c *gin.Context) {
	dbPath := models.GetDBPath()

	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		common.NotFound(c, "数据库文件不存在")
		return
	}
	if err != nil {
		slog.Error("无法访问数据库文件", "error", err, "path", dbPath)
		common.InternalServerError(c, "无法访问数据库文件")
		return
	}

	if fileInfo.Size() > 100*1024*1024 { // 100MB
		slog.Warn("导出的数据库文件较大", "size", fileInfo.Size(), "path", dbPath)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("llmio_backup_%s.db", timestamp)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	slog.Info("数据库导出", "filename", filename, "size", fileInfo.Size())
	c.File(dbPath)
}