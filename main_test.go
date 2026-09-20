package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
)

// recordingShutdowner 只记录「srv 被停机」这一时点，不做真实停机——
// 本测试关心的是三步的相对顺序，起真服务只会引入端口与时序噪声。
type recordingShutdowner struct {
	record func(string)
}

func (r *recordingShutdowner) Shutdown(context.Context) error {
	r.record("srv")
	return nil
}

// TestShutdownOrder 锁定关闭序的真不变式：停机被调用、排空先于关库、关库在最后。
//
// 存在的理由只有一个：顺序被误调换（尤其把关库提到排空之前）会让在途写库任务撞
// 「数据库已关闭」而静默丢数据，此前这条约束只有注释在保护。
//
// 刻意**不锁** srv 与 bgtask 记录的相对顺序：bgtask 那条记录来自任务 goroutine 完成
// 时点（夹具里 sleep 20ms 后落），全量负载下调度延迟 >20ms 时它会先于 shutdown 被调用
// 就完成——那是夹具时序、不是关闭序。srv→bgtask→closeDB 的调用顺序由 shutdown 内部
// 串行执行保证（main.go 的 shutdown 逐步同步调用，读码可见），本测试锁的是它对数据
// 安全真正有意义的后果：排空完成（记录已落）发生在关库之前，且关库是最后一步。
func TestShutdownOrder(t *testing.T) {
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	mgr := bgtask.NewManager()
	// 登记一个耗时任务：mgr.Shutdown 必须等它跑完才返回，故它的落点即证明
	// 「排空」整体发生在 closeDB 之前，而非仅仅调用了一下 Shutdown。
	mgr.Go(func(context.Context) {
		time.Sleep(20 * time.Millisecond)
		record("bgtask")
	})

	closeDB := func() error {
		record("closeDB")
		return nil
	}

	shutdown(&recordingShutdowner{record: record}, mgr, closeDB, nil)

	mu.Lock()
	defer mu.Unlock()

	idx := func(step string) int {
		for i, s := range order {
			if s == step {
				return i
			}
		}
		return -1
	}

	// ① 三步各被记录恰好一次（缺失 = 某步被跳过）
	for _, step := range []string{"srv", "bgtask", "closeDB"} {
		n := 0
		for _, s := range order {
			if s == step {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("shutdown order = %v, want %s 恰好一次", order, step)
		}
	}
	// ② 关库在最后：数据安全的核心约束
	if order[len(order)-1] != "closeDB" {
		t.Fatalf("shutdown order = %v, want closeDB 最后（排空中的任务仍要写库）", order)
	}
	// ③ bgtask 排空落点先于关库：本测试存在的理由
	if idx("bgtask") > idx("closeDB") {
		t.Fatalf("shutdown order = %v, want bgtask 排空先于 closeDB", order)
	}
}

// TestShutdownContinuesAfterStepFailure 任一步失败都不得提前返回：
// 否则停机失败会让数据库连接永不关闭（这正是关闭序每步都只记日志的原因）。
func TestShutdownContinuesAfterStepFailure(t *testing.T) {
	var closed bool

	shutdown(failingShutdowner{}, bgtask.NewManager(), func() error {
		closed = true
		return nil
	}, nil)

	if !closed {
		t.Fatal("closeDB was not called after server shutdown failed; database would leak")
	}
}

type failingShutdowner struct{}

func (failingShutdowner) Shutdown(context.Context) error {
	return errors.New("shutdown failed")
}
