package chat

import (
	"fmt"
	"sync"
)

// Registry 按 style 注册 Beforer 与 Processer。
// 新增协议端点时，只需调用 RegisterBeforer / RegisterProcesser 并添加路由即可。
// 所有方法并发安全（sync.RWMutex 保护）；init 期注册无竞争，运行期注册亦线程安全，但重复注册仍按规范 panic。
type Registry struct {
	mu         sync.RWMutex
	beforeers  map[string]Beforer
	processers map[string]Processer
}

// NewRegistry 创建一个新的 Registry。
func NewRegistry() *Registry {
	return &Registry{
		beforeers:  make(map[string]Beforer),
		processers: make(map[string]Processer),
	}
}

// RegisterBeforer 注册一个指定 style 的请求预处理器。
// 重复注册同一 style 会 panic。
func (r *Registry) RegisterBeforer(style string, b Beforer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.beforeers[style]; exists {
		panic(fmt.Sprintf("beforer for style %q already registered", style))
	}
	r.beforeers[style] = b
}

// RegisterProcesser 注册一个指定 style 的响应后处理器。
// 重复注册同一 style 会 panic。
func (r *Registry) RegisterProcesser(style string, p Processer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.processers[style]; exists {
		panic(fmt.Sprintf("processer for style %q already registered", style))
	}
	r.processers[style] = p
}

// GetBeforer 获取指定 style 的 Beforer。
func (r *Registry) GetBeforer(style string) (Beforer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if b, ok := r.beforeers[style]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("no beforer registered for style %q", style)
}

// GetProcesser 获取指定 style 的 Processer。
func (r *Registry) GetProcesser(style string) (Processer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.processers[style]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("no processer registered for style %q", style)
}

// defaultRegistry 是包级默认注册表，在 init() 阶段注册三种内置协议。
// 加锁后并发安全（导出的 Register*/Get* 均线程安全）；重复注册仍按规范 panic。
var defaultRegistry = NewRegistry()

// RegisterBeforer 在默认注册表中注册 Beforer。
func RegisterBeforer(style string, b Beforer) {
	defaultRegistry.RegisterBeforer(style, b)
}

// RegisterProcesser 在默认注册表中注册 Processer。
func RegisterProcesser(style string, p Processer) {
	defaultRegistry.RegisterProcesser(style, p)
}

// GetBeforer 从默认注册表获取 Beforer。
func GetBeforer(style string) (Beforer, error) {
	return defaultRegistry.GetBeforer(style)
}

// GetProcesser 从默认注册表获取 Processer。
func GetProcesser(style string) (Processer, error) {
	return defaultRegistry.GetProcesser(style)
}
