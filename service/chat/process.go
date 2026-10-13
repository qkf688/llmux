package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"
	"time"

	"github.com/qkf688/llmux/common"
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

type Processer func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error)

// SSEEvent 流式事件的日志侧名字；分帧判据单源于 common.ScanSSEFrames，
// 此别名保住 processerConfig.streamUsageExtract 的既有签名。
type SSEEvent = common.SSEFrame

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
// 首包时间与 usage 解析无条件执行：它们分别喂日志页的性能列与真实模型/供应商的 token
// 计量，曾被两个设置项做成可关，收益在纳秒级、代价是把计量打成 0（已删除那两个开关）。
func createProcesser(cfg processerConfig) Processer {
	if cfg.nonStreamUsagePath == "" {
		cfg.nonStreamUsagePath = "usage"
	}
	return func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
		var firstChunkTime time.Duration
		var once sync.Once

		var usageStr string
		var output models.OutputUnion

		chunkCount := 0

		// Stage 23：流式累积预算闸——OfStringArray 无上限累积会把内存与 ChatIO
		// 落库体积双双撑爆（长流 × 并发），超限产出头/标记/尾三 chunk。
		// 预算在 detached ctx 读（与 rawLogFieldLimit 同口径，防死 ctx 退默认值）。
		var outputAcc *outputBudgetAccumulator
		if stream {
			withDetachedLogCtx(ctx, func(dctx context.Context) {
				outputAcc = newOutputBudgetAccumulator(getResponseLogMaxBytes(dctx))
			})
		}
		if !stream {
			scanner := bufio.NewScanner(pr)
			scanner.Buffer(make([]byte, 0, InitScannerBufferSize), MaxScannerBufferSize)
			for chunk := range ScannerToken(scanner) {
				once.Do(func() {
					firstChunkTime = time.Since(start)
				})

				output.OfString = chunk
				usageStr = gjson.Get(chunk, cfg.nonStreamUsagePath).String()
				break
			}
			if err := scanner.Err(); err != nil {
				return nil, nil, err
			}
		} else {
			// 分帧走全库唯一判据（common.ScanSSEFrames），与网关转发读的是同一份代码。
			// 终止符用哨兵错误触发提前停：它不是失败，判定归调用方收敛。
			streamDone := errors.New("sse stream done")
			_, scanErr := common.ScanSSEFrames(pr, common.SSEScanConfig{MaxLineBytes: MaxScannerBufferSize},
				func(ev SSEEvent, _ common.SSEScanStats) error {
					once.Do(func() {
						firstChunkTime = time.Since(start)
					})

					chunk := ev.Data
					if cfg.streamDoneTerminator != "" && chunk == cfg.streamDoneTerminator {
						return streamDone
					}

					if cfg.enableErrorCheck {
						chunkCount++
						if chunkCount <= MaxErrorCheckChunks {
							errStr := gjson.Get(chunk, "error")
							if errStr.Exists() {
								return errors.New(errStr.String())
							}
						}
					}

					if chunk != "" {
						outputAcc.write(chunk)
					}

					if usageStr == "" && cfg.streamUsageExtract != nil {
						usageStr = cfg.streamUsageExtract(ev)
					}
					return nil
				})
			if scanErr != nil && !errors.Is(scanErr, streamDone) {
				return nil, nil, scanErr
			}
		}

		if outputAcc != nil {
			chunks, _ := outputAcc.flush()
			output.OfStringArray = chunks
		}

		usage, err := parseUsageJSON(usageStr)
		if err != nil {
			return nil, nil, err
		}

		// 分子取 completion_tokens（TPS 语义 = 生成速度），不用 TotalTokens：后者含
		// prompt，长上下文下算出来的不是速度而是「输入有多少」的假象——现场一条
		// prompt 48 万 / 生成 234 的请求，用 total 做分子把数值放大了约 2000 倍。
		//
		// 注意分母仍不可信：chunkTime 是「首帧到达 → 流读完」窗口，上游成批下发
		// （或对 stream 请求回整体 body）时该窗口塌到亚毫秒级，商照样爆表；此处的
		// `> 0` 只防除零、不表达「窗口小到测不出速度」。窗口的可测性判据另案，别把
		// 本行读成「TPS 已经准了」。
		chunkTime := time.Since(start) - firstChunkTime
		var tps float64
		if chunkTime.Seconds() > 0 {
			tps = float64(usage.CompletionTokens) / chunkTime.Seconds()
		}

		return &models.ChatLog{
			FirstChunkTime: firstChunkTime,
			ChunkTime:      chunkTime,
			Usage:          usage,
			Tps:            tps,
		}, &output, nil
	}
}

func ProcesserOpenAI(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	return processerOpenAI(ctx, pr, stream, start)
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

func ProcesserOpenAiRes(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	return processerOpenAiRes(ctx, pr, stream, start)
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

func ProcesserAnthropic(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	return processerAnthropic(ctx, pr, stream, start)
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
	// 与 before.go 的注册同源：键为裸 string，见那里的说明。
	RegisterProcesser(string(consts.StyleOpenAI), ProcesserOpenAI)
	RegisterProcesser(string(consts.StyleOpenAIRes), ProcesserOpenAiRes)
	RegisterProcesser(string(consts.StyleAnthropic), ProcesserAnthropic)
}
