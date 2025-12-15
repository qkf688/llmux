package service

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

	"github.com/atopos31/llmio/models"
	"github.com/tidwall/gjson"
)

const (
	InitScannerBufferSize    = 1024 * 8         // 8KB
	MaxScannerBufferSize     = 1024 * 1024 * 15 // 15MB
	MaxErrorCheckChunks      = 5                // 只检查前5个chunk的错误，优化性能
	DefaultChunkArrayCapacity = 128             // 预分配chunk数组容量，减少扩容
)

type Processer func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error)

func ProcesserOpenAI(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	// 首字时延
	var firstChunkTime time.Duration
	var once sync.Once

	var usageStr string
	var output models.OutputUnion

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, InitScannerBufferSize), MaxScannerBufferSize)
	chunkCount := 0

	// 优化3: 预分配切片容量，减少扩容开销
	if stream {
		output.OfStringArray = make([]string, 0, DefaultChunkArrayCapacity)
	}

	for chunk := range ScannerToken(scanner) {
		once.Do(func() {
			firstChunkTime = time.Since(start)
		})
		if !stream {
			output.OfString = chunk
			usageStr = gjson.Get(chunk, "usage").String()
			break
		}

		// 优化2: 使用 CutPrefix 避免不必要的字符串分配
		var ok bool
		chunk, ok = strings.CutPrefix(chunk, "data: ")
		if !ok {
			continue
		}

		if chunk == "[DONE]" {
			break
		}

		// 性能优化：只检查前几个chunk的错误
		// 大多数错误（认证、限流、参数错误）都在开始阶段返回
		chunkCount++
		if chunkCount <= MaxErrorCheckChunks {
			errStr := gjson.Get(chunk, "error")
			if errStr.Exists() {
				return nil, nil, errors.New(errStr.String())
			}
		}

		output.OfStringArray = append(output.OfStringArray, chunk)

		// 优化1: 只在还没找到usage时才查询，避免重复查询
		if usageStr == "" {
			usage := gjson.Get(chunk, "usage")
			if usage.Exists() && usage.Get("total_tokens").Int() != 0 {
				usageStr = usage.String()
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// token用量
	var openaiUsage models.Usage
	usage := []byte(usageStr)
	if json.Valid(usage) {
		if err := json.Unmarshal(usage, &openaiUsage); err != nil {
			return nil, nil, err
		}
	}

	chunkTime := time.Since(start) - firstChunkTime

	// 计算 TPS，避免除零错误
	var tps float64
	if chunkTime.Seconds() > 0 {
		tps = float64(openaiUsage.TotalTokens) / chunkTime.Seconds()
	}

	return &models.ChatLog{
		FirstChunkTime: firstChunkTime,
		ChunkTime:      chunkTime,
		Usage:          openaiUsage,
		Tps:            tps,
	}, &output, nil
}

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

func ProcesserOpenAiRes(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	// 首字时延
	var firstChunkTime time.Duration
	var once sync.Once

	var usageStr string
	var output models.OutputUnion

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, InitScannerBufferSize), MaxScannerBufferSize)
	var event string

	// 优化: 预分配切片容量，减少扩容开销
	if stream {
		output.OfStringArray = make([]string, 0, DefaultChunkArrayCapacity)
	}

	for chunk := range ScannerToken(scanner) {
		once.Do(func() {
			firstChunkTime = time.Since(start)
		})
		if !stream {
			output.OfString = chunk
			usageStr = gjson.Get(chunk, "usage").String()
			break
		}

		if after, ok := strings.CutPrefix(chunk, "event: "); ok {
			event = after
			continue
		}

		// 优化: 使用 CutPrefix 替代 TrimPrefix
		content, ok := strings.CutPrefix(chunk, "data: ")
		if !ok || content == "" {
			continue
		}

		output.OfStringArray = append(output.OfStringArray, content)

		// 优化: 只在特定事件时查询usage，避免重复查询
		if usageStr == "" && event == "response.completed" {
			usageStr = gjson.Get(content, "response.usage").String()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	var openAIResUsage OpenAIResUsage
	usage := []byte(usageStr)
	if json.Valid(usage) {
		if err := json.Unmarshal(usage, &openAIResUsage); err != nil {
			return nil, nil, err
		}
	}

	chunkTime := time.Since(start) - firstChunkTime

	// 计算 TPS，避免除零错误
	var tps float64
	if chunkTime.Seconds() > 0 {
		tps = float64(openAIResUsage.TotalTokens) / chunkTime.Seconds()
	}

	return &models.ChatLog{
		FirstChunkTime: firstChunkTime,
		ChunkTime:      chunkTime,
		Usage: models.Usage{
			PromptTokens:     openAIResUsage.InputTokens,
			CompletionTokens: openAIResUsage.OutputTokens,
			TotalTokens:      openAIResUsage.TotalTokens,
			PromptTokensDetails: models.PromptTokensDetails{
				CachedTokens: openAIResUsage.InputTokensDetails.CachedTokens,
			},
		},
		Tps: tps,
	}, &output, nil
}

func ProcesserAnthropic(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	// 首字时延
	var firstChunkTime time.Duration
	var once sync.Once

	var usageStr string

	var output models.OutputUnion

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, InitScannerBufferSize), MaxScannerBufferSize)
	var event string

	// 优化: 预分配切片容量，减少扩容开销
	if stream {
		output.OfStringArray = make([]string, 0, DefaultChunkArrayCapacity)
	}

	for chunk := range ScannerToken(scanner) {
		once.Do(func() {
			firstChunkTime = time.Since(start)
		})
		if !stream {
			output.OfString = chunk
			usageStr = gjson.Get(chunk, "usage").String()
			break
		}

		if after, ok := strings.CutPrefix(chunk, "event: "); ok {
			event = after
			continue
		}

		after, ok := strings.CutPrefix(chunk, "data: ")
		if !ok {
			continue
		}

		output.OfStringArray = append(output.OfStringArray, after)

		// 优化: 只在特定事件时查询usage，避免重复查询
		if usageStr == "" && event == "message_delta" {
			usageStr = gjson.Get(after, "usage").String()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	var athropicUsage AnthropicUsage
	usage := []byte(usageStr)
	if json.Valid(usage) {
		if err := json.Unmarshal(usage, &athropicUsage); err != nil {
			return nil, nil, err
		}
	}

	chunkTime := time.Since(start) - firstChunkTime
	totalTokens := athropicUsage.InputTokens + athropicUsage.OutputTokens

	// 计算 TPS，避免除零错误
	var tps float64
	if chunkTime.Seconds() > 0 {
		tps = float64(totalTokens) / chunkTime.Seconds()
	}

	return &models.ChatLog{
		FirstChunkTime: firstChunkTime,
		ChunkTime:      chunkTime,
		Usage: models.Usage{
			PromptTokens:     athropicUsage.InputTokens,
			CompletionTokens: athropicUsage.OutputTokens,
			TotalTokens:      totalTokens,
			PromptTokensDetails: models.PromptTokensDetails{
				CachedTokens: athropicUsage.CacheReadInputTokens,
			},
		},
		Tps: tps,
	}, &output, nil
}

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
