package streaming

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

type realtimeStreamState struct {
	writer *io.PipeWriter

	upstreamFormat consts.WireFormat
	clientFormat   consts.WireFormat

	currentEvent string
	lineCount    int
	errorCount   int

	sequenceNumber int

	accumulatedText      string
	accumulatedReasoning string

	// rawAcc 可选地累积上游原始 SSE 字节流（含 event/data 前缀和 \n\n 分隔），
	// 供日志记录 RawResponseBody 用。nil 时不累积（非日志场景避免内存开销）。
	// 仅在转换 goroutine 内写入；goroutine 结束（pipe Close）后调用方才读取，无并发。
	rawAcc *rawSSEAccumulator

	// sideChannel 是转换旁路产物的出口：上游原始体与**上游原始 usage** 都经它
	// 交给落库侧。只有读上游原始流的那一跳持有它（多跳时第二跳为 nil），
	// 否则捕获到的会是中间格式而非上游真值。
	sideChannel *models.TransformSideChannel

	// OpenAI Chat streaming meta (used when the output format is OpenAI Chat).
	openAIID                                  string
	openAIModel                               string
	openAICreated                             int64
	openAIToolCallNextIndex                   int
	responsesOutputIndexToOpenAIToolCallIndex map[int]int

	// openAIToolCallBuffers 按 OpenAI tool_call index 缓冲「id/name 未齐备」的工具调用。
	//
	// 上游常把 id 与 name 分帧发送（甚至只发其一）：修复前 id 帧即发 output_item.added
	// 且 name 写死一次——id 帧缺 name 产出空名 tool_use（下游客户端报 No such tool
	// 并把脏块写回历史，会话被出站校验 400 卡死）；无 id 帧则整帧丢弃，工具调用凭空消失。
	// 改为 id+name 齐备才发 added，参数增量先进缓冲，流末兜底时有 name 无 id 补
	// call_ 前缀 id 发出、有 id 无 name 丢弃。
	//
	// 缓冲项自带 output_index（OpenAI 的 tool_call index 与 message 项的
	// output_index 会撞车，必须另行分配），故不再另建 index→output_index 映射表。
	openAIToolCallBuffers map[int]*openAIToolCallBuffer

	// openAIStreamMeta 缓存上游 OpenAI Chat chunk 里的 id / model / created。
	// 流末兜底收尾（上游没给 finish_reason）拿不到具体 chunk，
	// response.completed 的这三项只能取缓存值，否则会是空串 / 0。
	openAIStreamMeta responseStreamMeta

	// openAIOutputClosed 标记 output item 的收尾事件是否已发出。
	// Responses 协议里终态不可撤回：重复的 output_item.done 与 response.completed
	// 同属协议违规，故收尾一旦发出就不得再发第二套。
	openAIOutputClosed bool

	// openAIFinalized 标记 response.completed 是否已写出。
	// [DONE]、流末 EOF 兜底、上游读错误兜底三个出口都调同一个收尾函数，
	// 由它保证只交付一次。
	openAIFinalized bool

	unknownRouteCount int

	responseID string
	itemID     string

	// Responses 侧已产出的 output item，供 response.completed 的 output 数组使用。
	responsesOutput       responsesOutputItems
	messageOutputIndex    int
	hasMessageOutputIndex bool

	// messageStreamStarted 标记本流的 message 项是否已经开过（openai 线用）。
	// 部分 OpenAI 兼容上游会在末包重复带 role，重复开会发出第二套
	// response.created / output_item.added(message)，客户端看到的是同一
	// response id 与同一 item id 各出现两次——严格 Responses 客户端判协议异常。
	messageStreamStarted bool

	// responseStreamStarted 标记本流的 response.created / response.in_progress
	// 是否已发出（openai 线用）。与 messageStreamStarted 分开是因为两者
	// 的开启条件不同：流首事件与 role 无关（上游可能先给 reasoning / tool_calls），
	// message 项则要等 role 或 content。
	responseStreamStarted bool

	reasoningItemID      string
	hasReasoningItem     bool
	reasoningOutputIndex int

	// OpenAI 流式规范里 stream_options.include_usage 的 usage 是在 finish_reason
	// **之后**单独发一个 choices:[] 的尾包，而非与 finish_reason 同包。因此
	// response.completed 不能在看到 finish_reason 时就发出——那时读到的 usage 是
	// null，跨协议后 token 统计会全为 0（待办 #18）。
	//
	// pendingCompleted 缓存已构建好但尚未写出的 response.completed 事件，
	// pendingUsage 缓存最后见到的 OpenAI 格式 usage（可能来自 finish 包本身，
	// 也可能来自之后的尾包）。二者在 [DONE] 或流末合并后一次性写出。
	pendingCompleted map[string]interface{}
	pendingUsage     map[string]interface{}

	// Anthropic 把 usage 拆在两个事件里：message_start.message.usage 给 input 侧
	// （input_tokens / cache_read_input_tokens / cache_creation_input_tokens），
	// message_delta.usage 给 output_tokens——旧版上游的 message_delta 甚至不重复
	// 带 input_tokens。只读 message_delta 会让转换后的 input 侧 token 全为 0，
	// 故在此跨事件缓存 message_start 的 usage，供 message_delta 合并。
	anthropicStartUsage map[string]interface{}

	// finalize 在上游流结束（含 EOF 且无 [DONE] 的非规范上游）时被调用一次，
	// 用于 flush 上面的延后事件。仅在确有延后内容时由路由 handler 设置；
	// nil 表示该路由无需收尾。
	finalize func(*realtimeStreamState) error

	// Responses -> Anthropic: map output_index to Anthropic content_block index.
	anthropicActiveBlockIndex                 int
	anthropicNextBlockIndex                   int
	responsesOutputIndexToAnthropicBlockIndex map[int]int

	// Responses -> Anthropic 并行顺序化：Anthropic streaming 一次只允许一个 active
	// content_block，但 OpenAI Responses 的并行 tool_call / reasoning+content 是按
	// output_index 交错到达的。下列字段把非 active output_index 的 delta 缓冲起来，
	// 由 output_item.done 驱动 block 切换并 flush 缓冲，保证参数完整不丢。
	//
	// anthropicActiveOutputIndex：当前已开未关 block 对应的 output_index，-1 表示无。
	// anthropicItemMeta：output_index → item 元数据（type/id/name），item.added 时记录。
	// anthropicDeltaBuffer：output_index → 待 flush 的 delta 切片（非 active 时缓冲）。
	// anthropicDoneOutputIndices：已收到 output_item.done 的 output_index 集合。
	// anthropicStartedOutputIndices：已发出 content_block_start 的 output_index 集合。
	//   职责边界：responsesOutputIndexToAnthropicBlockIndex 管「block index 分配」（message
	//   在 item.added 时预分配但未 start），anthropicStartedOutputIndices 管「start 事件是否
	//   已发放」。两者生命周期强耦合但语义不同：image 打断时清映射（强制下次分配新 index）
	//   但不清 started（该 block 确实 start 过），形成「映射=无、started=true」中间态——
	//   这是有意为之，让 flushRemainingAnthropicBuffers 据此跳过已 start 的 item 不误补发。
	anthropicActiveOutputIndex    int
	anthropicItemMeta             map[int]anthropicItemMeta
	anthropicDeltaBuffer          map[int][]anthropicBufferedDelta
	anthropicDoneOutputIndices    map[int]bool
	anthropicStartedOutputIndices map[int]bool
}

