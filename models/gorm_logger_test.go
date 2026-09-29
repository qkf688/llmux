package models

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormLogLeakProbe 仅用于日志行为测试：Token 带唯一索引，重复插入必然触发
// 失败语句日志路径（生产链路里 api_keys.KeyHash 撞库即此路径）。
type gormLogLeakProbe struct {
	ID    uint   `gorm:"primaryKey"`
	Token string `gorm:"uniqueIndex;type:varchar(64)"`
}

// openGormLogTestDB 用 testutil.NewFile 独立文件库：TempDir 唯一 DSN 不受子测试名
// （含 /、CJK）干扰，也不依赖 cache=shared 与连接复用的巧合；「测试结束先关连接
// 再让 TempDir 清理」的意图由 testutil 契约内建（cleanup LIFO，不泄漏句柄）。
// 原 DSN 的 busy_timeout(5000) 由 FileOptions.BusyTimeoutMs 承担。
func openGormLogTestDB(t *testing.T, gormCfg *gorm.Config) *gorm.DB {
	t.Helper()
	db, _ := testutil.NewFile(t, testutil.FileOptions{BusyTimeoutMs: 5000, Gorm: gormCfg}, &gormLogLeakProbe{})
	return db
}

func newLoggerForTest(buf *bytes.Buffer, config logger.Config) logger.Interface {
	return newGormSlogLogger(slog.New(slog.NewTextHandler(buf, nil)), config)
}

// 回归 Stage 1：撞唯一索引的失败 INSERT 曾被 gorm 默认 logger 以参数内插后的
// 完整 SQL 打进 stdout（密文 hex / SHA-256 hash 随行泄露）。适配器必须
// 「见 SQL 不见值」。
func TestGormSlogLogger_FailedStatementLogsSQLButNotParamValues(t *testing.T) {
	var buf bytes.Buffer
	db := openGormLogTestDB(t, &gorm.Config{Logger: newLoggerForTest(&buf, logger.Config{
		LogLevel:             logger.Error,
		ParameterizedQueries: true,
	})})

	const sentinel = "SENTINEL-TOKEN-4f9a17"
	if err := db.Create(&gormLogLeakProbe{Token: sentinel}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}
	buf.Reset()
	if err := db.Create(&gormLogLeakProbe{Token: sentinel}).Error; err == nil {
		t.Fatal("duplicate insert 应失败（触发失败语句日志路径）")
	}

	out := buf.String()
	if !strings.Contains(out, "INSERT") {
		t.Fatalf("失败 INSERT 应记结构化日志，实得: %q", out)
	}
	if strings.Contains(out, sentinel) {
		t.Fatalf("日志泄露了参数值: %q", out)
	}
	if !strings.Contains(out, "UNIQUE constraint failed") {
		t.Fatalf("日志应保留原始错误文本供排障，实得: %q", out)
	}
}

// /v1 鉴权按 hash 点查 miss 会返回 ErrRecordNotFound；默认 gorm logger 将其
// 整条打日志（外部可无限刷）。开关语义必须与 gorm 默认实现一致，且
// wantLogged 分支要断言到内容（任何一行日志都不能算数）。
func TestGormSlogLogger_RecordNotFoundSwitch(t *testing.T) {
	tests := []struct {
		name       string
		ignoreFlag bool
		wantLogged bool
	}{
		{name: "ignore_on", ignoreFlag: true, wantLogged: false},
		{name: "ignore_off", ignoreFlag: false, wantLogged: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			db := openGormLogTestDB(t, &gorm.Config{Logger: newLoggerForTest(&buf, logger.Config{
				LogLevel:                  logger.Error,
				ParameterizedQueries:      true,
				IgnoreRecordNotFoundError: tt.ignoreFlag,
			})})
			buf.Reset()

			var probe gormLogLeakProbe
			err := db.Where("token = ?", "no-such-token").First(&probe).Error
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("期望 ErrRecordNotFound，实得 %v", err)
			}
			out := buf.String()
			if tt.wantLogged {
				if !strings.Contains(out, "record not found") || !strings.Contains(out, "SELECT") {
					t.Fatalf("未开启忽略时 miss 应记含错误与 SQL 的 error 日志，实得: %q", out)
				}
			} else if out != "" {
				t.Fatalf("开启忽略后 miss 不应留痕，实得: %q", out)
			}
		})
	}
}

