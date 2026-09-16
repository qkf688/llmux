package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormSlogLogger 把 gorm 的 SQL 日志桥接进 slog（日志门面单源，不再裸写 stdout），
// 并关闭敏感参数泄露：gorm 默认 logger 在失败/慢语句时打印参数内插后的完整 SQL，
// api_keys / credentials 的失败 INSERT（撞唯一索引是设计内常规路径，应用层不预查，
// 见 api-keys 方案 §5）会把密文 hex、SHA-256 hash 整行落进日志管道。
//
// 闸的作用域（必须知道，否则会误以为高枕无忧）：ParamsFilter 只护常规回调路径
// （gorm callbacks.go 的 trace 闭包先调 ParamsFilter 再 Explain）。db.Scan()/
// Raw(...).Scan() finisher 内部临时换 logger.Recorder，其 ParamsFilter 走进程级
// RecorderParamsFilter（默认透传），传给我们的已是内插值的成品 SQL，无法二次过滤
// （覆写该全局变量会引入跨包可变全局态，否决）。契约：含敏感绑定值（key 密文/hash）
// 的查询禁用 Scan finisher，一律走 Find/First——同契约记入 system-ops.md 接口表。
type gormSlogLogger struct {
	*slog.Logger
	logger.Config
}

// newGormSlogLogger 构造接入 slog 的 gorm logger。级别阈值语义与 gorm 默认
// logger 对齐（Error=失败语句，Warn=慢查询，Info=全量），不另造判据。
func newGormSlogLogger(slogger *slog.Logger, config logger.Config) logger.Interface {
	return &gormSlogLogger{Logger: slogger, Config: config}
}

// LogMode 返回仅改级别的副本（gorm 的 Debug() 等按会话调 LogMode，不得污染共享实例）。
func (g *gormSlogLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *g
	clone.LogLevel = level
	return &clone
}

func (g *gormSlogLogger) Info(ctx context.Context, msg string, data ...any) {
	if g.LogLevel >= logger.Info {
		g.Logger.InfoContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (g *gormSlogLogger) Warn(ctx context.Context, msg string, data ...any) {
	if g.LogLevel >= logger.Warn {
		g.Logger.WarnContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (g *gormSlogLogger) Error(ctx context.Context, msg string, data ...any) {
	if g.LogLevel >= logger.Error {
		g.Logger.ErrorContext(ctx, fmt.Sprintf(msg, data...))
	}
}

// ParamsFilter 实现 gorm.ParamsFilter，常规回调路径的泄值总闸。
func (g *gormSlogLogger) ParamsFilter(ctx context.Context, sql string, params ...any) (string, []any) {
	if g.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

// Trace 逐语句回调。分支判定与 gorm v1.30 默认实现等价，仅三处展示层差异：
// 不带 FileWithLineNum（调用点在 gorm 内部无诊断价值，需要 source 用 slog
// HandlerOptions.AddSource 统一开）；Colorful 忽略；rows 未知（-1）时省略该
// 属性而非打魔法数。IgnoreRecordNotFoundError 关闭 /v1 鉴权 hash 点查 miss
// 时的 not-found 日志洪水。
func (g *gormSlogLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if g.LogLevel <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	elapsedMS := float64(elapsed.Nanoseconds()) / 1e6

	switch {
	case err != nil && g.LogLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !g.IgnoreRecordNotFoundError):
		sql, rows := fc()
		attrs := []any{"error", err, "elapsed_ms", elapsedMS}
		if rows >= 0 {
			attrs = append(attrs, "rows", rows)
		}
		g.Logger.ErrorContext(ctx, "gorm sql failed", append(attrs, "sql", sql)...)
	case g.SlowThreshold != 0 && elapsed > g.SlowThreshold && g.LogLevel >= logger.Warn:
		sql, rows := fc()
		attrs := []any{"slow_threshold", g.SlowThreshold, "elapsed_ms", elapsedMS}
		if rows >= 0 {
			attrs = append(attrs, "rows", rows)
		}
		g.Logger.WarnContext(ctx, "gorm slow sql", append(attrs, "sql", sql)...)
	case g.LogLevel == logger.Info:
		sql, rows := fc()
		attrs := []any{"elapsed_ms", elapsedMS}
		if rows >= 0 {
			attrs = append(attrs, "rows", rows)
		}
		g.Logger.InfoContext(ctx, "gorm sql", append(attrs, "sql", sql)...)
	}
}

// gormConfig 是生产 gorm.Open 的配置单一来源（Init 与装配测试共用，防策略漂移）：
// Warn 级 + 200ms 慢查询阈值与原 gorm 默认 logger 同参；两个泄露闸恒开。
func gormConfig(slogger *slog.Logger) *gorm.Config {
	return &gorm.Config{
		Logger: newGormSlogLogger(slogger, logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	}
}
