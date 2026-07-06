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

	// OpenAI Chat streaming meta (used when the output format is OpenAI Chat).
	openAIID                                  string
	openAIModel                               string
	openAICreated                             int64
	openAIToolCallNextIndex                   int
	responsesOutputIndexToOpenAIToolCallIndex map[int]int

	unknownRouteCount int

	responseID string
	itemID     string

	reasoningItemID      string
	hasReasoningItem     bool
	reasoningOutputIndex int

	// Responses -> Anthropic: map output_index to Anthropic content_block index.
	anthropicActiveBlockIndex                 int
	anthropicNextBlockIndex                   int
	responsesOutputIndexToAnthropicBlockIndex map[int]int
}

// TransformResponseRealtime performs real-time streaming response conversion
// directly from the response Body reader.
func TransformResponseRealtime(response *http.Response, providerType, clientType string) (*http.Response, error) {
	// Reduce N×N streaming conversions to N+N by routing through OpenAI Responses
	// streaming format when neither side is already using it.
	//
	// provider -> openai-res (canonical) -> client
	if providerType != "openai-res" && clientType != "openai-res" {
		return transformResponseRealtimeViaResponses(response, providerType, clientType)
	}

	pr, pw := io.Pipe()

	go func() {
		defer response.Body.Close()
		err := transformStreamBodyRealtime(response.Body, pw, providerType, clientType)
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

func transformResponseRealtimeViaResponses(response *http.Response, providerType, clientType string) (*http.Response, error) {
	midReader, midWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	go func() {
		defer response.Body.Close()

		err := transformStreamBodyRealtime(response.Body, midWriter, providerType, "openai-res")
		if err != nil {
			midWriter.CloseWithError(err)
			return
		}
		midWriter.Close()
	}()

	go func() {
		defer midReader.Close()

		err := transformStreamBodyRealtime(midReader, outWriter, "openai-res", clientType)
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

func transformStreamBodyRealtime(src io.ReadCloser, dst *io.PipeWriter, providerType, clientType string) error {
	scanner := bufio.NewScanner(src)
	// Increase initial buffer to 64KB, and cap at maxSSEEventSize (avoid "token too long").
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)

	state := &realtimeStreamState{
		writer:                    dst,
		providerType:              providerType,
		clientType:                clientType,
		anthropicActiveBlockIndex: -1,
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
	switch {
	case state.providerType == "anthropic" && state.clientType == "openai-res":
		return handleRealtimeAnthropicToResponses(state, data)
	case state.providerType == "openai-res" && state.clientType == "anthropic":
		return handleRealtimeResponsesToAnthropic(state, data)
	case state.providerType == "openai" && state.clientType == "openai-res":
		return handleRealtimeOpenAIToResponses(state, data)
	case state.providerType == "openai-res" && state.clientType == "openai":
		return handleRealtimeResponsesToOpenAI(state, data)
	default:
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
