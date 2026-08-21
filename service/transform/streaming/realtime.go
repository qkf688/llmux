package streaming

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

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

	// OpenAI Chat -> Responses: map OpenAI tool_call index to Responses output_index.
	// OpenAI 的 tool_call index 从 0 起，与 message 项的 output_index 会撞车，必须另行分配。
	openAIToolCallIndexToResponsesOutputIndex map[int]int

	unknownRouteCount int

	responseID string
	itemID     string

	// Responses 侧已产出的 output item，供 response.completed 的 output 数组使用。
	responsesOutput       responsesOutputItems
	messageOutputIndex    int
	hasMessageOutputIndex bool

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
	scanner := bufio.NewScanner(src)
	// Increase initial buffer to 64KB, and cap at maxSSEEventSize (avoid "token too long").
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)

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

	var eventName string
	var dataLines []string
	eventSize := 0

	flushEvent := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			eventSize = 0
			return nil
		}

		state.currentEvent = eventName
		data := strings.Join(dataLines, "\n")

		eventName = ""
		dataLines = dataLines[:0]
		eventSize = 0

		if data == "" {
			state.currentEvent = ""
			return nil
		}

		// 累积原始 SSE 字节流供日志记录 RawResponseBody（头尾双段保留，见 raw_accumulator.go）。
		accumulateRawSSE(state, data)

		if err := dispatchRealtimeStreamChunk(state, data); err != nil {
			return err
		}
		state.currentEvent = ""
		return nil
	}

	for scanner.Scan() {
		state.lineCount++
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			// blank line indicates end of a SSE event.
			if err := flushEvent(); err != nil {
				return err
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
			eventSize += len(data)
			if eventSize > maxSSEEventSize {
				return fmt.Errorf("sse event too large: %d > %d", eventSize, maxSSEEventSize)
			}
			dataLines = append(dataLines, data)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
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
			"lines_processed", state.lineCount,
			"errors_encountered", state.errorCount,
			"error", err)
		return err
	}

	// EOF but without trailing blank line: flush last event once.
	if err := flushEvent(); err != nil {
		// 这里不像 scanner.Err() 分支那样再尝试 finalize：该错误来自**写下游** pipe 失败，
		// finalize 同样是写下游，必然一起失败；而 scanner.Err() 是读上游失败、下游仍可写。
		slog.Error("failed to flush last SSE event in stream transformation",
			"upstream_format", state.upstreamFormat,
			"client_format", state.clientFormat,
			"lines_processed", state.lineCount,
			"errors_encountered", state.errorCount,
			"error", err)
		return err
	}

	// 上游可能不发 [DONE] 就直接 EOF，此时延后的事件（如等 usage 尾包的
	// response.completed）还压在 state 里，必须在此收尾，否则客户端收不到终态。
	if state.finalize != nil {
		if err := state.finalize(state); err != nil {
			slog.Error("failed to finalize stream transformation",
				"upstream_format", state.upstreamFormat,
				"client_format", state.clientFormat,
				"lines_processed", state.lineCount,
				"error", err)
			return err
		}
	}

	slog.Debug("stream transformation completed",
		"upstream_format", state.upstreamFormat,
		"client_format", state.clientFormat,
		"lines_processed", state.lineCount,
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
