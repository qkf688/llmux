package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/atopos31/llmio/handler"
	"github.com/atopos31/llmio/middleware"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/crypto/x509roots/fallback"
)

func init() {
	ctx := context.Background()
	models.Init(ctx, "./db/llmio.db")
	slog.Info("TZ", "time.Local", time.Local.String())

	// 启动健康检测服务
	go service.GetHealthChecker().Start(ctx)

	// 启动模型自动同步服务
	syncService := service.NewModelSyncService(models.DB)
	syncService.StartAutoSync(ctx)
}

func main() {
	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))

	authOpenAI := middleware.Auth(os.Getenv("TOKEN"))
	authAnthropic := middleware.AuthAnthropic(os.Getenv("TOKEN"))

	v1 := router.Group("/v1")
	v1.GET("/models", authOpenAI, handler.ModelsHandler)

	v1.POST("/chat/completions", authOpenAI, handler.ChatCompletionsHandler)
	v1.POST("/responses", authOpenAI, handler.ResponsesHandler)
	v1.POST("/messages", authAnthropic, handler.Messages)
	// TODO
	v1.POST("/count_tokens", authAnthropic)

	api := router.Group("/api")
	api.Use(middleware.Auth(os.Getenv("TOKEN")))
	api.GET("/metrics/use/:days", handler.Metrics)
	api.GET("/metrics/dailies/:days", handler.MetricsDailies)
	api.GET("/metrics/hourlies/today", handler.MetricsHourliesToday)
	api.GET("/metrics/counts", handler.Counts)
	api.GET("/metrics/real-model-counts", handler.RealModelCounts)
	api.GET("/metrics/total", handler.MetricsTotal)
	api.GET("/metrics/providers", handler.ProviderMetrics)
	// Provider management
	api.GET("/providers/template", handler.GetProviderTemplates)
	api.GET("/providers", handler.GetProviders)
	api.GET("/providers/models/:id", handler.GetProviderModels)
	api.POST("/providers/:id/test", handler.ProviderModelTestHandler)
	api.POST("/providers", handler.CreateProvider)
	api.PUT("/providers/:id", handler.UpdateProvider)
	api.DELETE("/providers/:id", handler.DeleteProvider)
	api.DELETE("/providers/:id/associations", handler.ClearProviderAssociations)

	// Model management
	api.GET("/models", handler.GetModels)
	api.POST("/models", handler.CreateModel)
	api.PUT("/models/:id", handler.UpdateModel)
	api.GET("/models/:id/template", handler.GetModelTemplate)
	api.POST("/models/:id/template/items", handler.AddModelTemplateItem)
	api.DELETE("/models/:id/template/items", handler.DeleteModelTemplateItem)
	api.DELETE("/models/batch", handler.BatchDeleteModels)
	api.PUT("/models/batch", handler.BatchUpdateModels)
	api.DELETE("/models/:id", handler.DeleteModel)

	// Model-provider association management
	api.GET("/model-providers", handler.GetModelProviders)
	api.GET("/model-providers/status", handler.GetModelProviderStatus)
	api.GET("/model-providers/health-status", handler.GetModelProviderHealthStatus)
	api.POST("/model-providers", handler.CreateModelProvider)
	api.PUT("/model-providers/:id", handler.UpdateModelProvider)
	api.PATCH("/model-providers/:id/status", handler.UpdateModelProviderStatus)
	api.PATCH("/model-providers/batch/status", handler.BatchUpdateModelProvidersStatus)
	api.PATCH("/model-providers/batch/capabilities", handler.BatchUpdateModelProvidersCapabilities)
	api.DELETE("/model-providers/batch", handler.BatchDeleteModelProviders)
	api.DELETE("/model-providers/:id", handler.DeleteModelProvider)
	api.GET("/model-providers/auto-associate/preview", handler.PreviewAutoAssociate)
	api.POST("/model-providers/auto-associate", handler.AutoAssociateModels)
	api.GET("/providers/blacklist", handler.GetProviderBlacklist)
	api.PUT("/providers/blacklist", handler.UpdateProviderBlacklist)
	api.GET("/model-providers/clean-invalid/preview", handler.PreviewCleanInvalid)
	api.POST("/model-providers/clean-invalid", handler.CleanInvalidAssociations)

	// System status and monitoring
	api.GET("/logs", handler.GetRequestLogs)
	api.GET("/logs/:id", handler.GetRequestLogDetail)
	api.GET("/logs/:id/chat-io", handler.GetChatIO)
	api.DELETE("/logs/batch", handler.BatchDeleteLogs)
	api.DELETE("/logs/clear", handler.ClearAllLogs)
	api.DELETE("/logs/clear-filtered", handler.ClearFilteredLogs)
	api.DELETE("/logs/:id", handler.DeleteLog)
	api.GET("/user-agents", handler.GetUserAgents)
	api.POST("/maintenance/vacuum", handler.VacuumDatabase)

	// System configuration
	api.GET("/config", handler.GetSystemConfig)
	api.PUT("/config", handler.UpdateSystemConfig)
	api.GET("/system/database-stats", handler.GetDatabaseStats)
	api.GET("/system/export-config", handler.ExportConfig)
	api.GET("/system/export-database", handler.ExportDatabase)
	api.POST("/system/import-config", handler.ImportConfig)

	// Settings
	api.GET("/settings", handler.GetSettings)
	api.PUT("/settings", handler.UpdateSettings)
	api.POST("/settings/reset-weights", handler.ResetModelWeights)
	api.POST("/settings/reset-priorities", handler.ResetModelPriorities)
	api.POST("/settings/enable-all-associations", handler.EnableAllAssociations)

	// Health check management
	api.GET("/health-check/settings", handler.GetHealthCheckSettings)
	api.PUT("/health-check/settings", handler.UpdateHealthCheckSettings)
	api.GET("/health-check/logs", handler.GetHealthCheckLogs)
	api.DELETE("/health-check/logs", handler.ClearHealthCheckLogs)
	api.POST("/health-check/run/:id", handler.RunHealthCheck)
	api.POST("/health-check/run-all", handler.RunHealthCheckAll)
	api.GET("/health-check/batch/:batchId", handler.GetBatchHealthCheckStatus)

	// Provider connectivity test
	api.GET("/test/:id", handler.ProviderTestHandler)
	api.GET("/test/react/:id", handler.TestReactHandler)
	api.GET("/test/structured-output/:id", handler.TestStructuredOutputHandler)

	// Model sync management
	api.POST("/model-sync/:id", handler.SyncProviderModels)
	api.POST("/model-sync/all", handler.SyncAllProviders)
	api.GET("/model-sync/stats", handler.GetModelSyncStats)
	api.GET("/model-sync/logs", handler.GetModelSyncLogs)
	api.GET("/model-sync/recent-added-models", handler.GetRecentAddedModels)
	api.DELETE("/model-sync/logs", handler.DeleteModelSyncLogs)
	api.DELETE("/model-sync/logs/clear", handler.ClearModelSyncLogs)
	api.DELETE("/model-sync/logs/clear-errors", handler.ClearModelSyncErrorLogs)

	// Virtual model management
	api.GET("/virtual-models", handler.GetVirtualModels)
	api.POST("/virtual-models", handler.CreateVirtualModel)
	api.PUT("/virtual-models/:id", handler.UpdateVirtualModel)
	api.DELETE("/virtual-models/:id", handler.DeleteVirtualModel)
	api.GET("/virtual-models/:id/mappings", handler.GetVirtualModelMappings)
	api.POST("/virtual-models/:id/mappings", handler.CreateVirtualModelMapping)
	api.POST("/virtual-models/:id/mappings/batch", handler.BatchCreateVirtualModelMapping)
	api.DELETE("/virtual-models/:id/mappings/batch", handler.BatchDeleteVirtualModelMapping)
	api.PUT("/virtual-models/:id/mappings/:mapping_id", handler.UpdateVirtualModelMapping)
	api.DELETE("/virtual-models/:id/mappings/:mapping_id", handler.DeleteVirtualModelMapping)
	api.GET("/virtual-models/:id/stats", handler.GetVirtualModelStats)

	setwebui(router)
	router.Run(":7070")
}

//go:embed webui/dist
var distFiles embed.FS

//go:embed webui/dist/index.html
var indexHTML []byte

func setwebui(r *gin.Engine) {
	subFS, err := fs.Sub(distFiles, "webui/dist/assets")
	if err != nil {
		panic(err)
	}

	r.StaticFS("/assets", http.FS(subFS))

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && !strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte("404 Not Found"))
	})
}
