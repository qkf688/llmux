package streaming

import "sync"

// RealtimeChunkHandler 处理一条实时流 data 帧。
type RealtimeChunkHandler func(state *realtimeStreamState, data string) error

type realtimeRouteKey struct {
	providerType string
	clientType   string
}

var (
	realtimeRouteMu sync.RWMutex
	realtimeRoutes  = make(map[realtimeRouteKey]RealtimeChunkHandler)
)

// RegisterRealtimeRoute 注册 (providerType, clientType) → handler。重复注册 panic。
//
// 若 handler 有需要延后到流末才能写出的事件（如 OpenAI 的 usage 尾包晚于 finish 到达），
// 在 handler 内设置 state.finalize；transformStreamBodyRealtime 会在流末调用一次。
// 这是补在流生命周期上的实例级钩子，不是本注册表的静态映射，故不做对称的 finalizer 注册表
// （见 realtime.go 的 finalize 字段注释）。
func RegisterRealtimeRoute(providerType, clientType string, h RealtimeChunkHandler) {
	if h == nil {
		panic("realtime route handler must not be nil")
	}
	key := realtimeRouteKey{providerType: providerType, clientType: clientType}
	realtimeRouteMu.Lock()
	defer realtimeRouteMu.Unlock()
	if _, exists := realtimeRoutes[key]; exists {
		panic("realtime route already registered: " + providerType + " -> " + clientType)
	}
	realtimeRoutes[key] = h
}

func lookupRealtimeRoute(providerType, clientType string) (RealtimeChunkHandler, bool) {
	realtimeRouteMu.RLock()
	defer realtimeRouteMu.RUnlock()
	h, ok := realtimeRoutes[realtimeRouteKey{providerType: providerType, clientType: clientType}]
	return h, ok
}
