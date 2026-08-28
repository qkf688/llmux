package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/associations"
	"github.com/qkf688/llmux/handler/auth"
	"github.com/qkf688/llmux/handler/autoassoc"
	"github.com/qkf688/llmux/handler/database"
	"github.com/qkf688/llmux/handler/importexport"
	"github.com/qkf688/llmux/handler/logs"
	"github.com/qkf688/llmux/handler/modelapi"
	"github.com/qkf688/llmux/handler/pools"
	"github.com/qkf688/llmux/handler/providerapi"
	"github.com/qkf688/llmux/handler/settings"
	"github.com/qkf688/llmux/handler/testapi"
	"github.com/qkf688/llmux/handler/virtualmodels"
	"github.com/qkf688/llmux/middleware"
)

// Deps 路由注册依赖（鉴权密钥等）。
// 新增 API：在对应子包 Register 追加，禁止改 main。
type Deps struct {
	JWTSecret string
}

// RegisterAll 唯一业务路由注册入口：建组、鉴权、挂接各域路由。
// 注册顺序以历史 main.go 现序为准；method/path/handler 与现网 1:1。
// Group / Use 仅在此函数；子包 Register 只挂叶子路由。
func RegisterAll(r *gin.Engine, d Deps) {
	v1 := r.Group("/v1")
	RegisterV1(v1, d)

	api := r.Group("/api")

	// 登录路由不挂鉴权中间件
	auth.RegisterLogin(api, d.JWTSecret)

	// 以下路由需要 JWT 鉴权
	api.Use(middleware.AuthJWT(d.JWTSecret, Repos().User))

	// 以下调用顺序对齐历史 main.go 现序（跨包交错处用拆分 Register）
	RegisterMetrics(api)

	providerapi.RegisterHead(api)     // template / list / models/:id
	testapi.RegisterProviderTest(api) // POST /providers/:id/test
	providerapi.RegisterCRUD(api)     // create / update / delete / associations

	modelapi.Register(api)

	associations.Register(api)
	autoassoc.RegisterAssociate(api)
	providerapi.RegisterBlacklist(api)
	autoassoc.RegisterClean(api)

	logs.Register(api)
	database.RegisterVacuum(api)

	settings.RegisterSystemConfig(api)
	database.RegisterDatabaseStats(api)
	importexport.RegisterExportConfig(api)
	database.RegisterExportDatabase(api)
	importexport.RegisterImportConfig(api)

	settings.Register(api)

	RegisterHealthCheck(api)

	testapi.Register(api)

	RegisterModelSync(api)

	virtualmodels.Register(api)

	pools.Register(api)

	// auth 受保护路由（me / rotate api key / change password）
	auth.Register(api)
}
