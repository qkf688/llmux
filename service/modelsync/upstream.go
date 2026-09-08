package modelsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/channel"
)

// ListUpstreamModels 只读拉取供应商的上游模型列表（管理端「上游模型」弹窗数据源）。
//
// 与 SyncProviderModels 共用三端口径（channel.SelectPrimaryEndpoint + channel.GroupCandidates
// + channel.NewClientForGroup），差别只在**不落库**：不写 KeyGroup.Models、不产 ModelSyncLog、
// 不触发同步后动作，也**不应用过滤规则**——过滤规则约束的是「同步落进白名单的范围」，
// 弹窗要的是上游授权全集供人工挑选，两者语义不同。
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
	groups := channel.GroupCandidates(snapshot)
	if len(groups) == 0 {
		return nil, errors.New("no key groups to browse")
	}
	endpoint, ok := channel.SelectPrimaryEndpoint(provider.Type, snapshot.Endpoints)
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
	for _, group := range groups {
		client, err := channel.NewClientForGroup(snapshot, group, endpoint, now)
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
