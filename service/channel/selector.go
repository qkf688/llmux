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
//
// 轮询状态（分组档内 / 凭据组内）存在 Selector 实例内：生产路径必须复用一个
// 长寿实例（DefaultSelector），新建实例会让指针恒从 0 起算、多凭据/多组退化为
// 确定性首选（credential_select / group_select 的轮询语义失效）。
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

// defaultSelector 进程级共享选路器：轮询状态（分组档内 / 凭据组内）跨请求推进，
// 与虚拟模型轮询单例对齐（rrstate 注释「与虚拟模型轮询现状一致」）。chat 主链路
// 与未来 healthcheck/modelsync 复用同一实例，保证同一供应商的组/凭据压力分摊。
var defaultSelector = &Selector{}

// DefaultSelector 返回进程级共享选路器（并发安全，内部持锁）。
func DefaultSelector() *Selector { return defaultSelector }

// ResetDefaultSelectorForTest 清空默认选路器轮询指针（测试隔离用；
// 集成测试用例开头复位，避免受前序用例推进位置影响）。
func ResetDefaultSelectorForTest() { defaultSelector.Reset() }
