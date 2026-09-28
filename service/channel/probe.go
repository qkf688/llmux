package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// NewClientFromSelection 用「端点 + 凭据」这条已选定链路装配打上游的客户端，是所有
// **非 chat 流量**（模型同步 / 管理端上游浏览 / 凭据探活 / 健康检测 / testapi 连通性
// 测试；#24 按凭据测试同样走这条）唯一的客户端装配点：
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
	client, _, err := newClientFromPlainKey(provider, endpoint, plainKey)
	return client, err
}

// newClientFromPlainKey 装配链路的后半段（动态 config → 剥 custom_models → 按端点协议
// 派生类型 → providers.New），并回传继承链解析出的实际出站 URL。
//
// 单独成函数是为了让 ProbeGroup 能「解密一次、既喂给 config 又据此算掩码」：不出现
// 同一把 key 解两遍的两条解密路径，也不必为拿 URL 再跑一次 BuildConfig。
func newClientFromPlainKey(provider models.Provider, endpoint models.Endpoint, plainKey string) (providers.Provider, string, error) {
	cfg, upstreamURL, err := BuildConfig(provider, endpoint, plainKey)
	if err != nil {
		return nil, "", err
	}
	cfg, err = DropCustomModels(cfg)
	if err != nil {
		return nil, "", err
	}
	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		return nil, "", fmt.Errorf("channel: unknown endpoint protocol %q (endpoint %d)", endpoint.Protocol, endpoint.ID)
	}
	client, err := providers.New(providerType, cfg, provider.Proxy)
	if err != nil {
		return nil, "", fmt.Errorf("channel: create upstream client for provider %d: %w", provider.ID, err)
	}
	return client, upstreamURL, nil
}

// GroupProbe 一次「按组打上游」的装配产物：客户端 + 实际用到的链路标识。
//
// 为什么要带标识而不是只回客户端：管理端诊断（testapi 四个测试端点、#24 按凭据测试）
// 的失败响应只显示到分组一层（C2 定案：✕ 纯显红、不铺开错误详情），「到底用的哪把
// key」只剩服务端日志这一条线索，所以掩码必须在装配现场就取到。
type GroupProbe struct {
	Client       providers.Provider
	Group        models.KeyGroup
	CredentialID uint
	KeyMasked    string
	UpstreamURL  string
	EndpointID   uint
	Protocol     string // 端点协议（openai / responses / anthropic），日志里区分方言
}

// ProbeGroup 同 NewClientForGroup（组内首个可用凭据），但额外回传链路标识，并把明文
// key 收在函数内部——只对外暴露掩码。掩码用喂进 config 的同一份解密结果算，避免
// 「调用方自己再解一次」造成两条解密路径。
func ProbeGroup(snapshot *Snapshot, group models.KeyGroup, endpoint models.Endpoint, now time.Time) (GroupProbe, error) {
	cred, err := FirstUsableCredential(snapshot, group.ID, now)
	if err != nil {
		return GroupProbe{}, err
	}
	plainKey, err := DecryptCredentialKey(cred)
	if err != nil {
		// 空明文（解密成功但 key 为空）= 配置缺失，归凭据层失败可跳组（Stage 27）；
		// 其余解密失败（密文损坏）保持全局性失败直接上抛——换下一个组必然同样失败。
		if errors.Is(err, ErrCredentialNotConfigured) {
			return GroupProbe{}, fmt.Errorf("%w: 分组 %q 的凭据未配置 API Key（明文为空）", ErrNoCredentialAvailable, group.Name)
		}
		return GroupProbe{}, err
	}
	client, upstreamURL, err := newClientFromPlainKey(snapshot.Provider, endpoint, plainKey)
	if err != nil {
		return GroupProbe{}, err
	}
	return GroupProbe{
		Client:       client,
		Group:        group,
		CredentialID: cred.ID,
		KeyMasked:    credentialcrypto.MaskKey(plainKey),
		UpstreamURL:  upstreamURL,
		EndpointID:   endpoint.ID,
		Protocol:     endpoint.Protocol,
	}, nil
}

// NewClientForGroup 装配「用某个分组的凭据打上游」的客户端：组内首个可用凭据
// （FirstUsableCredential）→ NewClientFromSelection。逐组尝试的诊断类流量（模型同步、
// 管理端上游浏览、健康检查）共用的最小单元——这两步在任何调用方写出来都一样，留成
// 各包副本就是漂移入口（改一处忘一处 → 有的路径推进轮询、有的不推进）。
//
// 需要知道「用的是哪条凭据」时改用 ProbeGroup。
//
// 组内无可用凭据 → 透传 FirstUsableCredential 包装的 ErrNoCredentialAvailable
// （errors.Is 可判），调用方按「跳过该组、继续其余组」处理。
func NewClientForGroup(snapshot *Snapshot, group models.KeyGroup, endpoint models.Endpoint, now time.Time) (providers.Provider, error) {
	probe, err := ProbeGroup(snapshot, group, endpoint, now)
	if err != nil {
		return nil, err
	}
	return probe.Client, nil
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
	if _, ok := parsed[common.CustomModelsConfigKey]; !ok {
		return config, nil
	}
	delete(parsed, common.CustomModelsConfigKey)
	out, err := json.Marshal(parsed)
	if err != nil {
		return "", fmt.Errorf("channel: marshal config after dropping custom_models: %w", err)
	}
	return string(out), nil
}
