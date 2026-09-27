package common

// 体积防御类上限单源（Stage 26）：日志行/字段的字节上限走代码常量不做设置
// （consts/limits.go 口径：可被调小的防线不是防线）。此前的 accessLogLineMaxBytes
//（middleware，4 KiB）是首例；新增 slog 侧 path attr 上限与之同族同值。

// SlogPathMaxBytes slog 结构化日志中 path attr 的单值上限（recovery 与 httpresp
// 的 500 留痕两处同源）：病态长 URL（仅受 net/http MaxHeaderBytes ~1 MiB 兜底）
// 触发 panic/500 时，无上限会把 MiB 级 path 写进日志行。
const SlogPathMaxBytes = 4 << 10