// TransformResponseRealtime performs real-time streaming response conversion
// directly from the response Body reader.
//
// sideChannel 承载转换旁路产物（上游原始体 + 上游原始 usage）；nil 表示不需要旁路。
func TransformResponseRealtime(response *http.Response, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	// Reduce N×N streaming conversions to N+N by routing through OpenAI Responses
	// streaming format when neither side is already using it.
	//
	// provider -> openai-res (canonical) -> client
	if upstreamFormat != consts.FormatOpenAIResponses && clientFormat != consts.FormatOpenAIResponses {
		return transformResponseRealtimeViaResponses(response, upstreamFormat, clientFormat, sideChannel)
	}

	pr, pw := io.Pipe()

	go func() {
		defer response.Body.Close()
		err := transformStreamBodyRealtime(response.Body, pw, upstreamFormat, clientFormat, sideChannel)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        response.Header.Clone(),
		Body:          pr,
		ContentLength: -1,
	}

	return newResponse, nil
}

func transformResponseRealtimeViaResponses(response *http.Response, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	midReader, midWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	go func() {
		defer response.Body.Close()

		// 第一个 goroutine 读上游原始 body，持有 sideChannel（旁路捕获上游原始体与 usage）；
		// 第二个 goroutine 读中间格式，传 nil——否则捕获的会是中间格式而非上游真值。
		err := transformStreamBodyRealtime(response.Body, midWriter, upstreamFormat, consts.FormatOpenAIResponses, sideChannel)
		if err != nil {
			midWriter.CloseWithError(err)
			return
		}
		midWriter.Close()
	}()

	go func() {
		defer midReader.Close()

		err := transformStreamBodyRealtime(midReader, outWriter, consts.FormatOpenAIResponses, clientFormat, nil)
		if err != nil {
			outWriter.CloseWithError(err)
			return
		}
		outWriter.Close()
	}()

	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        response.Header.Clone(),
		Body:          outReader,
		ContentLength: -1,
	}

	return newResponse, nil
}

