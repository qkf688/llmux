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

	upstreamURL, err := resolveUpstreamURLFromBase(base, endpoint)
	if err != nil {
		return "", "", err
	}

	base["base_url"] = upstreamURL
	base["api_key"] = plainKey
	out, err := json.Marshal(base)
	if err != nil {
		return "", "", fmt.Errorf("channel: marshal dynamic config: %w", err)
	}
	return string(out), upstreamURL, nil
}

// resolveUpstreamURLFromBase 从已解析的 provider config map 与端点解析上游 URL
// （继承链单源：端点 URL 非空用端点 URL，否则用 provider config 的 base_url）。
func resolveUpstreamURLFromBase(base map[string]any, endpoint models.Endpoint) (string, error) {
	upstreamURL := strings.TrimSpace(endpoint.URL)
	if upstreamURL == "" {
		if s, ok := base["base_url"].(string); ok {
			upstreamURL = strings.TrimSpace(s)
		}
	}
	if upstreamURL == "" {
		return "", errors.New("channel: no upstream URL (endpoint URL empty and provider base_url empty)")
	}
	return upstreamURL, nil
}

// ResolveUpstreamURL 按继承链解析上游 URL（端点 URL 优先，否则 provider 的 base_url）。
//
// 为什么要从 BuildConfig 里独立出来：选路**失败**时 Select 不带 UpstreamURL 回来
// （凭据层失败只回传 Endpoint/Group，端点层/分组层失败连端点都没有），但失败留痕
// 仍要能说出「打的是哪家的哪个 URL」——那正是候选淘汰留痕要的归因信息。
//
// URL 解析不依赖明文 key（只看 endpoint.URL 与 provider config 的 base_url），
// 故可在「还没选出凭据」时独立调用。两者皆空则报错，与 BuildConfig 的失败口径一致。
func ResolveUpstreamURL(provider models.Provider, endpoint models.Endpoint) (string, error) {
	base := make(map[string]any)
	if strings.TrimSpace(provider.Config) != "" {
		if err := json.Unmarshal([]byte(provider.Config), &base); err != nil {
			return "", fmt.Errorf("channel: parse provider config: %w", err)
		}
	}
	return resolveUpstreamURLFromBase(base, endpoint)
}

// DecryptCredentialKey 解密凭据明文 key。cipher 未初始化（main 装配缺失）→ 明确
// 报错：与「明文落库路径禁止」同一原则——无解密能力就显式失败，不做降级。
// 解密成功但明文为空（含全空白）→ 同样报错（Stage 27）：空明文 = 配置缺失而非可用
// key，放行会带着空凭据头打上游挨 401，把配置问题伪装成上游鉴权失败；判据放在解密
// 后单点，Select/RetryCredential/ProbeGroup/NewClientFromSelection 四条装配路径自动
// 同享（诊断 + 生产聊天 + 健康检查全域不再选到空凭据）。
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
	if strings.TrimSpace(plain) == "" {
		return "", fmt.Errorf("channel: credential %d decrypts to empty key: %w (credential not configured)", c.ID, ErrCredentialNotConfigured)
	}
	return plain, nil
}
