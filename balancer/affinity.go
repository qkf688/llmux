package balancer

import (
	"container/list"
	"sync"
	"time"
)

const (
	// DefaultAffinityMaxEntries 粘性表容量上限：键空间是「活跃对话身份数」，不设上限会
	// 随对话量无界增长；4096 覆盖个人/小团队网关的并发对话量，超出按 LRU 淘汰。
	DefaultAffinityMaxEntries = 4096
	// DefaultAffinityTTL 空闲过期：供应商提示词缓存 TTL 多为 5~10 分钟且命中续期，
	// 30 分钟空闲后原缓存必已蒸发，保留钉定只剩害处（挤占容量）。
	DefaultAffinityTTL = 30 * time.Minute
)

// Target 缓存亲和钉定的选路目标；字段 0 = 该层不钉。
//
// 刻意没有 EndpointID：端点按「入站 wire 命中或主协议」确定性选择（channel 设计定案
// 不做同协议多 URL），同一对话 wire 不变即天然稳定，钉了也是冗余。
type Target struct {
	ModelID             uint
	ModelWithProviderID uint
	GroupID             uint
	CredentialID        uint
}

type affinityEntry struct {
	key    string
	target Target
	last   time.Time // 最近命中/写入时刻，空闲 TTL 与 LRU 排序共用
}

// Affinity 会话粘性表：对话身份键 → 选路目标（LRU 容量上限 + 空闲 TTL 过期）。
//
// 进程内内存态：重启归零、多实例部署不同步——与 Selector 轮询状态（rrstate）同口径，
// 网关选路无跨实例一致性要求，钉定丢失只退化为不粘（回到现状选择），无正确性影响。
// 所有方法内部持锁（并发请求共享默认实例），now 由调用方注入（可测）。
type Affinity struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	ll    *list.List // *affinityEntry，front = 最近使用
	byKey map[string]*list.Element
}

// NewAffinity 构造粘性表；max/ttl 非正时回落默认值（配置错误不该做出「永不过期/
// 无上限」这种放大解释）。
func NewAffinity(max int, ttl time.Duration) *Affinity {
	if max <= 0 {
		max = DefaultAffinityMaxEntries
	}
	if ttl <= 0 {
		ttl = DefaultAffinityTTL
	}
	return &Affinity{max: max, ttl: ttl, ll: list.New(), byKey: make(map[string]*list.Element)}
}

// defaultAffinity 进程级共享粘性表：chat 主链路与虚拟路径复用同一实例，同一对话
// 无论走哪条路径都钉到同一目标。
var defaultAffinity = NewAffinity(DefaultAffinityMaxEntries, DefaultAffinityTTL)

// DefaultAffinity 返回进程级共享粘性表（并发安全）。
func DefaultAffinity() *Affinity { return defaultAffinity }

// ResetDefaultAffinityForTest 清空默认粘性表（测试隔离用；集成测试用例开头复位，
// 避免受前序用例钉定影响）。
func ResetDefaultAffinityForTest() { defaultAffinity.Reset() }

// Reset 清空全部粘性记录（测试隔离用）。
func (a *Affinity) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ll = list.New()
	a.byKey = make(map[string]*list.Element)
}

// Get 取钉定目标：命中则刷新 recency（空闲 TTL 从此刻重新起算）；过期/不存在/空键
// 均视为未钉（调用方回退常规选择，无需区分原因）。
func (a *Affinity) Get(key string, now time.Time) (Target, bool) {
	if key == "" {
		return Target{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	el, ok := a.byKey[key]
	if !ok {
		return Target{}, false
	}
	e := el.Value.(*affinityEntry)
	if now.Sub(e.last) > a.ttl {
		a.ll.Remove(el)
		delete(a.byKey, key)
		return Target{}, false
	}
	e.last = now
	a.ll.MoveToFront(el)
	return e.target, true
}

// Put 记录/改钉目标（成功后调用：首次钉定与转移后改钉同入口）。空键忽略；同键覆盖。
func (a *Affinity) Put(key string, t Target, now time.Time) {
	if key == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if el, ok := a.byKey[key]; ok {
		e := el.Value.(*affinityEntry)
		e.target = t
		e.last = now
		a.ll.MoveToFront(el)
		return
	}
	e := &affinityEntry{key: key, target: t, last: now}
	a.byKey[key] = a.ll.PushFront(e)
	for a.ll.Len() > a.max {
		a.evictOldest()
	}
}

// DeleteIf 摘掉指定键的钉定，且**仅当它当前仍指向 expectMWP（ModelWithProviderID）**
// 时才摘，返回是否真的摘掉。提交后失败（上游腰斩）时由 chat 层调用：这次没能从钉定
// 目标拿到成功，且已无法换家重来——不清钉会让该对话的下一个请求继续优先撞它。
//
// 为何要「比对后再删」而不是直接 Delete：
//   - 清钉发生在**响应交付之后**的日志管道 goroutine 里，而改钉（recordStickyTarget）
//     发生在转发成功那一刻。同一段对话的并发请求可能在两者之间改钉：若不比对，本次
//     的清钉会误删掉「另一次请求刚写进去的新钉」。
//   - 反过来，比对也**不会**挡住该清的：转发成功时 recordStickyTarget 已把钉子指到
//     本次真正服务的关联（含故障转移后的赢家），故清钉只要比对本次服务的关联即可命中。
//
// 比对粒度取关联而非凭据：同一关联内换 key 仍是「这家出了问题」，清关联级记忆才对。
//
// 与 Reset 全清的分界：只摘这一条对话，其余对话的钉定原样存活。一段对话出问题不该
// 让全体活跃会话集体失忆（那是 Reset 才会造成的惊群，故不在此路径使用）。
// 空键 / 未知键 / 重复删除 / 钉定已指向别家，一律返回 false，调用方无需先判存在性。
// 摘掉后该键不再占槽，后续成功仍可 Put 重建。
func (a *Affinity) DeleteIf(key string, expectMWP uint) bool {
	if key == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	el, ok := a.byKey[key]
	if !ok {
		return false
	}
	if el.Value.(*affinityEntry).target.ModelWithProviderID != expectMWP {
		return false // 钉定已指向别家：不是「刚断流的那家」，不动它
	}
	a.ll.Remove(el)
	delete(a.byKey, key)
	return true
}

// evictOldest 淘汰最久未用记录（调用方持锁）。
func (a *Affinity) evictOldest() {
	el := a.ll.Back()
	if el == nil {
		return
	}
	a.ll.Remove(el)
	delete(a.byKey, el.Value.(*affinityEntry).key)
}