func transformStreamBodyRealtime(src io.ReadCloser, dst *io.PipeWriter, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) error {
	state := &realtimeStreamState{
		writer:                     dst,
		upstreamFormat:             upstreamFormat,
		clientFormat:               clientFormat,
		anthropicActiveBlockIndex:  -1,
		anthropicActiveOutputIndex: -1,
		sideChannel:                sideChannel,
	}

	// 累积结果在本函数所有返回路径上都要落到侧信道，故用 defer 统一 flush。
	// 注意：出错路径虽然也会 flush，但消费方 RecordLog 在 processer 出错时不读取
	// 累积体（见 service/chat/chat_record.go 错误分支），失败流的 RawResponseBody
	// 目前仍为空——此处只保证累积器侧不丢数据。
	if sideChannel.WantsRawBody() {
		state.rawAcc = &rawSSEAccumulator{}
		defer func() {
			body, dropped := state.rawAcc.finalize()
			sideChannel.AppendRawBody(body)
			if dropped > 0 {
				// 截断只影响日志字段、不影响转发，但必须留下运行时信号，
				// 否则运维不查 DB 就无从知晓发生过截断。
				slog.Warn("raw SSE log truncated, middle section dropped",
					"dropped_bytes", dropped,
					"head_limit", rawAccumulatorHeadSize,
					"tail_limit", rawAccumulatorTailSize,
					"upstream_format", upstreamFormat,
					"client_format", clientFormat,
				)
			}
		}()
	}

	// 分帧走全库唯一判据（common.ScanSSEFrames）：「测试判通过的流」与「网关实际
	// 处理的流」从此是同一份代码，不再靠注释约定同步。
	var dispatchedInCallbackErr bool
	handleFrame := func(frame common.SSEFrame, stats common.SSEScanStats) error {
		state.lineCount = stats.Lines
		data := frame.Data
		if data == "" {
			state.currentEvent = ""
			return nil
		}
		// 单事件体积守卫（R5：从按行累计改为按帧交付长度判定，阈值与文案不变）。
		if len(data) > maxSSEEventSize {
			return fmt.Errorf("sse event too large: %d > %d", len(data), maxSSEEventSize)
		}

		state.currentEvent = frame.Event

		// 累积原始 SSE 字节流供日志记录 RawResponseBody（头尾双段保留，见 raw_accumulator.go）。
		accumulateRawSSE(state, data)

		if err := dispatchRealtimeStreamChunk(state, data); err != nil {
			dispatchedInCallbackErr = true
			return err
		}
		state.currentEvent = ""
		return nil
	}

	stats, scanErr := common.ScanSSEFrames(src, common.SSEScanConfig{MaxLineBytes: maxSSEEventSize}, handleFrame)
	if dispatchedInCallbackErr {
		// 错误来自写下游 pipe 失败或事件超预算：finalize 同样是写下游，必然一起失败，
		// 不做补救交付（与旧实现一致）。
		return scanErr
	}
	if scanErr != nil {
		// 读上游失败，但下游 pipe 仍可写：尽力把延后的终态事件交付出去，
		// 否则「finish 之后、流末之前断流」这一窗口内客户端连 completed 都收不到
		// （旧实现在 finish 时立即发出，此处保持同等的尽力交付语义）。
		if state.finalize != nil {
			if ferr := state.finalize(state); ferr != nil {
				slog.Warn("failed to deliver pending completion after upstream read error",
					"upstream_format", state.upstreamFormat,
					"client_format", state.clientFormat,
					"error", ferr)
			}
		}
		slog.Error("scanner error in stream transformation",
			"upstream_format", state.upstreamFormat,
			"client_format", state.clientFormat,
			"lines_processed", stats.Lines,
			"errors_encountered", state.errorCount,
			"error", scanErr)
		return scanErr
	}

	// 上游可能不发 [DONE] 就直接 EOF，此时延后的事件（如等 usage 尾包的
	// response.completed）还压在 state 里，必须在此收尾，否则客户端收不到终态。
	// 末帧交付已含在 ScanSSEFrames 的 EOF flush 语义内。
	if state.finalize != nil {
		if err := state.finalize(state); err != nil {
			slog.Error("failed to finalize stream transformation",
				"upstream_format", state.upstreamFormat,
				"client_format", state.clientFormat,
				"lines_processed", stats.Lines,
				"error", err)
			return err
		}
	}

	slog.Debug("stream transformation completed",
		"upstream_format", state.upstreamFormat,
		"client_format", state.clientFormat,
		"lines_processed", stats.Lines,
		"errors_encountered", state.errorCount)
	return nil
}

