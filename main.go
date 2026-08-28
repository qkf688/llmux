package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/common/credentialcrypto"
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
}

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET env is required")
		os.Exit(1)
	}

	// 凭据加密密钥解析（在 models.Init 之前：存量迁移需要它加密 api_key）。
	// 与 JWT_SECRET 同地位：部署级永久密钥；未设则首启生成并落盘 ./db/credential.key。
	credKey, err := credentialcrypto.ResolveKey(os.Getenv("CREDENTIAL_ENCRYPTION_KEY"), "./db/credential.key")
	if err != nil {
		slog.Error("resolve credential encryption key", "error", err)
		os.Exit(1)
	}
	credCipher, err := credentialcrypto.New(credKey)
	if err != nil {
		slog.Error("invalid CREDENTIAL_ENCRYPTION_KEY (need 32-byte hex)", "error", err)
		os.Exit(1)
	}
	credentialcrypto.SetDefault(credCipher)

	// 启动装配：建库/迁移与 repository 绑定必须早于任何仓储使用者（BootstrapAdmin 等）。
	// 放在 main 而非 init：init 里做 IO 会让 package main 的测试一跑就迁移开发库；
	// 放在 JWT_SECRET 校验之后，缺密钥时也不会白建库。
	models.Init(context.Background(), "./db/llmux.db")
	repository.SetDefault(repository.New(models.DB))
	slog.Info("TZ", "time.Local", time.Local.String())

	// 信号 ctx 是所有长驻后台循环的退出信号源；取消它才能让 healthcheck /
	// modelsync 的 ticker 循环停下来，进而被 bgtask 等到。
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// 再包一层可取消 ctx，使「启动失败」与「收到信号」共用同一条关闭路径。
	// stopSignals 只停止信号投递、不会取消 sigCtx，故不能用它触发关闭。
	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()

	// bgtask 默认实例与 repository.SetDefault 同为启动装配的一环：请求路径里
	// fire-and-forget 的写库任务都登记到它，关闭时统一排空。
	bgtask.SetDefault(bgtask.NewManager())

	// bootstrap admin 账号（在 JWT_SECRET 校验之后，避免缺密钥时白建 admin）
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	fromEnv := adminPassword != ""
	if !fromEnv {
		var err error
		adminPassword, err = auth.GenerateRandomPassword()
		if err != nil {
			slog.Error("generate random admin password", "error", err)
			os.Exit(1)
		}
	}
	plain, err := auth.BootstrapAdmin(ctx, repository.Default().User, adminPassword)
	if err != nil {
		slog.Error("bootstrap admin", "error", err)
		os.Exit(1)
	}
	auth.LogBootstrap(plain, fromEnv)

	startBackgroundServices(ctx)

	router := gin.Default()

	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))

	handler.RegisterAll(router, handler.Deps{JWTSecret: jwtSecret})

	setwebui(router)

	srv := &http.Server{Addr: ":7070", Handler: router}
	go func() {
		// ErrServerClosed 是 Shutdown 的正常返回，不是启动失败。
		// 启动失败不能 os.Exit：此时 startBackgroundServices 已登记了可能正在写库的
		// 后台任务，直接退进程会跳过 shutdown 把它们硬切。取消 ctx 走同一条关闭序。
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
			cancel()
		}
	}()
	slog.Info("server started", "addr", srv.Addr)

	<-ctx.Done()
	shutdown(srv, bgtask.Default(), models.Close)
}

// 关闭各阶段的等待上限。
//
// serverShutdownTimeout 必须存在而非「等到自然结束」：流式（SSE）响应在 handler 层
// 由 io.Copy 阻塞回写，HTTP client 无整体超时，上游不结束流就永远不结束——没有上限
// 会让 Shutdown 无限挂住。超时后剩余连接被强制关闭，代价是截断在途流式响应。
const (
	serverShutdownTimeout = 30 * time.Second
	bgtaskDrainTimeout    = 10 * time.Second
)

// serverShutdowner 抽出 *http.Server 的关闭能力：shutdown 只需要「能停机」这一项，
// 依赖能力而非具体类型（DIP），顺便让关闭顺序可被测试断言而不必真起 HTTP 服务。
type serverShutdowner interface {
	Shutdown(ctx context.Context) error
}

// shutdown 按序收尾：停收新请求并等在途请求 → 排空后台写库任务 → 关闭数据库。
// 每一步失败都只记日志、不提前返回，否则前一步的失败会导致数据库连接永不关闭。
//
// 三个依赖一律经参数注入而非包级全局（bgtask.Default / models.Close）：顺序错了会让
// 在途写库任务撞「数据库已关闭」，注入后这条顺序才能被测试锁定，而不是只有注释在保护。
func shutdown(srv serverShutdowner, mgr *bgtask.Manager, closeDB func() error) {
	slog.Info("shutting down")

	srvCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(srvCtx); err != nil {
		slog.Error("http server shutdown", "error", err)
	}

	// 必须在 closeDB 之前：排空中的任务仍要写库。
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), bgtaskDrainTimeout)
	defer cancelDrain()
	if err := mgr.Shutdown(drainCtx); err != nil {
		slog.Error("drain background tasks", "error", err)
	}

	if err := closeDB(); err != nil {
		slog.Error("close database", "error", err)
	}
	slog.Info("shutdown complete")
}

// startBackgroundServices 启动健康检查与模型自动同步。
// 两者的 goroutine 由各自内部经 bgtask 登记，ctx 取消即其退出信号。
func startBackgroundServices(ctx context.Context) {
	service.GetHealthChecker().Start(ctx)

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
