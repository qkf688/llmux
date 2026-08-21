package streaming

import (
	"sync"

	"github.com/qkf688/llmux/consts"
)

// RealtimeChunkHandler 处理一条实时流 data 帧。
type RealtimeChunkHandler func(state *realtimeStreamState, data string) error

// realtimeRouteKey 是 (上游 body 形状, 客户端 body 形状) 二元组。
// 键是协议形状而非供应商 type：接一家 OpenAI 兼容的新上游不需要新增流式路由。
type realtimeRouteKey struct {
	upstreamFormat consts.WireFormat
	clientFormat   consts.WireFormat
}

var (
	realtimeRouteMu sync.RWMutex
	realtimeRoutes  = make(map[realtimeRouteKey]RealtimeChunkHandler)
)

// RegisterRealtimeRoute 注册 (upstreamFormat, clientFormat) → handler。重复注册 panic。
//
// 若 handler 有需要延后到流末才能写出的事件（如 OpenAI 的 usage 尾包晚于 finish 到达），
// 在 handler 内设置 state.finalize；transformStreamBodyRealtime 会在流末调用一次。
// 这是补在流生命周期上的实例级钩子，不是本注册表的静态映射，故不做对称的 finalizer 注册表
// （见 realtime.go 的 finalize 字段注释）。
func RegisterRealtimeRoute(upstreamFormat, clientFormat consts.WireFormat, h RealtimeChunkHandler) {
	if h == nil {
		panic("realtime route handler must not be nil")
	}
	key := realtimeRouteKey{upstreamFormat: upstreamFormat, clientFormat: clientFormat}
	realtimeRouteMu.Lock()
	defer realtimeRouteMu.Unlock()
	if _, exists := realtimeRoutes[key]; exists {
		panic("realtime route already registered: " + string(upstreamFormat) + " -> " + string(clientFormat))
	}
	realtimeRoutes[key] = h
}

func lookupRealtimeRoute(upstreamFormat, clientFormat consts.WireFormat) (RealtimeChunkHandler, bool) {
	realtimeRouteMu.RLock()
	defer realtimeRouteMu.RUnlock()
	h, ok := realtimeRoutes[realtimeRouteKey{upstreamFormat: upstreamFormat, clientFormat: clientFormat}]
	return h, ok
}
