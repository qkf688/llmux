package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
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
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service"
	"github.com/qkf688/llmux/service/auth"
	"github.com/qkf688/llmux/service/credwrite"
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
	boot, err := auth.BootstrapAdmin(ctx, repository.Default(), adminPassword)
	if err != nil {
		slog.Error("bootstrap admin", "error", err)
		os.Exit(1)
	}
	auth.LogBootstrap(boot, fromEnv)

	// 存量 chat_logs 头快照一次性脱敏（Stage 16 立项，Stage 26 扩两列）：破坏性清洗
	// 只改值不删行、幂等，只动 request_headers / response_headers 非空的行；坏 JSON 行
	// 在 repo 内逐行 Warn 跳过。extraKeys = 现存配置自配头名全集（关联级 customer_headers
	// + key 级自定义头键名并集）——已删配置的历史头名洗不到（文档化局限）。整体失败不
	// 阻断启动（库已在 models.Init 验证，此处失败概率极低），但明文凭据残留是安全面，
	// 必须 Error 级留痕——清洗的使命就是消灭存量明文。
	extraHeaderKeys := collectConfiguredHeaderKeys(ctx)
	if sanitized, err := repository.Default().ChatLog.SanitizeLegacyLogSnapshots(ctx, extraHeaderKeys); err != nil {
		slog.Error("sanitize legacy chat log header snapshots", "error", err)
	} else if sanitized > 0 {
		slog.Info("legacy chat log header snapshots sanitized", "rows", sanitized)
	}

	// 存量空明文凭据一次性清洗（Stage 27）：删行而非停用——解密为空的凭据已无法通过
	// 选路（DecryptCredentialKey 空明文守卫），留着只会在后台显示 active 误导管理员。
	// 同样必须在 startBackgroundServices 之前（无凭据并发写者）；失败不阻断启动但 Error
	// 留痕（清洗使命是消灭配置误导面）。
	if deleted, err := repository.Default().Credential.DeleteLegacyEmptyKeyCredentials(ctx); err != nil {
		slog.Error("sanitize legacy empty-plain credentials", "error", err)
	} else if deleted > 0 {
		slog.Info("legacy empty-plain credentials deleted", "rows", deleted)
	}

	// 引擎装配先于后台服务启动：信任边界（TRUSTED_PROXIES）配置错误要 fail fast，
	// 此时还没有可能在写库的后台任务，可安全 os.Exit；挪到之后则必须走 cancel 关闭路径。
	router, err := newRouter(handler.Deps{JWTSecret: jwtSecret}, parseTrustedProxies(os.Getenv("TRUSTED_PROXIES")))
	if err != nil {
		slog.Error("assemble router", "error", err)
		os.Exit(1)
	}

	startBackgroundServices(ctx)

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
	shutdown(srv, bgtask.Default(), models.Close, credwrite.Stop)
}

