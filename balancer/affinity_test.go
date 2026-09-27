package balancer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAffinity_PutGetAndEmptyKey(t *testing.T) {
	a := NewAffinity(4, time.Hour)
	now := time.Unix(1_700_000_000, 0)

	target := Target{ModelID: 1, ModelWithProviderID: 2, GroupID: 3, CredentialID: 4}
	a.Put("k1", target, now)

	got, ok := a.Get("k1", now)
	if !ok {
		t.Fatal("Get(k1) miss after Put")
	}
	if got != target {
		t.Fatalf("Get(k1) = %+v, want %+v", got, target)
	}

	if _, ok := a.Get("unknown", now); ok {
		t.Fatal("Get(unknown) hit, want miss")
	}

	// 空键 = 未识别对话身份：Put 忽略、Get 恒 miss（调用方以空键关闭粘性）。
	a.Put("", target, now)
	if _, ok := a.Get("", now); ok {
		t.Fatal("Get(\"\") hit, want miss")
	}
}

func TestAffinity_TTLExpiryAndGetRefresh(t *testing.T) {
	a := NewAffinity(4, 30*time.Minute)
	t0 := time.Unix(1_700_000_000, 0)
	a.Put("k", Target{CredentialID: 1}, t0)

	// 空闲未超 TTL：命中；命中即续期——此后过期从「最近一次访问」重新起算
	//（契约：Affinity.Get「命中则刷新 recency，空闲 TTL 从此刻重新起算」+ AC-6「闲置超 30 分钟」）。
	if _, ok := a.Get("k", t0.Add(20*time.Minute)); !ok {
		t.Fatal("Get within TTL: miss, want hit")
	}
	refresh := t0.Add(20*time.Minute + 29*time.Minute) // 此次命中把 last 续期到该时刻
	if _, ok := a.Get("k", refresh); !ok {
		t.Fatal("Get after refresh within TTL: miss, want hit（命中续期）")
	}
	// 距最近一次访问（refresh）超过 TTL：过期即未钉。
	if _, ok := a.Get("k", refresh.Add(30*time.Minute+time.Second)); ok {
		t.Fatal("Get after idle TTL: hit, want miss")
	}
}

func TestAffinity_LRUEvictionAndOverwrite(t *testing.T) {
	a := NewAffinity(2, time.Hour)
	t0 := time.Unix(1_700_000_000, 0)
	a.Put("a", Target{CredentialID: 1}, t0)
	a.Put("b", Target{CredentialID: 2}, t0)
	// 刷新 a 的 recency → b 成为最久未用。
	if _, ok := a.Get("a", t0.Add(time.Minute)); !ok {
		t.Fatal("Get(a) miss")
	}
	a.Put("c", Target{CredentialID: 3}, t0.Add(2*time.Minute))

	if _, ok := a.Get("b", t0.Add(3*time.Minute)); ok {
		t.Fatal("b survived LRU eviction, want evicted")
	}
	if got, ok := a.Get("a", t0.Add(3*time.Minute)); !ok || got.CredentialID != 1 {
		t.Fatalf("a evicted or wrong target: ok=%v got=%+v", ok, got)
	}

	// 同键覆盖：目标更新、不占新槽。
	a.Put("a", Target{CredentialID: 9}, t0.Add(4*time.Minute))
	if got, ok := a.Get("a", t0.Add(5*time.Minute)); !ok || got.CredentialID != 9 {
		t.Fatalf("overwrite not applied: ok=%v got=%+v", ok, got)
	}
}

func TestAffinity_Reset(t *testing.T) {
	a := NewAffinity(4, time.Hour)
	t0 := time.Unix(1_700_000_000, 0)
	a.Put("k", Target{CredentialID: 1}, t0)
	a.Reset()
	if _, ok := a.Get("k", t0); ok {
		t.Fatal("Get hit after Reset, want miss")
	}
}

// TestAffinity_Concurrent 并发混打压测（Stage 29）：多 goroutine 交错 Get/Put/Reset，
// 锁的职责是保住「LRU list 与 map 计数一致、不超容量」的结构不变量与 Target 读写原子性。
// 数据竞争本体靠 -race 现形（Linux CI race job 跑 `go test -race ./...`；Windows 工具链
// 缺 cgo 时 -race 报 0xc0000139 是加载期缺件，不是竞态报告，见 RULES）；
// 结构撕裂（list/map 计数漂移）单跑 -count=N 也能暴露。
func TestAffinity_Concurrent(t *testing.T) {
	a := NewAffinity(64, time.Hour)
	now := time.Unix(1_700_000_000, 0) // 固定时刻：TTL 恒不过期，miss 只可能来自 Reset/淘汰
	const workers = 8
	const opsPerWorker = 500

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				// 每 worker 16 键循环（128 键 > max 64，制造 LRU 淘汰路径）+ 共享热点键（制造同键覆盖）。
				key := fmt.Sprintf("w%d-k%d", id, i%16)
				a.Put(key, Target{ModelID: uint(id + 1), CredentialID: uint(i)}, now)
				a.Get(key, now)
				a.Put("shared-hot", Target{ModelID: uint(id + 1)}, now)
				a.Get("shared-hot", now)
			}
		}(w)
	}
	// Reset 混打：并发清理与读写交错，是粘性表并发面的一部分（调用方测试隔离入口）。
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			a.Reset()
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	// 计数不变量：list 与 map 同步撕裂即红。
	if a.ll.Len() != len(a.byKey) {
		t.Fatalf("结构不变量破坏: ll.Len()=%d, len(byKey)=%d", a.ll.Len(), len(a.byKey))
	}
	if a.ll.Len() > a.max {
		t.Fatalf("容量超限: %d > max %d", a.ll.Len(), a.max)
	}
	// 终态闭环：混打停止后 Put/Get 语义完好（确定性断言，不受 Reset 干扰）。
	a.Put("final", Target{CredentialID: 7}, now)
	if got, ok := a.Get("final", now); !ok || got.CredentialID != 7 {
		t.Fatalf("终态闭环失败: ok=%v got=%+v", ok, got)
	}
}
