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