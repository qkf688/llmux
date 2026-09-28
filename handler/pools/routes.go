package pools

import "github.com/gin-gonic/gin"

// Register 挂接号池域叶子路由：池 CRUD + 两个仍需池作用域的动作
// （批量导入 = 往这个池里灌 key；按凭据测试 = 测试模型是池级配置）。
//
// 凭据自身的读/写（列表、详情、启停、备注、明文、以后的凭据级设置）已迁到
// handler/credentials（`/api/credentials…`）——凭据是第一等资源，号池与分组只是
// 它的归属字段，按归属各写一份必然漂移。
func Register(rg gin.IRoutes) {
	rg.GET("/pools", GetPools)
	rg.POST("/pools", CreatePool)
	rg.PUT("/pools/:id", UpdatePool)
	rg.DELETE("/pools/:id", DeletePool)
	// batch 固定段优先于 :credId，避免 "batch" 被当成 credId 捕获
	rg.POST("/pools/:id/credentials/batch/import", BatchImportCredentials)
	rg.POST("/pools/:id/credentials/:credId/test", TestCredential)
}
