package channel

import "sync"

// groupRRKey 分组轮询键：同权重组间轮询按「供应商 + 权重档」区分。
type groupRRKey struct {
	providerID uint
	weight     int
}

// credRRKey 凭据轮询键：组内轮询按「供应商 + 分组」区分。
type credRRKey struct {
	providerID uint
	groupID    uint
}

// Selector 持有跨请求的轮询状态（分组权重档内轮询 + 凭据组内轮询）。
//
// 状态是进程内内存态：重启归零、多实例部署不同步——与虚拟模型轮询现状一致
// （网关选路无跨实例一致性要求，凭据层失败有冷却字段兜底）。可构造零值直接
// 使用（sync.Mutex 零值可用）；所有方法内部持锁，调用方无需（也不应）自行加锁。
//
// 两把 map 的 key 空间是配置变更史（providerID×weight / providerID×groupID），
// 不随请求量与凭据数量增长；删组/删供应商后旧 key 不回收，量级为配置级几十~
// 几百个 int，进程生命周期内可忽略，故不做清理机制。
type Selector struct {
	mu      sync.Mutex
	groupRR map[groupRRKey]int
	credRR  map[credRRKey]int
}

// Reset 清空全部轮询指针（测试隔离用）。
func (s *Selector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groupRR = nil
	s.credRR = nil
}

// nextGroupInRR 在「同权重档」候选切片上推进轮询指针：返回本次命中的下标
// （n 取模循环，选择即推进——失败转移是跳着选的，#13 无需"成功后回调"双轨）。
// 调用方保证 n > 0（档内必有成员）。
func (s *Selector) nextGroupInRR(providerID uint, weight, n int) (int, bool) {
	if n <= 0 {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupRR == nil {
		s.groupRR = make(map[groupRRKey]int)
	}
	key := groupRRKey{providerID: providerID, weight: weight}
	idx := s.groupRR[key] % n
	s.groupRR[key] = (idx + 1) % n
	return idx, true
}

// nextCredentialInRR 在组内凭据候选切片上推进轮询指针（语义同上）。
// 调用方保证 n > 0（候选列表非空）。
func (s *Selector) nextCredentialInRR(providerID, groupID uint, n int) (int, bool) {
	if n <= 0 {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.credRR == nil {
		s.credRR = make(map[credRRKey]int)
	}
	key := credRRKey{providerID: providerID, groupID: groupID}
	idx := s.credRR[key] % n
	s.credRR[key] = (idx + 1) % n
	return idx, true
}
