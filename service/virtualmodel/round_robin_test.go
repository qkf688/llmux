package virtualmodel

import "testing"

func TestUpdateRoundRobinIndex_ModuloByCandidateCount(t *testing.T) {
	// 模拟 5 个启用映射但候选池过滤后只剩 3 个的场景。
	// 旧实现用全量映射数 5 取模，会把 index 推到 [0,5)；
	// 修复后用候选池长度 3 取模，index 应始终在 [0,3) 内。
	const (
		virtualModelID = uint(100)
		candidateCount = 3
	)

	s := newTestService()

	for i := 0; i < candidateCount*2; i++ {
		s.UpdateRoundRobinIndex(virtualModelID, candidateCount)

		got := s.roundRobinState[virtualModelID]
		if got >= candidateCount {
			t.Errorf("after %d-th advance, index = %d, want in [0, %d)",
				i+1, got, candidateCount)
		}
	}

	// 验证轮转模式：3 个候选应按 1,2,0,1,2,0 轮转
	wantSequence := []int{1, 2, 0, 1, 2, 0}
	s2 := newTestService()
	for i, want := range wantSequence {
		s2.UpdateRoundRobinIndex(virtualModelID, candidateCount)
		got := s2.roundRobinState[virtualModelID]
		if got != want {
			t.Errorf("round %d: index = %d, want %d", i+1, got, want)
		}
	}
}

func TestUpdateRoundRobinIndex_ZeroCandidateCount_NoOp(t *testing.T) {
	s := newTestService()
	const virtualModelID = uint(200)

	// candidateCount <= 0 时不应推进，也不应 panic（除零保护）
	s.UpdateRoundRobinIndex(virtualModelID, 0)
	if _, exists := s.roundRobinState[virtualModelID]; exists {
		t.Error("expected no state entry for candidateCount=0")
	}

	s.UpdateRoundRobinIndex(virtualModelID, -1)
	if _, exists := s.roundRobinState[virtualModelID]; exists {
		t.Error("expected no state entry for negative candidateCount")
	}
}

func TestUpdateRoundRobinIndex_WrapsAround(t *testing.T) {
	// 手动把 index 设到候选池长度 - 1，推进一次应回到 0
	const (
		virtualModelID = uint(300)
		candidateCount = 4
	)

	s := newTestService()
	s.roundRobinState[virtualModelID] = candidateCount - 1 // index = 3

	s.UpdateRoundRobinIndex(virtualModelID, candidateCount)

	if got := s.roundRobinState[virtualModelID]; got != 0 {
		t.Errorf("after wrap-around, index = %d, want 0", got)
	}
}

// newTestService 构造一个不带 DB 依赖的 Service（UpdateRoundRobinIndex 已不需要 DB）。
func newTestService() *Service {
	return &Service{
		roundRobinState: make(map[uint]int),
	}
}
