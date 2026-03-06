package transform

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

	// OpenAI Chat streaming meta (used when the output format is OpenAI Chat).
	openAIID      string
	openAIModel   string
	openAICreated int64

	responseID string
	itemID     string

	reasoningItemID      string
	hasReasoningItem     bool
	reasoningOutputIndex int
}

// transformStreamResponseRealtime 实时流式响应转换（直接从 Body 读取器转换）
func transformStreamResponseRealtime(response *http.Response, providerType, clientType string) (*http.Response, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer response.Body.Close()

		scanner := bufio.NewScanner(response.Body)
		// 增加初始缓冲区大小到 64KB，最大使用 maxSSEEventSize（避免大事件导致 token too long）
		scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)

		state := &realtimeStreamState{
			writer:       pw,
			providerType: providerType,
			clientType:   clientType,
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
				// 空行是 SSE 消息分隔符：此时 event + data 组成一个完整事件
				if err := flushEvent(); err != nil {
					logRealtimeWriteError(state, err)
					pw.CloseWithError(err)
					return
				}
				continue
			}

			// 处理 event 行（记录事件类型）- 兼容带空格和不带空格两种格式
			if strings.HasPrefix(line, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			// 忽略注释行等
			if strings.HasPrefix(line, ":") {
				continue
			}

			// 处理 data 行 - 兼容带空格和不带空格两种格式
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}
				eventSize += len(data)
				if eventSize > maxSSEEventSize {
					err := fmt.Errorf("sse event too large: %d > %d", eventSize, maxSSEEventSize)
					logRealtimeWriteError(state, err)
					pw.CloseWithError(err)
					return
				}
				dataLines = append(dataLines, data)
				continue
			}

			// 其它字段（id/retry等）不参与转换
		}

		if err := scanner.Err(); err != nil {
			slog.Error("scanner error in stream transformation",
				"provider_type", state.providerType,
				"client_type", state.clientType,
				"lines_processed", state.lineCount,
				"errors_encountered", state.errorCount,
				"error", err)
			pw.CloseWithError(err)
			return
		}

		// EOF 但没有 trailing blank line：补一次 flush
		if err := flushEvent(); err != nil {
			slog.Error("failed to flush last SSE event in stream transformation",
				"provider_type", state.providerType,
				"client_type", state.clientType,
				"lines_processed", state.lineCount,
				"errors_encountered", state.errorCount,
				"error", err)
			pw.CloseWithError(err)
		} else {
			slog.Debug("stream transformation completed",
				"provider_type", state.providerType,
				"client_type", state.clientType,
				"lines_processed", state.lineCount,
				"errors_encountered", state.errorCount)
		}
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

func dispatchRealtimeStreamChunk(state *realtimeStreamState, data string) error {
	switch {
	case state.providerType == "anthropic" && state.clientType == "openai":
		return handleRealtimeAnthropicToOpenAI(state, data)
	case state.providerType == "openai" && state.clientType == "anthropic":
		return handleRealtimeOpenAIToAnthropic(state, data)
	case state.providerType == "anthropic" && state.clientType == "openai-res":
		return handleRealtimeAnthropicToResponses(state, data)
	case state.providerType == "openai-res" && state.clientType == "anthropic":
		return handleRealtimeResponsesToAnthropic(state, data)
	case state.providerType == "openai" && state.clientType == "openai-res":
		return handleRealtimeOpenAIToResponses(state, data)
	case state.providerType == "openai-res" && state.clientType == "openai":
		return handleRealtimeResponsesToOpenAI(state, data)
	default:
		return writeRealtimeData(state, data)
	}
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
