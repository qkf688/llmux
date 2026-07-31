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

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service"
	_ "golang.org/x/crypto/x509roots/fallback"
)

func init() {
	ctx := context.Background()
	models.Init(ctx, "./db/llmux.db")
	repository.SetDefault(repository.New(models.DB))
	slog.Info("TZ", "time.Local", time.Local.String())
}

func main() {
	ctx := context.Background()
	startBackgroundServices(ctx)

	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))

	handler.RegisterAll(router, handler.Deps{Token: os.Getenv("TOKEN")})

	setwebui(router)
	router.Run(":7070")
}

// startBackgroundServices 启动健康检查与模型自动同步（Background 语义，与现网等价）。
func startBackgroundServices(ctx context.Context) {
	go service.GetHealthChecker().Start(ctx)

	syncService := service.NewModelSyncService(models.DB, nil)
	syncService.StartAutoSync(ctx)
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
