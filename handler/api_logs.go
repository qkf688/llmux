package handler

import (
	"github.com/atopos31/llmio/handler/healthchecklogs"
	"github.com/atopos31/llmio/handler/logs"
	"github.com/atopos31/llmio/handler/modelsynclogs"
)

// BatchDeleteLogsRequest 批量删除日志请求结构（兼容层）。
type BatchDeleteLogsRequest = logs.BatchDeleteLogsRequest

// GetRequestLogs 获取请求日志（兼容层）。
var GetRequestLogs = logs.GetRequestLogs

// GetRequestLogDetail 获取请求日志详情（兼容层）。
var GetRequestLogDetail = logs.GetRequestLogDetail

// GetChatIO 获取 ChatIO（兼容层）。
var GetChatIO = logs.GetChatIO

// GetUserAgents 获取 UserAgent 列表（兼容层）。
var GetUserAgents = logs.GetUserAgents

// DeleteLog 删除单条日志（兼容层）。
var DeleteLog = logs.DeleteLog

// BatchDeleteLogs 批量删除日志（兼容层）。
var BatchDeleteLogs = logs.BatchDeleteLogs

// ClearAllLogs 清空全部请求日志（兼容层）。
var ClearAllLogs = logs.ClearAllLogs

// ClearFilteredLogs 按条件清空请求日志（兼容层）。
var ClearFilteredLogs = logs.ClearFilteredLogs

// GetHealthCheckLogs 获取健康检测日志（兼容层）。
var GetHealthCheckLogs = healthchecklogs.GetHealthCheckLogs

// ClearHealthCheckLogs 清空健康检测日志（兼容层）。
var ClearHealthCheckLogs = healthchecklogs.ClearHealthCheckLogs

// GetModelSyncLogs 获取模型同步日志（兼容层）。
var GetModelSyncLogs = modelsynclogs.GetModelSyncLogs

// DeleteModelSyncLogs 批量删除模型同步日志（兼容层）。
var DeleteModelSyncLogs = modelsynclogs.DeleteModelSyncLogs

// ClearModelSyncLogs 清空模型同步日志（兼容层）。
var ClearModelSyncLogs = modelsynclogs.ClearModelSyncLogs

// ClearModelSyncErrorLogs 清空错误模型同步日志（兼容层）。
var ClearModelSyncErrorLogs = modelsynclogs.ClearModelSyncErrorLogs