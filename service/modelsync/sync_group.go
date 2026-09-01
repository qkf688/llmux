package modelsync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

// groupSyncOutcome 单分组同步结果。Skipped 表示未打上游（无凭据/无端点）；
// Err 表示打了上游或写库失败。全量替换：成功时 KeyGroup.Models = 上游授权集 CSV。
type groupSyncOutcome struct {
	GroupID    uint
	GroupName  string
	Skipped    bool
	SkipReason string
	ModelIDs   []string
	Err        error
}

// syncKeyGroup 用该组活跃凭据拉 /models，全量写入 KeyGroup.Models。
// 端点：启用且协议 = ProtocolOfType(Provider.Type) 的最小 ID；凭据：组内首个可用。
func (s *Service) syncKeyGroup(
	ctx context.Context,
	provider models.Provider,
	snapshot *channel.Snapshot,
	group models.KeyGroup,
	now time.Time,
) groupSyncOutcome {
	out := groupSyncOutcome{GroupID: group.ID, GroupName: group.Name}

	endpoint, ok := selectSyncEndpoint(provider.Type, snapshot.Endpoints)
	if !ok {
		out.Skipped = true
		out.SkipReason = "no enabled endpoint matching provider type"
		return out
	}

	cred, err := channel.SelectCredential(&channel.Selector{}, snapshot, group.ID, now)
	if err != nil {
		out.Skipped = true
		out.SkipReason = "no usable credential"
		return out
	}

	plainKey, err := channel.DecryptCredentialKey(cred)
	if err != nil {
		out.Err = fmt.Errorf("group %d decrypt credential: %w", group.ID, err)
		return out
	}
	cfg, _, err := channel.BuildConfig(provider, endpoint, plainKey)
	if err != nil {
		out.Err = fmt.Errorf("group %d build config: %w", group.ID, err)
		return out
	}
	cfg, err = dropCustomModels(cfg)
	if err != nil {
		out.Err = fmt.Errorf("group %d strip custom models: %w", group.ID, err)
		return out
	}

	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		out.Err = fmt.Errorf("group %d unknown endpoint protocol %q", group.ID, endpoint.Protocol)
		return out
	}
	chatModel, err := providers.New(providerType, cfg, provider.Proxy)
	if err != nil {
		out.Err = fmt.Errorf("group %d create provider client: %w", group.ID, err)
		return out
	}

	upstreamModels, err := chatModel.Models(ctx)
	if err != nil {
		out.Err = fmt.Errorf("group %d fetch models: %w", group.ID, err)
		return out
	}

	if provider.ModelFilterEnabled != nil && *provider.ModelFilterEnabled {
		filtered, ferr := s.filterModelsByRules(ctx, upstreamModels)
		if ferr != nil {
			out.Err = fmt.Errorf("group %d filter rules: %w", group.ID, ferr)
			return out
		}
		upstreamModels = filtered
	}

	modelIDs := collectUpstreamModelIDs(upstreamModels)
	modelsCSV := strings.Join(modelIDs, ",")
	if _, err := s.repos.KeyGroup.UpdateFields(ctx, group.ID, map[string]any{"models": modelsCSV}); err != nil {
		out.Err = fmt.Errorf("group %d update models whitelist: %w", group.ID, err)
		return out
	}

	out.ModelIDs = modelIDs
	return out
}

// selectSyncEndpoint 取启用端点中协议匹配 Provider.Type 默认协议、且 ID 最小者。
// endpoints 调用方应按 ID ASC（Assembler 已保证）。
func selectSyncEndpoint(providerType string, endpoints []models.Endpoint) (models.Endpoint, bool) {
	want := string(consts.ProtocolOfType(providerType))
	for _, ep := range endpoints {
		if ep.Enabled && ep.Protocol == want {
			return ep, true
		}
	}
	return models.Endpoint{}, false
}
