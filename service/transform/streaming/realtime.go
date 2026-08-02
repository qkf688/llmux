package streaming

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type realtimeStreamState struct {
	writer *io.PipeWriter

	providerType string
	clientType   string

	currentEvent string
	lineCount    int
	errorCount   int

	sequenceNumber int

	accumulatedText      string
	accumulatedReasoning string

	// rawAccumulator 可选地累积上游原始 SSE 字节流（含 data: 前缀和 \n\n 分隔），
	// 供日志记录 RawResponseBody 用。nil 时不累积（非日志场景避免内存开销）。
	// 仅在转换 goroutine 内写入；goroutine 结束（pipe Close）后调用方才读取，无并发。
	rawAccumulator     *strings.Builder
	rawAccumulatorFull bool

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
	anthropicActiveOutputIndex   int
	anthropicItemMeta            map[int]anthropicItemMeta
	anthropicDeltaBuffer         map[int][]anthropicBufferedDelta
	anthropicDoneOutputIndices   map[int]bool
}

// TransformResponseRealtime performs real-time streaming response conversion
// directly from the response Body reader.
//
// rawAccumulator 可选地累积上游原始 SSE 字节流；nil 时不累积。
func TransformResponseRealtime(response *http.Response, providerType, clientType string, rawAccumulator *strings.Builder) (*http.Response, error) {
	// Reduce N×N streaming conversions to N+N by routing through OpenAI Responses
	// streaming format when neither side is already using it.
	//
	// provider -> openai-res (canonical) -> client
	if providerType != "openai-res" && clientType != "openai-res" {
		return transformResponseRealtimeViaResponses(response, providerType, clientType, rawAccumulator)
	}

	pr, pw := io.Pipe()

	go func() {
		defer response.Body.Close()
		err := transformStreamBodyRealtime(response.Body, pw, providerType, clientType, rawAccumulator)
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

func transformResponseRealtimeViaResponses(response *http.Response, providerType, clientType string, rawAccumulator *strings.Builder) (*http.Response, error) {
	midReader, midWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	go func() {
		defer response.Body.Close()

		// 第一个 goroutine 读上游原始 body，传累积器；第二个 goroutine 读中间格式，不累积。
		err := transformStreamBodyRealtime(response.Body, midWriter, providerType, "openai-res", rawAccumulator)
		if err != nil {
			midWriter.CloseWithError(err)
			return
		}
		midWriter.Close()
	}()

	go func() {
		defer midReader.Close()

		err := transformStreamBodyRealtime(midReader, outWriter, "openai-res", clientType, nil)
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

func transformStreamBodyRealtime(src io.ReadCloser, dst *io.PipeWriter, providerType, clientType string, rawAccumulator *strings.Builder) error {
	scanner := bufio.NewScanner(src)
	// Increase initial buffer to 64KB, and cap at maxSSEEventSize (avoid "token too long").
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)

	state := &realtimeStreamState{
		writer:                      dst,
		providerType:                providerType,
		clientType:                  clientType,
		anthropicActiveBlockIndex:   -1,
		anthropicActiveOutputIndex:  -1,
		rawAccumulator:              rawAccumulator,
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

		// 累积原始 SSE 字节流供日志记录 RawResponseBody。达到上限后停止累积。
		accumulateRawSSE(state, eventName, data)

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
		slog.Error("scanner error in stream transformation",
			"provider_type", state.providerType,
			"client_type", state.clientType,
			"lines_processed", state.lineCount,
			"errors_encountered", state.errorCount,
			"error", err)
		return err
	}

	// EOF but without trailing blank line: flush last event once.
	if err := flushEvent(); err != nil {
		slog.Error("failed to flush last SSE event in stream transformation",
			"provider_type", state.providerType,
			"client_type", state.clientType,
			"lines_processed", state.lineCount,
			"errors_encountered", state.errorCount,
			"error", err)
		return err
	}

	slog.Debug("stream transformation completed",
		"provider_type", state.providerType,
		"client_type", state.clientType,
		"lines_processed", state.lineCount,
		"errors_encountered", state.errorCount)
	return nil
}

func dispatchRealtimeStreamChunk(state *realtimeStreamState, data string) error {
	if h, ok := lookupRealtimeRoute(state.providerType, state.clientType); ok {
		return h(state, data)
	}
	// Keep behavior: passthrough. Add observability for unexpected routes.
	if state.unknownRouteCount < 3 {
		state.unknownRouteCount++
		slog.Warn("unknown stream transform route, passthrough",
			"provider_type", state.providerType,
			"client_type", state.clientType,
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
		"provider_type", state.providerType,
		"client_type", state.clientType,
		"line", state.lineCount,
		"error", err)
}

func logRealtimeChunkParseError(state *realtimeStreamState, data string, err error) {
	state.errorCount++
	slog.Error("failed to parse SSE chunk in stream transformation",
		"provider_type", state.providerType,
		"client_type", state.clientType,
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

func nextRealtimeSequence(state *realtimeStreamState) int {
	seq := state.sequenceNumber
	state.sequenceNumber++
	return seq
}

// accumulateRawSSE 把原始 SSE event 追加到累积器，达到 maxRawAccumulatorSize 后停止。
// 累积格式与上游原始 SSE 一致：event 行（若有）+ data 行 + 空行，便于事后整段查看或解析。
func accumulateRawSSE(state *realtimeStreamState, eventName, data string) {
	if state.rawAccumulator == nil || state.rawAccumulatorFull {
		return
	}
	// 估算本次追加大小：event 行 + data 行 + 空行。
	chunk := 0
	if eventName != "" {
		chunk += len("event: ") + len(eventName) + 1
	}
	chunk += len("data: ") + len(data) + 2 // "data: <data>\n\n"

	if state.rawAccumulator.Len()+chunk > maxRawAccumulatorSize {
		state.rawAccumulatorFull = true
		slog.Warn("raw response body accumulator reached size limit, truncating",
			"limit", maxRawAccumulatorSize,
			"provider_type", state.providerType,
			"client_type", state.clientType,
		)
		return
	}

	if eventName != "" {
		state.rawAccumulator.WriteString("event: ")
		state.rawAccumulator.WriteString(eventName)
		state.rawAccumulator.WriteByte('\n')
	}
	state.rawAccumulator.WriteString("data: ")
	state.rawAccumulator.WriteString(data)
	state.rawAccumulator.WriteString("\n\n")
}
