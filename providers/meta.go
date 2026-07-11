package providers

import "sync"

// Metadata 描述某类 provider 的静态元数据（模板、探针请求体等）。
// 由各 provider 在 init() 中 RegisterMetadata，供 handler/healthcheck 查询，避免 type switch。
type Metadata struct {
	Type           string
	ConfigTemplate string // OCP-6：配置 JSON 模板
	TestBody       []byte // OCP-7：连通性测试体；未知 type 由调用方决定 error 或回退
	StructuredBody []byte // OCP-7：结构化输出测试体
	HealthCheckBody []byte // OCP-8：健康检查体（可与 TestBody 不同；未知 type 回退 openai）
}

var (
	metadataMu   sync.RWMutex
	metadataByType = make(map[string]Metadata)
	// metadataOrder 保持稳定展示顺序：openai → openai-res → anthropic
	metadataOrder []string
)

// RegisterMetadata 注册 provider 元数据。重复 type 会 panic。
func RegisterMetadata(m Metadata) {
	if m.Type == "" {
		panic("provider metadata type must not be empty")
	}
	metadataMu.Lock()
	defer metadataMu.Unlock()
	if _, exists := metadataByType[m.Type]; exists {
		panic("provider metadata already registered: " + m.Type)
	}
	metadataByType[m.Type] = m
	metadataOrder = append(metadataOrder, m.Type)
}

// MetadataOf 按 type 查询元数据。
func MetadataOf(providerType string) (Metadata, bool) {
	metadataMu.RLock()
	defer metadataMu.RUnlock()
	m, ok := metadataByType[providerType]
	return m, ok
}

// AllMetadata 按稳定展示顺序返回全部元数据：openai → openai-res → anthropic，其余按注册顺序追加。
func AllMetadata() []Metadata {
	metadataMu.RLock()
	defer metadataMu.RUnlock()

	// 固定核心顺序（与旧 template 切片一致），不依赖 init 文件加载顺序。
	preferred := []string{"openai", "openai-res", "anthropic"}
	seen := make(map[string]bool, len(preferred))
	out := make([]Metadata, 0, len(metadataByType))
	for _, t := range preferred {
		if m, ok := metadataByType[t]; ok {
			out = append(out, m)
			seen[t] = true
		}
	}
	for _, t := range metadataOrder {
		if seen[t] {
			continue
		}
		out = append(out, metadataByType[t])
	}
	return out
}