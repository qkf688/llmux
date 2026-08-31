package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
)

// BuildConfig 组装选中链路的动态 config（providers.New 的输入契约）：
//
//   - 以 Provider.Config 原 JSON 为基底 round-trip，保留全部既有字段
//     （anthropic 的 version/auth_type、代理等——未知字段不可丢）
//   - base_url 按继承链覆盖：端点 URL 非空用端点 URL（如 anthropic → …/claude/v1），
//     否则保留原 base_url（设计定案第 3 节字段级继承）
//   - api_key 恒写选中凭据明文：加密形态只存在于 credentials 表，
//     动态 config 是内存瞬态，不再回写任何明文兜底链
//
// 端点 URL 与 base_url 皆空 → 报错：BuildReq 会拼出残缺 URL，宁在上游之前失败。
// 返回 (config, upstreamURL)，upstreamURL 即继承链解析结果，供 SelectionResult / #13 日志。
func BuildConfig(provider models.Provider, endpoint models.Endpoint, plainKey string) (string, string, error) {
	base := make(map[string]any)
	if strings.TrimSpace(provider.Config) != "" {
		if err := json.Unmarshal([]byte(provider.Config), &base); err != nil {
			return "", "", fmt.Errorf("channel: parse provider config: %w", err)
		}
	}

	upstreamURL := strings.TrimSpace(endpoint.URL)
	if upstreamURL == "" {
		if s, ok := base["base_url"].(string); ok {
			upstreamURL = strings.TrimSpace(s)
		}
	}
	if upstreamURL == "" {
		return "", "", errors.New("channel: no upstream URL (endpoint URL empty and provider base_url empty)")
	}

	base["base_url"] = upstreamURL
	base["api_key"] = plainKey
	out, err := json.Marshal(base)
	if err != nil {
		return "", "", fmt.Errorf("channel: marshal dynamic config: %w", err)
	}
	return string(out), upstreamURL, nil
}

// DecryptCredentialKey 解密凭据明文 key。cipher 未初始化（main 装配缺失）→ 明确
// 报错：与「明文落库路径禁止」同一原则——无解密能力就显式失败，不做降级。
// 供选路（Select/RetryCredential）与探活（credprobe）共用，禁止调用方各自解密。
func DecryptCredentialKey(c models.Credential) (string, error) {
	cipher := credentialcrypto.Default()
	if cipher == nil {
		return "", errors.New("channel: credential cipher not initialized")
	}
	plain, err := cipher.Decrypt(c.Key)
	if err != nil {
		return "", fmt.Errorf("channel: decrypt credential %d: %w", c.ID, err)
	}
	return plain, nil
}
