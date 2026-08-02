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
	"github.com/joho/godotenv"
	"github.com/qkf688/llmux/handler"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service"
	"github.com/qkf688/llmux/service/auth"
	_ "golang.org/x/crypto/x509roots/fallback"
)

func init() {
	// 加载 .env（文件不存在则静默跳过，不影响系统环境变量）
	_ = godotenv.Load()

	ctx := context.Background()
	models.Init(ctx, "./db/llmux.db")
	repository.SetDefault(repository.New(models.DB))
	slog.Info("TZ", "time.Local", time.Local.String())

	// bootstrap admin 账号
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		var err error
		password, err = auth.GenerateRandomPassword()
		if err != nil {
			panic("generate random admin password: " + err.Error())
		}
	}
	plain, err := auth.BootstrapAdmin(ctx, repository.Default().User, password)
	if err != nil {
		panic("bootstrap admin: " + err.Error())
	}
	auth.LogBootstrap(plain)
}

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET env is required")
		os.Exit(1)
	}

	ctx := context.Background()
	startBackgroundServices(ctx)

	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))

	handler.RegisterAll(router, handler.Deps{JWTSecret: jwtSecret})

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
