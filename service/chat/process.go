package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	// parseUsage 将 usage JSON 字符串解析为 models.Usage。
	parseUsage func(usageStr string) (models.Usage, error)
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
			u, err := cfg.parseUsage(usageStr)
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
	parseUsage: func(usageStr string) (models.Usage, error) {
		var u models.Usage
		usage := []byte(usageStr)
		if json.Valid(usage) {
			if err := json.Unmarshal(usage, &u); err != nil {
				return models.Usage{}, err
			}
		}
		return u, nil
	},
})

type OpenAIResUsage struct {
	InputTokens        int64              `json:"input_tokens"`
	OutputTokens       int64              `json:"output_tokens"`
	TotalTokens        int64              `json:"total_tokens"`
	InputTokensDetails InputTokensDetails `json:"input_tokens_details"`
}

type InputTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
}

type AnthropicUsage struct {
	InputTokens              int64  `json:"input_tokens"`
	CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	ServiceTier              string `json:"service_tier"`
	// OpenAI 兼容字段（某些提供商如 kimi 会同时返回）
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	CachedTokens     int64 `json:"cached_tokens"`
}

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
	parseUsage: func(usageStr string) (models.Usage, error) {
		var u OpenAIResUsage
		usage := []byte(usageStr)
		if json.Valid(usage) {
			if err := json.Unmarshal(usage, &u); err != nil {
				return models.Usage{}, err
			}
		}
		return models.Usage{
			PromptTokens:     u.InputTokens,
			CompletionTokens: u.OutputTokens,
			TotalTokens:      u.TotalTokens,
			PromptTokensDetails: models.PromptTokensDetails{
				CachedTokens: u.InputTokensDetails.CachedTokens,
			},
		}, nil
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
	parseUsage: func(usageStr string) (models.Usage, error) {
		var u AnthropicUsage
		usage := []byte(usageStr)
		if json.Valid(usage) {
			if err := json.Unmarshal(usage, &u); err != nil {
				return models.Usage{}, err
			}
		}
		// 某些 OpenAI 兼容供应商（如 kimi）走 anthropic 直通时只回填 openai 兼容字段
		// （prompt_tokens / completion_tokens / cached_tokens），不给 anthropic 原生字段。
		// 缺原生字段时回退到兼容字段，避免这类上游 token 统计恒为 0。
		prompt := u.InputTokens
		if prompt == 0 {
			prompt = u.PromptTokens
		}
		completion := u.OutputTokens
		if completion == 0 {
			completion = u.CompletionTokens
		}
		cached := u.CacheReadInputTokens
		if cached == 0 {
			cached = u.CachedTokens
		}
		// total 口径仍为 prompt+completion（不含 Anthropic cache token），与全仓一致。
		totalTokens := prompt + completion
		return models.Usage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      totalTokens,
			PromptTokensDetails: models.PromptTokensDetails{
				CachedTokens: cached,
			},
		}, nil
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
