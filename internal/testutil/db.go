// Package testutil 是仓库唯一的测试建库工具（Stage 35 测试隔离收口）。
//
// 治的病：测试建库点各自写 gorm.Open，固定名 / 复用名的内存库在同一进程
// 连跑（go test -count=N）时逐轮串数据——第一轮写入的数据留给第二轮，断言
// 从「值不对」假红成「数据多了」。根因不是断言写错，是库不隔离。
//
// 契约（所有入口共同）：
//   - 每次调用发独立 DSN（进程内计数器），跨测试、跨 -count 轮次互不可见；
//   - AutoMigrate 传入的表集合由调用方给全（表集合是被测行为的一部分）；
//   - t.Cleanup 关连接：内存库随连接释放，文件库在 TempDir 清理前先关，
//     避免 Windows 上 SQLite 文件残留锁把绿用例拖成 FAIL。
package testutil

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// dbSeq 保证每个建库调用拿到唯一 DSN。进程内计数即够：-count=N 的多轮
// 同进程重复调用也各拿新名，这正是 -count 串库假红的根治点。
var dbSeq atomic.Int64

// New 返回进程内唯一的共享内存库（cache=shared + 单连接）。
//
// cache=shared 让多连接共享同一份内存库（裸 :memory: 每个连接各是一份空库，
// 表不可见）；再加单连接是双保险——SQLite 内存库每连接独立，单连接把并发写在
// 连接池上串行化，杜绝「建表连接 ≠ 查询连接」各看各的空库（项目测试纪律同条）。
func New(t *testing.T, tables ...any) *gorm.DB {
	t.Helper()
	return NewWithConfig(t, nil, tables...)
}

// NewWithConfig 同 New，但允许自定义 gorm.Config（如测试专用 logger 的
// 日志脱敏回归）。cfg 为 nil 时用默认配置。
func NewWithConfig(t *testing.T, cfg *gorm.Config, tables ...any) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:testutil_%d?mode=memory&cache=shared", dbSeq.Add(1))
	db := open(t, dsn, cfg, tables...)
	sqlDB := sqlDBOf(t, db)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db
}

// FileOptions 是文件库的可选项；零值 = 默认 journal、默认 gorm.Config。
type FileOptions struct {
	// WAL 开 journal_mode(WAL)：导出回归依赖「WAL 未 checkpoint」形态，
	// 该形态在内存库上不存在，必须文件库 + WAL。
	WAL bool
	// BusyTimeoutMs 开 busy_timeout(N)（毫秒）：并发写文件库时等待锁而非立报
	// SQLITE_BUSY（gorm 日志脱敏回归的文件库夹具用）。
	BusyTimeoutMs int
	// Gorm 自定义 gorm.Config，nil 用默认。
	Gorm *gorm.Config
}

// NewFile 返回 t.TempDir() 下的真实文件库与库路径。
//
// 文件库保留多连接真实语义（不强制单连接）；库路径供需要扫库文件字节
// （脱敏回归）或 VACUUM INTO（导出回归）的用例使用。
func NewFile(t *testing.T, opts FileOptions, tables ...any) (*gorm.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "testutil.db")
	dsn := path
	sep := "?"
	if opts.WAL {
		dsn += sep + "_pragma=journal_mode(WAL)"
		sep = "&"
	}
	if opts.BusyTimeoutMs > 0 {
		dsn += sep + fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeoutMs)
	}
	return open(t, dsn, opts.Gorm, tables...), path
}

// OpenExisting 打开一个已存在的库文件（如导出产物）做断言，只挂清理不建表。
// 与 NewFile 的区别：不创建新库、不迁移——被测物是文件本身。
func OpenExisting(t *testing.T, path string) *gorm.DB {
	t.Helper()
	return open(t, path, nil)
}

// open 统一开库 + 建表 + 登记清理。建表失败即 t.Fatal：测试夹具建不全就该
// 直接停，不该让后续用例带着缺表状态继续跑。
func open(t *testing.T, dsn string, cfg *gorm.Config, tables ...any) *gorm.DB {
	t.Helper()
	if cfg == nil {
		cfg = &gorm.Config{}
	}
	db, err := gorm.Open(sqlite.Open(dsn), cfg)
	if err != nil {
		t.Fatalf("testutil: open %s: %v", dsn, err)
	}
	// 关连接先于 TempDir 清理（t.Cleanup LIFO：本条后注册、先执行）。
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if len(tables) > 0 {
		if err := db.AutoMigrate(tables...); err != nil {
			t.Fatalf("testutil: migrate %s: %v", dsn, err)
		}
	}
	return db
}

func sqlDBOf(t *testing.T, db *gorm.DB) *sql.DB {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("testutil: get sql.DB: %v", err)
	}
	return sqlDB
}
