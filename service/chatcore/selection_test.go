package chatcore

import "testing"

func TestSelectByPriorityAndWeight_Empty(t *testing.T) {
	if _, err := SelectByPriorityAndWeight(map[uint]int{}, map[uint]int{}); err == nil {
		t.Fatal("expected error for empty weight items")
	}
}

func TestSelectByPriorityAndWeight_PicksHighestPriority(t *testing.T) {
	id, err := SelectByPriorityAndWeight(
		map[uint]int{1: 100, 2: 1},
		map[uint]int{1: 1, 2: 10},
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
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 5 {
		t.Fatalf("expected fallback ID 5, got %+v", id)
	}
}
