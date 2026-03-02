package transform

import (
	"io"
	"net/http"
	"strings"
)

func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
	if providerType == clientType {
		return response, nil
	}

	// 检查是否是流式响应
	contentType := response.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if isStream {
		// 流式响应：直接从 Body 读取器进行实时转换
		return transformStreamResponseRealtime(response, providerType, clientType)
	}

	// 非流式响应：读取完整响应体后转换
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()

	return transformNonStreamResponse(response, body, providerType, clientType)
}

func transformNonStreamResponse(response *http.Response, body []byte, providerType, clientType string) (*http.Response, error) {
	var unified *UnifiedResponse
	var err error

	// 供应商格式 -> 统一格式
	switch providerType {
	case "openai":
		unified, err = parseOpenAIResponse(body)
	case "openai-res":
		unified, err = parseResponsesResponse(body)
	case "anthropic":
		unified, err = parseAnthropicResponse(body)
	default:
		unified, err = parseOpenAIResponse(body)
	}

	if err != nil {
		return nil, err
	}

	// 统一格式 -> 客户端格式
	var newBody []byte
	switch clientType {
	case "openai":
		newBody, err = formatOpenAIResponse(unified)
	case "openai-res":
		newBody, err = formatResponsesResponse(unified)
	case "anthropic":
		newBody, err = formatAnthropicResponse(unified)
	default:
		newBody, err = formatOpenAIResponse(unified)
	}

	if err != nil {
		return nil, err
	}

	// 创建新响应
	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        response.Header.Clone(),
		Body:          io.NopCloser(strings.NewReader(string(newBody))),
		ContentLength: int64(len(newBody)),
	}

	return newResponse, nil
}

// transformStreamResponseRealtime 实时流式响应转换（直接从 Body 读取器转换）
