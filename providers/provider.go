package providers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"` // 使用 int64 存储 Unix 时间戳
	OwnedBy string `json:"owned_by"`
}

type Provider interface {
	// BuildReq 构造上游请求。约定（4b）：实现会以传入 header 为**基底就地修改**
	// （覆写 Content-Type / anthropic-beta、按认证方式 Set 凭据头并 Del 对侧）——
	// 调用方传出的 map 在 BuildReq 之后不得再复用（要复用先 Clone，参见
	// credprobe.stream_probe / completion_probe 的姿势）。
	BuildReq(ctx context.Context, header http.Header, model string, rawData []byte) (*http.Request, error)
	Models(ctx context.Context) ([]Model, error)
	GetProxy() string
}

// OpenAICompat 标记 OpenAI 兼容协议族（chat/completions 或 responses 风格配置导出）。
// *OpenAI / *OpenAIRes 实现；*Anthropic 不实现。供 React 测试等走统一能力接口，避免具体类型断言。
type OpenAICompat interface {
	Provider
	OpenAICompatBaseURL() string
	OpenAICompatAPIKey() string
	// OpenAICompatAuthHeaders 返回按本供应商认证方言（config 键 openai_auth_type）写好的
	// 凭据头（复用 applyAuthHeader 单源执法：选定方式 Set、对侧 Del、空值/未知值回退 bearer）。
	// 供不经 BuildReq 的诊断路径（ReAct 测试走 openai-go SDK 直发）直接使用——方言判断
	// 只存在 providers 一处，诊断与真实转发永不分叉；消费方只搬运头，禁止自行 switch 方言。
	OpenAICompatAuthHeaders() http.Header
}

// BetaFeatureCapable 暴露「本供应商是否启用了某个 beta 特性」的查询能力。
// 上层做协议合法性收敛时需要它：同一条硬约束在特定 beta 开启后可能整体失效
// （Anthropic interleaved thinking 下 thinking.budget_tokens 允许超过 max_tokens），
// 不看 beta 就收敛等于把合法请求静默改写。
// 目前仅 *Anthropic 实现；调用方**必须**断言本接口而非具体类型（OCP：新 type 想参与只需实现它）。
type BetaFeatureCapable interface {
	Provider
	HasBetaFeature(name string) bool
}

// CredentialCapable 暴露「当前装配的凭据明文」读取能力（OCP：调用方断言本接口而非具体类型）。
// 消费方限诊断前置守卫（testapi ReAct runner 的空密钥判定：明文为空 = 配置错误，
// 本地失败、零上游请求）；明文禁止进日志与响应（掩码口径见 testapi buildLink：只
// key_masked 进 slog）。目前 *Anthropic 实现；openai 系经 OpenAICompatAPIKey 读取，
// 不需要重复实现本接口。
type CredentialCapable interface {
	Provider
	CredentialPlaintext() string
}

func buildCustomModels(custom []string) []Model {
	now := time.Now().Unix()
	models := make([]Model, 0, len(custom))
	for _, model := range custom {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		models = append(models, Model{
			ID:      trimmed,
			Object:  "custom",
			Created: now,
			OwnedBy: "custom",
		})
	}
	return models
}

// Factory 根据 JSON 配置与外部代理地址构造 Provider。
// 实现内部负责反序列化，并在外部 proxy 非空时覆盖配置中的 proxy。
type Factory func(config, proxy string) (Provider, error)

var factories = make(map[string]Factory)

// Register 注册一个 Provider 构造器。
// 重复注册同一类型会 panic，避免运行期覆盖导致行为不可预期。
func Register(providerType string, factory Factory) {
	if _, exists := factories[providerType]; exists {
		panic("provider factory already registered: " + providerType)
	}
	factories[providerType] = factory
}

// New 根据类型从注册表创建 Provider 实例。
// proxy 参数优先级高于 config 内的代理字段，避免双处配置导致遗漏。
func New(providerType, providerConfig, proxy string) (Provider, error) {
	factory, ok := factories[providerType]
	if !ok {
		return nil, errors.New("unknown provider")
	}
	return factory(providerConfig, proxy)
}
