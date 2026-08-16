package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

const (
	InitScannerBufferSize     = 1024 * 8         // 8KB
	MaxScannerBufferSize      = 1024 * 1024 * 15 // 15MB
	MaxErrorCheckChunks       = 5                // 只检查前5个chunk的错误，优化性能
	DefaultChunkArrayCapacity = 128              // 预分配chunk数组容量，减少扩容
)

type Processer func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error)

type SSEEvent struct {
	Event string
	Data  string
}

func ScanSSEEvents(reader *bufio.Scanner) iter.Seq[SSEEvent] {
	return func(yield func(SSEEvent) bool) {
		var eventName string
		dataLines := make([]string, 0, 4)

		flush := func() bool {
			if len(dataLines) == 0 {
				eventName = ""
				return true
			}

			data := strings.Join(dataLines, "\n")
			dataLines = dataLines[:0]

			ev := SSEEvent{Event: eventName, Data: data}
			eventName = ""

			return yield(ev)
		}

		for reader.Scan() {
			line := strings.TrimRight(reader.Text(), "\r")
			if line == "" {
				if !flush() {
					return
				}
				continue
			}

			if after, ok := strings.CutPrefix(line, "event:"); ok {
				eventName = strings.TrimSpace(after)
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			if after, ok := strings.CutPrefix(line, "data:"); ok {
				data := strings.TrimSpace(after)
				if data == "" {
					continue
				}
				dataLines = append(dataLines, data)
				continue
			}
		}

		_ = flush()
	}
}

// processerConfig 配置各协议 Processer 的差异点，供 createProcesser 使用。
type processerConfig struct {
	// nonStreamUsagePath 非流式模式下 usage 的 gjson 路径（默认 "usage"）。
	nonStreamUsagePath string
	// streamDoneTerminator 流式终止符（如 OpenAI 的 "[DONE]"）；空表示无终止符。
	streamDoneTerminator string
	// enableErrorCheck 是否启用前 MaxErrorCheckChunks 个 chunk 的错误检查（仅 OpenAI）。
	enableErrorCheck bool
	// streamUsageExtract 从流式 chunk 中提取 usage 字符串。
	// 返回非空字符串表示找到 usage。仅在 usageStr == "" 时调用。
	streamUsageExtract func(ev SSEEvent) string
}

// parseUsageJSON 把上游 usage 的 JSON 片段解析并归一为 models.Usage。
//
// 三个协议共用一份：usage 的形状差异（input_tokens vs prompt_tokens、
// 嵌套 details vs 顶层字段、anthropic 与 openai 兼容字段并存）全部由
// models.UsageFromMap 的候选路径表吸收，故这里不是 per-protocol 配置项——
// 新增一种上游写法只该往候选表加一行，不该在此再分叉一份解析（OCP/DRY）。
//
// usageStr 非合法 JSON（含空串，即上游未给 usage）时返回零值 Usage 与 nil error，
// 与「上游明确报 0」同为零值：落库侧靠 UsageSource 区分可信度，不在此处编造。
func parseUsageJSON(usageStr string) (models.Usage, error) {
	raw := []byte(usageStr)
	if !json.Valid(raw) {
		return models.Usage{}, nil
	}
	var usage map[string]interface{}
	if err := json.Unmarshal(raw, &usage); err != nil {
		return models.Usage{}, fmt.Errorf("parse upstream usage: %w", err)
	}
	return models.UsageFromMap(usage), nil
}

// createProcesser 根据配置创建一个 Processer，统一 stream/non-stream、首包时间、TPS 逻辑。
func createProcesser(cfg processerConfig) Processer {
	if cfg.nonStreamUsagePath == "" {
		cfg.nonStreamUsagePath = "usage"
	}
	return func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		var firstChunkTime time.Duration
		var once sync.Once

		var usageStr string
		var output models.OutputUnion

		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, InitScannerBufferSize), MaxScannerBufferSize)
		chunkCount := 0

		if stream {
			output.OfStringArray = make([]string, 0, DefaultChunkArrayCapacity)
		}

		if !stream {
			for chunk := range ScannerToken(scanner) {
				if !disablePerformanceTracking {
					once.Do(func() {
						firstChunkTime = time.Since(start)
					})
				}

				output.OfString = chunk
				if !disableTokenCounting {
					usageStr = gjson.Get(chunk, cfg.nonStreamUsagePath).String()
				}
				break
			}
		} else {
			for ev := range ScanSSEEvents(scanner) {
				if !disablePerformanceTracking {
					once.Do(func() {
						firstChunkTime = time.Since(start)
					})
				}

				chunk := ev.Data
				if cfg.streamDoneTerminator != "" && chunk == cfg.streamDoneTerminator {
					break
				}

				if cfg.enableErrorCheck {
					chunkCount++
					if chunkCount <= MaxErrorCheckChunks {
						errStr := gjson.Get(chunk, "error")
						if errStr.Exists() {
							return nil, nil, errors.New(errStr.String())
						}
					}
				}

				if chunk != "" {
					output.OfStringArray = append(output.OfStringArray, chunk)
				}

				if !disableTokenCounting && usageStr == "" && cfg.streamUsageExtract != nil {
					usageStr = cfg.streamUsageExtract(ev)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, nil, err
		}

		var usage models.Usage
		if !disableTokenCounting {
			u, err := parseUsageJSON(usageStr)
			if err != nil {
				return nil, nil, err
			}
			usage = u
		}

		var chunkTime time.Duration
		var tps float64
		if !disablePerformanceTracking {
			chunkTime = time.Since(start) - firstChunkTime
			if chunkTime.Seconds() > 0 {
				tps = float64(usage.TotalTokens) / chunkTime.Seconds()
			}
		}

		return &models.ChatLog{
			FirstChunkTime: firstChunkTime,
			ChunkTime:      chunkTime,
			Usage:          usage,
			Tps:            tps,
		}, &output, nil
	}
}