// 慢查询分支：阈值为负使任意耗时（含 Windows 时钟 tick 下的 0）必命中，
// 断言成功语句超阈记 Warn 且不泄值。
func TestGormSlogLogger_SlowQueryLoggedAtWarn(t *testing.T) {
	var buf bytes.Buffer
	db := openGormLogTestDB(t, &gorm.Config{Logger: newLoggerForTest(&buf, logger.Config{
		LogLevel:             logger.Warn,
		SlowThreshold:        -1,
		ParameterizedQueries: true,
	})})
	buf.Reset()

	const sentinel = "SENTINEL-SLOW-77b2"
	if err := db.Create(&gormLogLeakProbe{Token: sentinel}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "slow sql") {
		t.Fatalf("超阈值语句应记 slow sql，实得: %q", out)
	}
	if strings.Contains(out, sentinel) {
		t.Fatalf("慢查询日志泄露了参数值: %q", out)
	}
}

// LogMode 必须返回副本而非就地改（gorm Debug() 按会话开 Info，污染共享实例
// 会把全进程 SQL 全量放开）。用行为断言：副本级放行 Info、原实例保持静默。
func TestGormSlogLogger_LogModeDoesNotMutateOriginal(t *testing.T) {
	var buf bytes.Buffer
	base := newLoggerForTest(&buf, logger.Config{LogLevel: logger.Warn, SlowThreshold: 0})
	session := base.LogMode(logger.Info)

	fc := func() (string, int64) { return "SELECT probe_col", 1 }
	session.Trace(context.Background(), time.Now(), fc, nil)
	if !strings.Contains(buf.String(), "SELECT probe_col") {
		t.Fatalf("LogMode(Info) 副本应放行 info 级语句日志，实得: %q", buf.String())
	}

	buf.Reset()
	base.Trace(context.Background(), time.Now(), fc, nil)
	if buf.Len() > 0 {
		t.Fatalf("LogMode 污染了原实例（Warn 级不应有 info 输出）: %q", buf.String())
	}
}

// 装配策略守卫：生产配置与 gorm 默认 logger 的同参项（Warn / 200ms）与两个
// 泄露闸是「策略常数」，允许字段断言（行为面已由下方泄闸测试覆盖）。
func TestGormConfig_LoggerPolicyParity(t *testing.T) {
	adapter, ok := gormConfig(slog.New(slog.NewTextHandler(io.Discard, nil))).Logger.(*gormSlogLogger)
	if !ok {
		t.Fatal("gormConfig 必须装配 *gormSlogLogger（slog 桥接门面）")
	}
	if adapter.LogLevel != logger.Warn {
		t.Errorf("LogLevel = %v, want Warn（与 gorm 默认同参）", adapter.LogLevel)
	}
	if adapter.SlowThreshold != 200*time.Millisecond {
		t.Errorf("SlowThreshold = %v, want 200ms（与 gorm 默认同参）", adapter.SlowThreshold)
	}
}

// 装配契约：gormConfig（生产 Init 的配置单一来源）两个泄露闸常开，全部用行为
// 断言钉住——not-found 不留痕、失败语句见 SQL 不见值——防止日后改配置改出静默回归。
func TestGormConfig_LeakGuardsAlwaysOn(t *testing.T) {
	var buf bytes.Buffer
	db := openGormLogTestDB(t, gormConfig(slog.New(slog.NewTextHandler(&buf, nil))))
	buf.Reset()

	var probe gormLogLeakProbe
	if err := db.Where("token = ?", "absent").First(&probe).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("期望 ErrRecordNotFound，实得 %v", err)
	}
	if out := buf.String(); strings.Contains(out, "record not found") {
		t.Fatalf("生产配置必须 IgnoreRecordNotFoundError=true，miss 却留痕: %q", out)
	}

	const sentinel = "SENTINEL-WIRING-c81d"
	if err := db.Create(&gormLogLeakProbe{Token: sentinel}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}
	buf.Reset()
	if err := db.Create(&gormLogLeakProbe{Token: sentinel}).Error; err == nil {
		t.Fatal("duplicate insert 应失败")
	}

	out := buf.String()
	if !strings.Contains(out, "INSERT") {
		t.Fatalf("生产配置下失败语句仍应留 SQL 形状日志，实得: %q", out)
	}
	if strings.Contains(out, sentinel) {
		t.Fatalf("生产配置泄露参数值: %q", out)
	}
}
