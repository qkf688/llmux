package transform

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/streaming"
)

// TransformProviderResponse 把上游响应体从 upstreamFormat 转成 clientFormat。
//
// 两个参数都是**协议形状**：两端形状相同即直通，与「上游供应商是谁」无关。
// 因此 openai 客户端打一家 OpenAI 兼容的新上游时会正确走直通，而不会因 provider type
// 字符串不同而白跑一趟转换。
func TransformProviderResponse(response *http.Response, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	if upstreamFormat == clientFormat {
		return response, nil
	}

	// 检查是否是流式响应
	contentType := response.Header.Get("Content-Type")
	isStream := strings.Contains(strings.ToLower(contentType), "text/event-stream")

	if isStream {
		// 流式响应：直接从 Body 读取器进行实时转换
		return streaming.TransformResponseRealtime(response, upstreamFormat, clientFormat, sideChannel)
	}

	// 非流式响应：读取完整响应体后转换
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()

	return transformNonStreamResponse(response, body, upstreamFormat, clientFormat, sideChannel)
}

func transformNonStreamResponse(response *http.Response, body []byte, upstreamFormat, clientFormat consts.WireFormat, sideChannel *models.TransformSideChannel) (*http.Response, error) {
	providerAdapter, err := getAdapter(upstreamFormat)
	if err != nil {
		return nil, err
	}
	unified, err := providerAdapter.ParseResponse(body)
	if err != nil {
		return nil, err
	}

	// 上游 usage 在此旁路交给落库侧：解析上游响应即得到，无 goroutine、无时序问题。
	// 落库优先用它而非从转换后的下游体反解（下游受目标协议表达能力限制）。
	if sideChannel != nil && unified != nil && unified.Usage != nil {
		sideChannel.SetUpstreamUsage(*unified.Usage)
	}

	clientAdapter, err := getAdapter(clientFormat)
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
