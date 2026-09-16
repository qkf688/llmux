// Package common 里的 SSE 读侧判据（本文件）是全库「什么是一条 SSE 事件」的
// 唯一 owner：探针（service/credprobe）、网关转发（service/transform/streaming）、
// chat 日志（service/chat）三处读流必须走同一判据，否则「测试判通过的流」与
// 「网关实际处理的流」会分裂成两套结论——历史上这里是三份手写循环，
// 漂移窗口永存，故收敛为单实现（见 AGENTS.md 4.4 DRY）。

package common

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// SSEContentType 是流式判定的媒体类型字面量，写头侧（handler/v1）亦可引用，
// 保证「声明流式」与「判定流式」同源。
const SSEContentType = "text/event-stream"

// IsEventStreamContentType 判定响应是否按 SSE 回。判据是 Contains（非 HasPrefix）：
// 网关转发链路按 Contains 分流，探针若用别的判据就会出现「网关按流处理、
// 测试判不支持流」的分裂；参数（charset 等）出现在媒体类型之前/之后都算流式。
func IsEventStreamContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), SSEContentType)
}

// SSEFrame 一个已分帧的 SSE 事件。Event 取自 event: 行（OpenAI Chat 通常为空，
// Anthropic / OpenAI Responses 有值）；Data 是该事件内所有 data: 行按 SSE 规范以 \n 合并的载荷。
//
// 载荷逐字节原样保留、不做反序列化再序列化：折叠层依赖 delta 字段名，任何「顺手规范化」
// 都会静默改变折叠结果。
type SSEFrame struct {
	Event string
	Data  string
}

// SSEScanConfig 是调用方的读取预算策略。预算数值本身归各调用方（探针止损、
// 网关防畸形、日志防放大），内核只承载分帧语义；零值字段 = 不限。
type SSEScanConfig struct {
	// MaxLineBytes 单行上限（bufio buffer cap）。超限表现为 bufio.ErrTooLong，
	// 与总/帧预算的 *SSEBudgetError 区分：前者是「读流失败」，后者是「主动止损」。
	MaxLineBytes int
	// MaxTotalBytes 整条流的消费字节预算（按行累计，含换行符）。
	MaxTotalBytes int
	// MaxFrames 帧数预算。字节预算封不住「每帧只占几字节」的小帧洪水，
	// 判定输入的常驻内存需要单独的帧数闸门。
	MaxFrames int
}

// SSEScanStats 扫描进度计数。预算/回调错误路径上返回的是「截至中止时」的值，
// 供调用方写诊断日志（如「已读到 N 帧」）。
type SSEScanStats struct {
	Lines  int
	Frames int
}

// SSEBudgetKind 区分两种主动止损的预算维度。
type SSEBudgetKind int

const (
	SSEBudgetBytes  SSEBudgetKind = iota + 1 // 消费字节超 MaxTotalBytes
	SSEBudgetFrames                          // 已交付帧数达 MaxFrames
)

// SSEBudgetError 表示「上游超预算，主动止损」，与读流失败互斥。
// 错误文案沿用历史探针口径（「字节数超过 N 预算」），调用方的 Detail 措辞因此零改动。
type SSEBudgetError struct {
	Kind  SSEBudgetKind
	Limit int
}

func (e *SSEBudgetError) Error() string {
	switch e.Kind {
	case SSEBudgetBytes:
		return fmt.Sprintf("字节数超过 %d 预算", e.Limit)
	default:
		return fmt.Sprintf("帧数超过 %d 预算", e.Limit)
	}
}

// defaultSSEMaxLineBytes 是 cfg.MaxLineBytes 未设时的行上限。三个现存调用方都显式
// 传值，此默认只兜新调用方忘设的场合（宁可得罪畸形上游，不可无界缓冲）。
const defaultSSEMaxLineBytes = 1 << 20

// ScanSSEFrames 按 SSE 规范从 src 分帧读取，每交付一帧调用一次 onFrame（携带当时
// 的统计快照，供调用方记日志）。无副作用、不解析 JSON：纯分帧。
//
// 语义（即全库唯一判据）：
//   - 空行交付当前事件；多行 data: 以 \n 合并；event: 行记名（TrimSpace），交付后重置；
//     ":" 开头为注释行；id:/retry: 等其余字段与裸行忽略。
//   - data: 载荷只去前导**一个**空格（SSE 规范），不去行尾空白、不 TrimSpace——
//     载荷要喂给逐字节比对帧的折叠层与透传路由，任何多余修剪都是失真。
//   - EOF 且末帧无分隔空行时仍交付，否则 message_stop 这类收尾事件会凭空消失。
//   - onFrame 返回错误立即停止并把该错误**原样**返回（调用方用自己的哨兵区分
//     「正常读完提前停」与「真错误」，如 [DONE] 终止符）。
//   - 返回值约定：预算超限 → (stats, *SSEBudgetError)；读失败（含行超 bufio cap 的
//     bufio.ErrTooLong）→ (stats, err)；正常读完（含 onFrame 提前停）→ (stats, onFrame 错误或 nil)。
func ScanSSEFrames(src io.Reader, cfg SSEScanConfig, onFrame func(SSEFrame, SSEScanStats) error) (SSEScanStats, error) {
	maxLineBytes := cfg.MaxLineBytes
	if maxLineBytes <= 0 {
		maxLineBytes = defaultSSEMaxLineBytes
	}
	scanner := bufio.NewScanner(src)
	// 初缓冲 64KB：合法探针/转发帧远小于此，避免逐行扩容；上限由 maxLineBytes 封死。
	// 初缓冲不能超过 maxLineBytes：bufio 只在「缓冲写满仍未见换行」时判定超限，
	// 若初始缓冲（按 cap 取整段长度）比上限大，小于缓冲的行就永远逃过 ErrTooLong。
	scanner.Buffer(make([]byte, 0, min(64*1024, maxLineBytes)), maxLineBytes)

	var stats SSEScanStats
	var eventName string
	dataLines := make([]string, 0, 4)
	totalBytes := 0

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		frame := SSEFrame{Event: eventName, Data: strings.Join(dataLines, "\n")}
		eventName = ""
		dataLines = dataLines[:0]
		stats.Frames++
		return onFrame(frame, stats)
	}

	for scanner.Scan() {
		rawLine := scanner.Bytes()
		stats.Lines++
		totalBytes += len(rawLine) + 1 // +1 计入换行符，与「消费字节数」同口径
		if cfg.MaxTotalBytes > 0 && totalBytes > cfg.MaxTotalBytes {
			return stats, &SSEBudgetError{Kind: SSEBudgetBytes, Limit: cfg.MaxTotalBytes}
		}
		if cfg.MaxFrames > 0 && stats.Frames >= cfg.MaxFrames {
			return stats, &SSEBudgetError{Kind: SSEBudgetFrames, Limit: cfg.MaxFrames}
		}

		line := strings.TrimRight(string(rawLine), "\r")
		switch {
		case line == "":
			if err := flush(); err != nil {
				return stats, err
			}
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, ":"):
			// SSE 注释行（Anthropic 的 ping/keep-alive 常见）：不成帧。
		case strings.HasPrefix(line, "data:"):
			value := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if value == "" {
				continue
			}
			dataLines = append(dataLines, value)
		}
		// 其余行（无冒号的裸行、id:/retry: 等字段）对判定无信息量，忽略。
	}
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	if err := flush(); err != nil {
		return stats, err
	}
	return stats, nil
}