func ProcesserOpenAI(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return processerOpenAI(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

var processerOpenAI = createProcesser(processerConfig{
	nonStreamUsagePath:   "usage",
	streamDoneTerminator: "[DONE]",
	enableErrorCheck:     true,
	streamUsageExtract: func(ev SSEEvent) string {
		usage := gjson.Get(ev.Data, "usage")
		if usage.Exists() && usage.Get("total_tokens").Int() != 0 {
			return usage.String()
		}
		return ""
	},
})

func ProcesserOpenAiRes(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return processerOpenAiRes(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

var processerOpenAiRes = createProcesser(processerConfig{
	nonStreamUsagePath: "usage",
	streamUsageExtract: func(ev SSEEvent) string {
		if ev.Event == "response.completed" {
			return gjson.Get(ev.Data, "response.usage").String()
		}
		return ""
	},
})

func ProcesserAnthropic(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
	return processerAnthropic(ctx, pr, stream, start, disablePerformanceTracking, disableTokenCounting)
}

var processerAnthropic = createProcesser(processerConfig{
	nonStreamUsagePath: "usage",
	streamUsageExtract: func(ev SSEEvent) string {
		if ev.Event == "message_delta" {
			return gjson.Get(ev.Data, "usage").String()
		}
		return ""
	},
})

func ScannerToken(reader *bufio.Scanner) iter.Seq[string] {
	return func(yield func(string) bool) {
		for reader.Scan() {
			chunk := reader.Text()
			if chunk == "" {
				continue
			}
			if !yield(chunk) {
				return
			}
		}
	}
}

func init() {
	RegisterProcesser(consts.StyleOpenAI, ProcesserOpenAI)
	RegisterProcesser(consts.StyleOpenAIRes, ProcesserOpenAiRes)
	RegisterProcesser(consts.StyleAnthropic, ProcesserAnthropic)
}
