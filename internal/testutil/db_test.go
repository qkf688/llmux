package testutil

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

type isoProbe struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

// TestNew_IsolatedAcrossCalls 锁建库隔离契约：两次建库互不可见。
// 这就是 -count=N 串库假红的根治点——同进程重复建库若复用同一份数据，
// 用例断言会从「值不对」劣化成「数据逐轮翻倍」。
func TestNew_IsolatedAcrossCalls(t *testing.T) {
	ctx := context.Background()
	a := New(t, &isoProbe{})
	b := New(t, &isoProbe{})

	if err := a.WithContext(ctx).Create(&isoProbe{Name: "only-in-a"}).Error; err != nil {
		t.Fatalf("create in a: %v", err)
	}

	var countA, countB int64
	if err := a.WithContext(ctx).Model(&isoProbe{}).Count(&countA).Error; err != nil {
		t.Fatalf("count a: %v", err)
	}
	if err := b.WithContext(ctx).Model(&isoProbe{}).Count(&countB).Error; err != nil {
		t.Fatalf("count b: %v", err)
	}
	if countA != 1 {
		t.Fatalf("a sees %d rows, want 1", countA)
	}
	if countB != 0 {
		t.Fatalf("b sees %d rows, want 0（串库：a 的数据漏进 b）", countB)
	}
}

// TestNew_MultiConnSeesSameDB 锁共享内存库语义：单连接约束下并发 goroutine
// 也能看到同一份数据（建表连接与查询连接不分离是前提）。
func TestNew_MultiConnSeesSameDB(t *testing.T) {
	ctx := context.Background()
	db := New(t, &isoProbe{})

	done := make(chan error, 1)
	go func() {
		done <- db.WithContext(ctx).Create(&isoProbe{Name: "from-goroutine"}).Error
	}()
	if err := <-done; err != nil {
		t.Fatalf("goroutine create: %v", err)
	}

	var n int64
	if err := db.WithContext(ctx).Model(&isoProbe{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

// TestNewFile_PathAndIsolation 锁文件库入口：返回可用库路径、库可读写、
// 两次调用目录互不干扰。
func TestNewFile_PathAndIsolation(t *testing.T) {
	ctx := context.Background()
	dbA, pathA := NewFile(t, FileOptions{}, &isoProbe{})
	dbB, pathB := NewFile(t, FileOptions{}, &isoProbe{})

	if pathA == "" || pathB == "" {
		t.Fatal("NewFile 返回空库路径")
	}
	if pathA == pathB {
		t.Fatalf("两次调用同一路径 %q，TempDir 应互不干扰", pathA)
	}

	if err := dbA.WithContext(ctx).Create(&isoProbe{Name: "x"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var countA, countB int64
	if err := dbA.WithContext(ctx).Model(&isoProbe{}).Count(&countA).Error; err != nil {
		t.Fatalf("count a: %v", err)
	}
	if err := dbB.WithContext(ctx).Model(&isoProbe{}).Count(&countB).Error; err != nil {
		t.Fatalf("count b: %v", err)
	}
	if countA != 1 || countB != 0 {
		t.Fatalf("counts = (%d, %d), want (1, 0)", countA, countB)
	}
}

// TestNewFile_OptionsApplied 锁选项生效：WAL 与 busy_timeout 落到库上
// （导出回归依赖 WAL 形态，选项失效等于回归静默失守）。
func TestNewFile_OptionsApplied(t *testing.T) {
	db, _ := NewFile(t, FileOptions{WAL: true, BusyTimeoutMs: 5000}, &isoProbe{})
	assertPragma(t, db, "journal_mode", "wal")
	assertPragma(t, db, "busy_timeout", "5000")
}

func assertPragma(t *testing.T, db *gorm.DB, name, want string) {
	t.Helper()
	var got string
	row := db.Raw("PRAGMA " + name).Row()
	if err := row.Scan(&got); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("PRAGMA %s = %q, want %q", name, got, want)
	}
}
