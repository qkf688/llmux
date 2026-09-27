package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// ExportDatabase 导出完整数据库文件。
//
// 不直接 c.File 主库文件：SQLite WAL 模式下最近写入先进 -wal 边车文件，checkpoint
// 才合并进主文件——直接回主文件会把「建表语句之外的新行」整段漏掉（回归
// TestExportDatabase_ContainsUncheckpointedWALWrites 改前实测红：导出物缺种子行）。
// 这里用 VACUUM INTO 打一致性快照（含 WAL 内容），写临时文件、响应完成后删除；
// 与 Stage 16 存量清洗同款 VACUUM 语义（system-ops 白名单直连）。
func ExportDatabase(c *gin.Context) {
	dbPath := models.GetDBPath()

	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		httpresp.NotFound(c, "数据库文件不存在")
		return
	}
	if err != nil {
		httpresp.InternalErrorMessage(c, "无法访问数据库文件", err, "path", dbPath)
		return
	}

	if fileInfo.Size() > 100*1024*1024 { // 100MB
		slog.Warn("导出的数据库文件较大", "size", fileInfo.Size(), "path", dbPath)
	}

	// 临时快照放主库同目录：跨盘 rename 无关（此处不 rename），但同目录保证同文件
	// 系统语义；路径含时间戳防并发导出互踩。写失败即收口 500，不带 raw err。
	timestamp := time.Now().Format("20060102_150405")
	snapshotPath := dbPath + fmt.Sprintf(".export-%s.tmp", timestamp)
	defer func() {
		if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("清理导出临时快照失败", "path", snapshotPath, "error", err)
		}
	}()

	if err := models.DB.Exec("VACUUM INTO ?", snapshotPath).Error; err != nil {
		httpresp.InternalErrorMessage(c, "生成数据库导出快照失败", err)
		return
	}

	snapshotInfo, err := os.Stat(snapshotPath)
	if err != nil {
		httpresp.InternalErrorMessage(c, "读取数据库导出快照失败", err, "path", snapshotPath)
		return
	}

	filename := fmt.Sprintf("llmux_backup_%s.db", timestamp)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", snapshotInfo.Size()))

	slog.Info("数据库导出", "filename", filename, "size", snapshotInfo.Size(), "source", filepath.Base(dbPath))
	c.File(snapshotPath)
}
