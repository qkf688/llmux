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

// TestShutdownOrder 锁定关闭三步的顺序：停机 → 排空后台写库 → 关库。
//
// 存在的理由只有一个：顺序被误调换（尤其把关库提到排空之前）会让在途写库任务撞
// 「数据库已关闭」而静默丢数据，此前这条约束只有注释在保护。
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

	want := []string{"srv", "bgtask", "closeDB"}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != len(want) {
		t.Fatalf("shutdown order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("shutdown order = %v, want %v", order, want)
		}
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