func dispatchRealtimeStreamChunk(state *realtimeStreamState, data string) error {
	if h, ok := lookupRealtimeRoute(state.upstreamFormat, state.clientFormat); ok {
		return h(state, data)
	}
	// Keep behavior: passthrough. Add observability for unexpected routes.
	if state.unknownRouteCount < 3 {
		state.unknownRouteCount++
		slog.Warn("unknown stream transform route, passthrough",
			"upstream_format", state.upstreamFormat,
			"client_format", state.clientFormat,
			"event", state.currentEvent,
			"line", state.lineCount,
			"data_length", len(data),
			"data_preview", func() string {
				if len(data) > 120 {
					return data[:120]
				}
				return data
			}(),
		)
	}
	return writeRealtimeData(state, data)
}

func writeRealtimeData(state *realtimeStreamState, data string) error {
	_, err := io.WriteString(state.writer, "data: "+data+"\n\n")
	return err
}

func logRealtimeWriteError(state *realtimeStreamState, err error) {
	slog.Error("failed to write to pipe in stream transformation",
		"upstream_format", state.upstreamFormat,
		"client_format", state.clientFormat,
		"line", state.lineCount,
		"error", err)
}

func logRealtimeChunkParseError(state *realtimeStreamState, data string, err error) {
	state.errorCount++
	slog.Error("failed to parse SSE chunk in stream transformation",
		"upstream_format", state.upstreamFormat,
		"client_format", state.clientFormat,
		"line", state.lineCount,
		"data_length", len(data),
		"error", err,
		"data_preview", func() string {
			if len(data) > 100 {
				return data[:100]
			}
			return data
		}())
}

// nextRealtimeSequence 取下一个 Responses 事件序号。
//
// 不变量：**取号顺序必须等于写出顺序**。序号是流内单调计数，没有重排缓冲
// （writeRealtimeOrderedData 只负责把 payload["type"] 同时写成 SSE event: 行，
// 与排序无关）。因此延后写出的事件必须延后取号——在构建时取号、写出时才发，
// 会让它的序号小于中间插入的事件，造成错序。
func nextRealtimeSequence(state *realtimeStreamState) int {
	seq := state.sequenceNumber
	state.sequenceNumber++
	return seq
}
