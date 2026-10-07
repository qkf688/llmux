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
	// 系统语义。写失败即收口 500，不带 raw err。
	//
	// 命名只由 exportSnapshotPath 单点产出：清理回归断言要锚住「实现的真实命名」，
	// 命名散在实现与测试两处时，实现改名会让断言静默失效（Stage 41 修的正是这个形态）。
	// 注意时间戳只到**秒**：同秒并发导出会算出同一条路径、后到者 VACUUM INTO 必失败
	//（既有缺陷，已随导出族登记 find-issues），此处不顺手改口径。
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	snapshotPath := exportSnapshotPath(dbPath, now)
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

// exportSnapshotPath 返回一次导出的临时快照路径：<库路径>.export-<时间戳>.tmp。
//
// 命名收成单点，是为了让「导出后快照不残留」的回归断言能锚在**实现真实产出的名字**上
// （export_test.go 的控制表第一行由本函数产出）。命名一旦散成实现与测试两份字面量，
// 实现改名时断言不吭声、目录扫描恒绿，那就是 Stage 41 修掉的假绿形态。
func exportSnapshotPath(dbPath string, ts time.Time) string {
	return dbPath + fmt.Sprintf(".export-%s.tmp", ts.Format("20060102_150405"))
}
