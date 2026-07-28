package handler

import (
	"github.com/atopos31/llmio/handler/associations"
	"github.com/atopos31/llmio/handler/autoassoc"
	"github.com/atopos31/llmio/handler/database"
	"github.com/atopos31/llmio/handler/importexport"
	"github.com/atopos31/llmio/handler/logs"
	"github.com/atopos31/llmio/handler/modelapi"
	"github.com/atopos31/llmio/handler/providerapi"
	"github.com/atopos31/llmio/handler/settings"
	"github.com/atopos31/llmio/handler/testapi"
	"github.com/atopos31/llmio/handler/virtualmodels"
	"github.com/atopos31/llmio/middleware"
	"github.com/gin-gonic/gin"
)

// Deps 路由注册依赖（鉴权令牌等）。
// 新增 API：在对应子包 Register 追加，禁止改 main。
type Deps struct {
	Token string
}

// RegisterAll 唯一业务路由注册入口：建组、鉴权、挂接各域路由。
// 注册顺序以历史 main.go 现序为准；method/path/handler 与现网 1:1。
// Group / Use 仅在此函数；子包 Register 只挂叶子路由。
func RegisterAll(r *gin.Engine, d Deps) {
	v1 := r.Group("/v1")
	RegisterV1(v1, d)

	api := r.Group("/api")
	api.Use(middleware.Auth(d.Token))

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
}
