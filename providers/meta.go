package providers

import (
	"sync"

	"github.com/qkf688/llmux/consts"
)

// Metadata 描述某类 provider 的静态元数据（模板、探针请求体等）。
// 由各 provider 在 init() 中 RegisterMetadata，供 handler/healthcheck 查询，避免 type switch。
type Metadata struct {
	Type string
	// WireFormat 声明本 provider 的上游 API 用哪种 body 形状收发。
	// 这是 provider type（开集）与协议形状（闭集）之间**唯一**的映射来源：
	// service/transform 只认 WireFormat，因此加一家 OpenAI 兼容上游只需在这里声明
	// consts.FormatOpenAIChat，转换层零改动（OCP）。
	WireFormat      consts.WireFormat
	ConfigTemplate  string // OCP-6：配置 JSON 模板
	TestBody        []byte // OCP-7：连通性测试体；未知 type 由调用方决定 error 或回退
	StructuredBody  []byte // OCP-7：结构化输出测试体
	HealthCheckBody []byte // OCP-8：健康检查体（可与 TestBody 不同；未知 type 回退 openai）
}

var (
	metadataMu     sync.RWMutex
	metadataByType = make(map[string]Metadata)
	// metadataOrder 保持稳定展示顺序：openai → openai-res → anthropic
	metadataOrder []string
)

// RegisterMetadata 注册 provider 元数据。重复 type 会 panic。
// 未声明 WireFormat 同样 panic：漏声明会让转换层查不到形状，若在此放过，
// 错误会推迟到真实请求时才以「上游 400」的形态出现，离根因很远。
func RegisterMetadata(m Metadata) {
	if m.Type == "" {
		panic("provider metadata type must not be empty")
	}
	if m.WireFormat == "" {
		panic("provider metadata wire format must not be empty: " + m.Type)
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

// WireFormatOf 给出某个 provider type 的上游 body 形状。
// 未注册的 type 返回 false 而不兜底——兜底会让「新 provider 忘了注册」退化成按 OpenAI 形状构建请求。
func WireFormatOf(providerType string) (consts.WireFormat, bool) {
	m, ok := MetadataOf(providerType)
	if !ok {
		return "", false
	}
	return m.WireFormat, true
}

// AllMetadata 按稳定展示顺序返回全部元数据：openai → openai-res → anthropic，其余按注册顺序追加。
func AllMetadata() []Metadata {
	metadataMu.RLock()
	defer metadataMu.RUnlock()

	// 固定核心顺序（与旧 template 切片一致），不依赖 init 文件加载顺序。
	preferred := []string{TypeOpenAI, TypeOpenAIRes, TypeAnthropic}
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
