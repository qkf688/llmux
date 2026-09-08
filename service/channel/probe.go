package channel

import (
	"encoding/json"
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// NewClientFromSelection 用「端点 + 凭据」这条已选定链路装配打上游的客户端，是所有
// **非 chat 流量**（模型同步 / 管理端上游浏览 / 凭据探活 / #19 testapi / #21 healthcheck
// / #24 按凭据测试）唯一的客户端装配点：
//
//	解密凭据 → BuildConfig 动态 config → 剥 custom_models → 按端点协议派生注册类型 → providers.New
//
// 为什么必须有这一份：S6 起 Provider.Config 不再承载 api_key（密钥只存在于 credentials
// 表、运行时在此注入明文），任何绕过本函数、直用 Provider.Config 打上游的路径都恒被
// 上游判 401。
//
// 与 chat 主链路刻意不同：chat 的 config 来自 Select/RetryCredential（含分组白名单与
// 权重轮询语义），且它只发补全请求、不发 Models()，因此不需要剥 custom_models。
func NewClientFromSelection(provider models.Provider, endpoint models.Endpoint, cred models.Credential) (providers.Provider, error) {
	plainKey, err := DecryptCredentialKey(cred)
	if err != nil {
		return nil, err
	}
	cfg, _, err := BuildConfig(provider, endpoint, plainKey)
	if err != nil {
		return nil, err
	}
	cfg, err = DropCustomModels(cfg)
	if err != nil {
		return nil, err
	}
	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		return nil, fmt.Errorf("channel: unknown endpoint protocol %q (endpoint %d)", endpoint.Protocol, endpoint.ID)
	}
	client, err := providers.New(providerType, cfg, provider.Proxy)
	if err != nil {
		return nil, fmt.Errorf("channel: create upstream client for provider %d: %w", provider.ID, err)
	}
	return client, nil
}

// DropCustomModels 从动态 config JSON 移除 custom_models，使 Providers.Models() 真的
// 打上游。不剥则 openai/anthropic 的 Models() 本地短路返回自定义列表：管理端浏览拿不到
// 上游授权全集，探活更会把失效 key 判成成功并误恢复 active。
//
// config 为空或不含该键时**原样返回**：避免无谓的 unmarshal/marshal 往返改写键序，
// 让「剥与不剥」在字节层面可预期（测试断言与日志据此稳定）。
func DropCustomModels(config string) (string, error) {
	if config == "" {
		return config, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", fmt.Errorf("channel: parse config to drop custom_models: %w", err)
	}
	if _, ok := parsed["custom_models"]; !ok {
		return config, nil
	}
	delete(parsed, "custom_models")
	out, err := json.Marshal(parsed)
	if err != nil {
		return "", fmt.Errorf("channel: marshal config after dropping custom_models: %w", err)
	}
	return string(out), nil
}