// newRouter 组装 HTTP 引擎。装配从 main 的内联代码里抽成函数（与 shutdown 的依赖
// 注入同构）——只有可被测试还原的装配，其顺序才可能被锁定，此前这条顺序只有注释在保护。
//
// 中间件装配序即请求路径顺序，契约见 framework-conventions §1，由
// TestGateRouterMiddlewareOrder（顺序）+ TestRouterAssembly_*（后果）双重锁定：
//
//	RequestID 最先 —— 401/404/panic 也要有号可对，错误/访问/上游三处日志共用同一号；
//	AccessLog     —— 读 RequestID 回写的入站头快照，故必须在其后；
//	Recovery      —— 替代 gin.Recovery：panic 型 500 同样走 httpresp 收口并带 rid（Stage 3）；
//	gzip          —— 压缩排除 /v1/（SSE 流不压）。
//
// 全部 router.Use 必须先于 handler.RegisterAll：gin 为已注册路由快照 handler 链，
// Use 写在 RegisterAll 之后对那些路由根本不生效（本函数的顺序即为约束本身）。
//
// trustedProxies 是引擎信任边界（TRUSTED_PROXIES 解析结果，见 parseTrustedProxies）：
// 只有这些网段内代理的 X-Forwarded-For 会被采信为 ClientIP，非法值在此 fail fast
// 返回错误（信任边界配置错误不得静默回退）。本项目口径是「未配置 = 全关」，由调用方
// 经 parseTrustedProxies 显式传非 nil 空切片表达（nil 与空切片在 gin v1.10 同为全关）。
func newRouter(deps handler.Deps, trustedProxies []string) (*gin.Engine, error) {
	router := gin.New()

	// 信任边界装配在中间件之前：ClientIP 的判定口径对全链一致。显式调用即覆盖
	// gin 默认的 0.0.0.0/0 + ::/0（信任全部——只有从不调用 SetTrustedProxies 才
	// 放行伪造 XFF）；nil 与空切片同为全关（gin v1.10：prepareTrustedCIDRs 对 nil
	// 返回空集、isTrustedProxy 对空集判 false）。
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies %v: %w", trustedProxies, err)
	}
	router.Use(middleware.RequestID())
	router.Use(middleware.AccessLog())
	router.Use(middleware.Recovery())

	router.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))

	handler.RegisterAll(router, deps)

	setwebui(router)
	return router, nil
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

// shutdown 按序收尾：停收新请求并等在途请求 → Flush/停凭据写队列 → 排空后台写库任务 → 关闭数据库。
// 每一步失败都只记日志、不提前返回，否则前一步的失败会导致数据库连接永不关闭。
//
// 依赖一律经参数注入而非包级全局：顺序错了会让在途写库撞「数据库已关闭」，
// 注入后这条顺序才能被测试锁定。stopCredWrite 可为 nil（单测不启队列时）。
func shutdown(srv serverShutdowner, mgr *bgtask.Manager, closeDB func() error, stopCredWrite func(context.Context) error) {
	slog.Info("shutting down")

	srvCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(srvCtx); err != nil {
		slog.Error("http server shutdown", "error", err)
	}

	// 凭据写 worker 长驻：先 Flush 再 requestStop，使 bgtask.Shutdown 能等到它退出，
	// 且关库前队列落库完成（#6-4-1）。
	if stopCredWrite != nil {
		flushCtx, cancelFlush := context.WithTimeout(context.Background(), bgtaskDrainTimeout)
		if err := stopCredWrite(flushCtx); err != nil {
			slog.Error("stop credential write queue", "error", err)
		}
		cancelFlush()
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

// startBackgroundServices 启动健康检查、模型自动同步与凭据写队列 worker。
// healthcheck/modelsync 的 goroutine 由各自内部经 bgtask 登记，ctx 取消即退出信号；
// credwrite worker 由 Stop（在 shutdown 内）显式停收并排空。
func startBackgroundServices(ctx context.Context) {
	credwrite.Start()

	service.GetHealthChecker().Start(ctx)

	syncService := service.NewModelSyncService(models.DB, nil)
	syncService.StartAutoSync(ctx)
}

// collectConfiguredHeaderKeys 现存配置自配头名全集（Stage 26）：关联级 customer_headers
// 与 key 级自定义头的键名并集，供存量请求头快照清洗动态并入掩码判据。读库失败返回
// nil（清洗按纯固定名单执行并 slog.Warn——头名收集失败不得阻断启动，局限与空集
// 同形，注明即可）。
func collectConfiguredHeaderKeys(ctx context.Context) []string {
	repos := repository.Default()
	seen := map[string]struct{}{}
	var keys []string
	add := func(m map[string]string) {
		for k := range m {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
	}
	assocs, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		slog.Warn("collect configured header keys: list associations", "error", err)
		return nil
	}
	for _, a := range assocs {
		add(a.CustomerHeaders)
	}
	apiKeys, err := repos.ApiKey.List(ctx)
	if err != nil {
		slog.Warn("collect configured header keys: list api keys", "error", err)
		return nil
	}
	for _, k := range apiKeys {
		add(k.Headers)
	}
	return keys
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
