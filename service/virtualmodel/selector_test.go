package virtualmodel

import "testing"

func TestRegisterSelector_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate selector registration")
		}
	}()
	RegisterSelector("priority", prioritySelectorInstance)
}

func TestGetSelector_KnownStrategies(t *testing.T) {
	cases := []struct {
		strategy      string
		wantAdvance   bool
		wantFound     bool
	}{
		{"priority", false, true},
		{"round_robin", true, true},
		{"random", false, true},
		{"unknown", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.strategy, func(t *testing.T) {
			sel, found := GetSelector(tc.strategy)
			if found != tc.wantFound {
				t.Errorf("GetSelector(%q) found = %v, want %v", tc.strategy, found, tc.wantFound)
			}
			if sel == nil {
				t.Fatalf("GetSelector(%q) returned nil selector", tc.strategy)
			}
			if got := sel.RequiresAdvanceOnSuccess(); got != tc.wantAdvance {
				t.Errorf("RequiresAdvanceOnSuccess() = %v, want %v", got, tc.wantAdvance)
			}
		})
	}
}

func TestGetSelector_UnknownFallsBackToPriority(t *testing.T) {
	sel, found := GetSelector("nonexistent")
	if found {
		t.Error("expected found = false for unknown strategy")
	}
	if sel != prioritySelectorInstance {
		t.Error("expected fallback to priority selector")
	}
}
