package modelsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/channel"
)

// groupUpstreamClient 装配「用某分组凭据打上游」的客户端：组内首个可用凭据 → 解密 →
// 端点 URL / base_url 继承链组装动态 config → 剥 custom_models（否则 openai/anthropic
// 的 Models() 本地短路返回自定义列表，压根不打上游）→ 按端点协议派生 provider 注册类型。
//
// 同步（syncKeyGroup）与管理端只读浏览（ListUpstreamModels）共用本函数：打上游的链路
// 只允许一份实现。此前管理端弹窗直接用 Provider.Config 组装客户端，而 S6 起 config 不再
// 承载 api_key（凭据只存在于 credentials 表、运行时由 channel.BuildConfig 注入），
// 于是号池/内联凭据的供应商恒被上游判 401「You didn't provide an API key」。
//
// 新建 Selector 而非 channel.DefaultSelector()：同步/浏览不参与生产轮询的压力分摊，
// 恒取组内首个可用凭据（可复现，且不推进 chat 链路的轮询指针）。
// 组内无可用凭据返回 channel.ErrNoCredentialAvailable（errors.Is 可判），调用方按跳过处理。
func groupUpstreamClient(
	provider models.Provider,
	snapshot *channel.Snapshot,
	group models.KeyGroup,
	endpoint models.Endpoint,
	now time.Time,
) (providers.Provider, error) {
	cred, err := channel.SelectCredential(&channel.Selector{}, snapshot, group.ID, now)
	if err != nil {
		return nil, err
	}
	plainKey, err := channel.DecryptCredentialKey(cred)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	cfg, _, err := channel.BuildConfig(provider, endpoint, plainKey)
	if err != nil {
		return nil, fmt.Errorf("build config: %w", err)
	}
	cfg, err = dropCustomModels(cfg)
	if err != nil {
		return nil, fmt.Errorf("strip custom models: %w", err)
	}
	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		return nil, fmt.Errorf("unknown endpoint protocol %q", endpoint.Protocol)
	}
	client, err := providers.New(providerType, cfg, provider.Proxy)
	if err != nil {
		return nil, fmt.Errorf("create provider client: %w", err)
	}
	return client, nil
}

// ListUpstreamModels 只读拉取供应商的上游模型列表（管理端「上游模型」弹窗数据源）。
//
// 与 SyncProviderModels 共用端点/凭据选择（selectSyncEndpoint + groupUpstreamClient），
// 差别只在**不落库**：不写 KeyGroup.Models、不产 ModelSyncLog、不触发同步后动作，
// 也**不应用过滤规则**——过滤规则约束的是「同步落进白名单的范围」，弹窗要的是上游授权
// 全集供人工挑选，两者语义不同。
//
// 多分组时逐组拉取、按模型 ID 去重合并（组织级视图 = 各分组授权集并集，与
// mergeProviderModelCatalog 的目录口径同构）；单组失败或组内无可用凭据不阻断其余组，
// 全组都拿不到模型才报错。repos 为 nil 时用 repository.Default()。
func ListUpstreamModels(ctx context.Context, repos *repository.Repositories, providerID uint) ([]providers.Model, error) {
	if repos == nil {
		repos = repository.Default()
	}
	provider, err := repos.Provider.Get(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("get provider %d: %w", providerID, err)
	}

	snapshot, err := channel.NewAssembler(repos).Assemble(ctx, *provider)
	if err != nil {
		return nil, fmt.Errorf("assemble channel snapshot: %w", err)
	}
	if len(snapshot.Groups) == 0 {
		return nil, errors.New("no key groups to browse")
	}
	endpoint, ok := selectSyncEndpoint(provider.Type, snapshot.Endpoints)
	if !ok {
		return nil, errors.New("no enabled endpoint matching provider type")
	}

	now := time.Now()
	merged := make([]providers.Model, 0)
	seen := make(map[string]struct{})
	var (
		failures  []string
		succeeded bool
	)
	for _, group := range snapshot.Groups {
		client, err := groupUpstreamClient(*provider, snapshot, group, endpoint, now)
		if err != nil {
			// 组内无可用凭据不是错误：其余组可能仍持有可用 key（与同步的 Skipped 同义）。
			if !errors.Is(err, channel.ErrNoCredentialAvailable) {
				failures = append(failures, fmt.Sprintf("group %q: %s", group.Name, err))
			}
			continue
		}
		upstreamModels, err := client.Models(ctx)
		if err != nil {
			failures = append(failures, fmt.Sprintf("group %q: %s", group.Name, err))
			continue
		}
		succeeded = true
		for _, m := range upstreamModels {
			if _, dup := seen[m.ID]; dup {
				continue
			}
			seen[m.ID] = struct{}{}
			merged = append(merged, m)
		}
	}

	if !succeeded {
		if len(failures) > 0 {
			return nil, errors.New(strings.Join(failures, "; "))
		}
		return nil, errors.New("no usable credential in any key group")
	}
	return merged, nil
}
