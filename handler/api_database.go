package handler

import "github.com/atopos31/llmio/handler/database"

// GetDatabaseStats 获取数据库统计信息（兼容层）。
var GetDatabaseStats = database.GetDatabaseStats

// VacuumDatabase 执行数据库真空操作（兼容层）。
var VacuumDatabase = database.VacuumDatabase