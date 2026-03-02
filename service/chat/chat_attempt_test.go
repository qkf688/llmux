package chat

import "testing"

func TestApplyProviderSelectionResult_ReduceWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{ReduceWeight: true})

	if weightItems[1] != 6 {
		t.Fatalf("expected weight reduced to 6, got %d", weightItems[1])
	}
	if priorityItems[1] != 7 {
		t.Fatalf("expected priority unchanged, got %d", priorityItems[1])
	}
}

func TestApplyProviderSelectionResult_RemoveWeightAndPriority(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{
		RemoveWeight:   true,
		RemovePriority: true,
	})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; ok {
		t.Fatal("expected priority entry removed")
	}
}

func TestApplyProviderSelectionResult_RemoveOnlyWeight(t *testing.T) {
	weightItems := map[uint]int{1: 9}
	priorityItems := map[uint]int{1: 7}

	applyProviderSelectionResult(weightItems, priorityItems, 1, singleProviderAttemptResult{RemoveWeight: true})

	if _, ok := weightItems[1]; ok {
		t.Fatal("expected weight entry removed")
	}
	if _, ok := priorityItems[1]; !ok {
		t.Fatal("expected priority entry kept")
	}
}
