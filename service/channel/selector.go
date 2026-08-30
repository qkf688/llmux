package channel

import (
	"time"

	"github.com/qkf688/llmux/consts"
)

// Select 完成一次完整选路：端点 → 分组 → 凭据 → 动态 config。
//
// 每层错误携带 sentinel（errors.Is 可判），调用方言（chat 链路 #12）按层处置：
// 凭据层失败可组内换 key 重试 / 冷却（#13）；分组耗尽判定分组成败；
// 端点层失败则供应商整体失败。组织级语义（ModelWithProvider.ConsecutiveFailures）
// 不在这里触碰——本模块只选路，不判成败。
//
// now 由调用方注入：冷却判定与测试的时钟注入点（生产传 time.Now()）。
func (s *Selector) Select(snapshot *Snapshot, clientWire consts.WireFormat, modelName string, now time.Time) (SelectionResult, error) {
	ep, err := SelectEndpoint(snapshot, clientWire)
	if err != nil {
		return SelectionResult{}, err
	}
	g, err := SelectGroup(s, snapshot, modelName)
	if err != nil {
		return SelectionResult{}, err
	}
	c, err := SelectCredential(s, snapshot, g.ID, now)
	if err != nil {
		return SelectionResult{}, err
	}
	plainKey, err := plainCredentialKey(c)
	if err != nil {
		return SelectionResult{}, err
	}
	cfg, upstreamURL, err := BuildConfig(snapshot.Provider, ep, plainKey)
	if err != nil {
		return SelectionResult{}, err
	}
	return SelectionResult{
		Endpoint:    ep,
		Group:       g,
		Credential:  c,
		UpstreamURL: upstreamURL,
		Config:      cfg,
	}, nil
}
