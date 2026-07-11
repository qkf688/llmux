package handler

import (
	"github.com/atopos31/llmio/handler/database"
	"github.com/atopos31/llmio/handler/importexport"
)

// ExportConfig 导出系统配置（兼容层）。
var ExportConfig = importexport.ExportConfig

// ImportConfig 导入系统配置（兼容层）。
var ImportConfig = importexport.ImportConfig

// ExportDatabase 导出完整数据库文件（兼容层；实现位于 database 包）。
var ExportDatabase = database.ExportDatabase