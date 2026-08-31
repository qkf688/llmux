package channel

import (
	"fmt"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
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
		// 凭据层失败：仍带回已选端点/分组。#6-3 探活必须锁定「刚耗尽」的那一组，
		// 禁止调用方再跑 SelectGroup（选择即推进 RR，同权重组会切到另一组）。
		return SelectionResult{Endpoint: ep, Group: g}, err
	}
	plainKey, err := DecryptCredentialKey(c)
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

// RetryCredential 组内重选凭据（#13 组内故障转移）：锁定**原分组与原端点**，
// 在刷新后的快照上重选下一条可用凭据。调用时机：上一凭据凭据级失败并已写冷却
// 之后，调用方重新 Assemble 得到刷新快照——冷却中的凭据被 credentialUsable 剔除、
// 轮询指针「选择即推进」天然选到下一条；同请求内重试不再复用旧快照里的冷却值。
//
// 分组不自动切换：设计定案第 4 节「分组失败 → 供应商整体失败」，本层无可用凭据
// 即 ErrNoCredentialAvailable 上抛，由 chat 链路做组织级淘汰。
// 与 Select 复用同一尾部组装（解密 + BuildConfig），保证同套 config 继承链语义。
func (s *Selector) RetryCredential(snapshot *Snapshot, groupID uint, endpoint models.Endpoint, now time.Time) (SelectionResult, error) {
	c, err := SelectCredential(s, snapshot, groupID, now)
	if err != nil {
		return SelectionResult{}, err
	}
	plainKey, err := DecryptCredentialKey(c)
	if err != nil {
		return SelectionResult{}, err
	}
	cfg, upstreamURL, err := BuildConfig(snapshot.Provider, endpoint, plainKey)
	if err != nil {
		return SelectionResult{}, err
	}
	g, ok := groupByID(snapshot, groupID)
	if !ok {
		return SelectionResult{}, fmt.Errorf("%w: group %d gone after refresh", ErrNoGroupMatches, groupID)
	}
	return SelectionResult{
		Endpoint:    endpoint,
		Group:       g,
		Credential:  c,
		UpstreamURL: upstreamURL,
		Config:      cfg,
	}, nil
}

// groupByID 在快照分组中按 ID 取回分组信息（RetryCredential 锁组重选要用）。
func groupByID(snapshot *Snapshot, id uint) (models.KeyGroup, bool) {
	for _, g := range snapshot.Groups {
		if g.ID == id {
			return g, true
		}
	}
	return models.KeyGroup{}, false
}
