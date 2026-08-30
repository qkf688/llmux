package channel

import (
	"sync"
	"testing"
)

// TestSelectorGroupRR_Concurrent 锁 AC-6 轮询指针并发正确性：
// 20 goroutine 并发推进 2000 次（n=4 的档内轮询），最终指针位置必须等于
// 总推进次数 % 4（mutex 串行化下的精确语义），且无 -race 告警。
func TestSelectorGroupRR_Concurrent(t *testing.T) {
	s := &Selector{}
	const goroutines = 20
	const perGoroutine = 100
	const n = 4

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perGoroutine {
				idx, ok := s.nextGroupInRR(1, 3, n)
				if !ok || idx < 0 || idx >= n {
					t.Errorf("nextGroupInRR = (%d, %v), want index in [0,%d)", idx, ok, n)
					return
				}
			}
		}()
	}
	wg.Wait()

	total := goroutines * perGoroutine
	got, _ := s.nextGroupInRR(1, 3, n)
	want := total % n
	if got != want {
		t.Fatalf("第 %d 次推进后指针 = %d, want %d（并发推进丢步）", total+1, got, want)
	}
}

// TestSelectorCredRR_Advance 锁「选择即推进」单轨语义：同一候选集连续选择
// 依次命中每个候选后回到起点（取模循环）。
func TestSelectorCredRR_Advance(t *testing.T) {
	s := &Selector{}
	seen := make([]int, 0, 3)
	for range 3 {
		idx, ok := s.nextCredentialInRR(1, 7, 3)
		if !ok {
			t.Fatal("nextCredentialInRR(3) = not ok")
		}
		seen = append(seen, idx)
	}
	if seen[0] != 0 || seen[1] != 1 || seen[2] != 2 {
		t.Fatalf("连续推进 = %v, want [0 1 2]", seen)
	}
	// 第 4 次回到起点
	idx, _ := s.nextCredentialInRR(1, 7, 3)
	if idx != 0 {
		t.Fatalf("第 4 次推进 = %d, want 0（取模回绕）", idx)
	}
}

// TestSelectorReset 锁 Reset 复位语义：推进后 Reset 再选择回到起点。
func TestSelectorReset(t *testing.T) {
	s := &Selector{}
	s.nextCredentialInRR(1, 7, 3)
	s.nextCredentialInRR(1, 7, 3)
	s.Reset()
	idx, _ := s.nextCredentialInRR(1, 7, 3)
	if idx != 0 {
		t.Fatalf("Reset 后首次推进 = %d, want 0", idx)
	}
}
