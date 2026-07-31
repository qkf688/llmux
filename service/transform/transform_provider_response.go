package transform

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/qkf688/llmux/service/transform/streaming"
)

func TransformProviderResponse(response *http.Response, providerType, clientType string) (*http.Response, error) {
	if providerType == clientType {
		return response, nil
	}

	// 检查是否是流式响应
	contentType := response.Header.Get("Content-Type")
	isStream := strings.Contains(strings.ToLower(contentType), "text/event-stream")

	if isStream {
		// 流式响应：直接从 Body 读取器进行实时转换
		return streaming.TransformResponseRealtime(response, providerType, clientType)
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
	providerAdapter, err := getAdapterOrDefault(providerType)
	if err != nil {
		return nil, err
	}
	unified, err := providerAdapter.ParseResponse(body)
	if err != nil {
		return nil, err
	}

	clientAdapter, err := getAdapterOrDefault(clientType)
	if err != nil {
		return nil, err
	}
	newBody, err := clientAdapter.FormatResponse(unified)
	if err != nil {
		return nil, err
	}

	// 创建新响应
	newHeader := response.Header.Clone()
	newHeader.Del("Content-Length")
	newHeader.Del("Content-Encoding")
	newHeader.Del("Transfer-Encoding")
	newHeader.Set("Content-Type", "application/json")
	newHeader.Set("Content-Length", strconv.Itoa(len(newBody)))

	newResponse := &http.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Proto:         response.Proto,
		ProtoMajor:    response.ProtoMajor,
		ProtoMinor:    response.ProtoMinor,
		Header:        newHeader,
		Body:          io.NopCloser(bytes.NewReader(newBody)),
		ContentLength: int64(len(newBody)),
	}

	return newResponse, nil
}
