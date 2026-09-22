package chatcore

import "testing"

func TestSelectByPriorityAndWeight_Empty(t *testing.T) {
	if _, err := SelectByPriorityAndWeight(map[uint]int{}, map[uint]int{}, 0); err == nil {
		t.Fatal("expected error for empty weight items")
	}
}

func TestSelectByPriorityAndWeight_PicksHighestPriority(t *testing.T) {
	id, err := SelectByPriorityAndWeight(
		map[uint]int{1: 100, 2: 1},
		map[uint]int{1: 1, 2: 10},
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 2 {
		t.Fatalf("expected ID 2 from highest priority set, got %+v", id)
	}
}

func TestSelectByPriorityAndWeight_FallbackToWeightOnly(t *testing.T) {
	id, err := SelectByPriorityAndWeight(
		map[uint]int{5: 1},
		map[uint]int{},
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 5 {
		t.Fatalf("expected fallback ID 5, got %+v", id)
	}
}

// TestSelectByPriorityAndWeight_Prefer 锁 AC-3 粘性不越权：preferID 只在
// 「最高优先级档内且权重 > 0」时直返（0 权重既有语义 = 不参与，粘性不得复活它）；
// 掉出档外 / 权重 0 / 不在池中 → 不钉、按现状选择。
func TestSelectByPriorityAndWeight_Prefer(t *testing.T) {
	weight := map[uint]int{1: 100, 2: 1, 3: 50}
	priority := map[uint]int{1: 1, 2: 10, 3: 10}

	// ① prefer 在最高档（2/3 同为 priority 10）→ 直返
	id, err := SelectByPriorityAndWeight(weight, priority, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 3 {
		t.Fatalf("prefer 档内未被钉: %+v, want 3", id)
	}

	// ② prefer 掉出高档（1 的 priority=1）→ 不钉，结果必须仍在高档 {2,3}
	for range 20 {
		id, err = SelectByPriorityAndWeight(weight, priority, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id == nil || *id == 1 {
			t.Fatalf("低优先级档的 prefer 被钉住了: %+v", id)
		}
	}

	// ③ prefer 权重 0（既有语义：不参与）→ 不钉，只剩唯一合法候选 1
	id, err = SelectByPriorityAndWeight(map[uint]int{1: 100, 2: 0}, map[uint]int{1: 5, 2: 5}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 1 {
		t.Fatalf("权重 0 的 prefer 被钉住了: %+v, want 1", id)
	}

	// ④ prefer 不在池中 → 现状行为（结果仍在池内）
	id, err = SelectByPriorityAndWeight(weight, priority, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id == 99 {
		t.Fatalf("池外 prefer 被钉住了: %+v", id)
	}
}
