package service

import (
	"bufio"
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
		// 增加初始缓冲区大小到 64KB，最大 15MB（与 process.go 一致）
		scanner.Buffer(make([]byte, 0, 64*1024), 15*1024*1024)

		state := &realtimeStreamState{
			writer:       pw,
			providerType: providerType,
			clientType:   clientType,
		}

		for scanner.Scan() {
			state.lineCount++
			line := scanner.Text()
			if line == "" {
				// 空行是 SSE 消息分隔符
				state.currentEvent = ""
				continue
			}

			// 处理 event 行（记录事件类型）- 兼容带空格和不带空格两种格式
			if strings.HasPrefix(line, "event:") {
				state.currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			// 处理 data 行 - 兼容带空格和不带空格两种格式
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}

			if err := dispatchRealtimeStreamChunk(state, data); err != nil {
				logRealtimeWriteError(state, err)
				pw.CloseWithError(err)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Error("scanner error in stream transformation",
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
