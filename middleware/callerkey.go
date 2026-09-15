package middleware

import "github.com/gin-gonic/gin"

// ContextKeyCallerKey 是 /v1 鉴权通过后，调用 key 快照在 gin context 中的键。
const ContextKeyCallerKey = "callerKey"

// CallerKey 是通过鉴权的 api_keys 行的只读快照（值在请求入口定格：启停/删除
// 不打断在途流，鉴权只在入口执行一次——多 api-key 方案 §2 语义）。
// #4-3（日志归因）与 #6（key 级出站头）共用本形状，一次定形：#6 只加 Headers
// 字段，不改 context 键与既有读方（OCP）。
type CallerKey struct {
	ID   uint
	Name string
	// Headers 是该 key 配置的出站头只读快照（#6）——定入 context 即定格，
	// 下游（ReqMeta → BuildHeaders）只读不写。浅拷贝自 DB 行，与本结构「值在
	// 请求入口定格」的既有不变量同构（鉴权不依赖加密装配态的语义不变）。
	Headers map[string]string
}

// CurrentCallerKey 从 gin context 取调用 key 快照。
// 仅 AuthAPIKey 放行的请求非 nil；JWT 管理端路径恒为 nil（那是登录态，没有调用 key）。
func CurrentCallerKey(c *gin.Context) *CallerKey {
	v, ok := c.Get(ContextKeyCallerKey)
	if !ok {
		return nil
	}
	caller, _ := v.(*CallerKey)
	return caller
}
