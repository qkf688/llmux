package transform

import (
	"context"
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

type FormatAdapter interface {
	ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error)
	FromUnified(unified *models.UnifiedRequest) ([]byte, error)
	ParseResponse(body []byte) (*models.UnifiedResponse, error)
	FormatResponse(unified *models.UnifiedResponse) ([]byte, error)
}

// formatAdapters 按 body 协议形状索引适配器，**不按上游供应商 type 索引**。
// 因此接一家 OpenAI 兼容的新上游（只在 providers 侧声明 WireFormat 为 FormatOpenAIChat）
// 在本包零改动（OCP）。
var formatAdapters = map[consts.WireFormat]FormatAdapter{}

func RegisterAdapter(format consts.WireFormat, adapter FormatAdapter) {
	if format == "" {
		panic("transform adapter format must not be empty")
	}
	if adapter == nil {
		panic(fmt.Sprintf("transform adapter %q must not be nil", format))
	}
	if _, exists := formatAdapters[format]; exists {
		panic(fmt.Sprintf("transform adapter %q is already registered", format))
	}
	formatAdapters[format] = adapter
}

// getAdapter 按形状取适配器，未注册即报错。
//
// 刻意**不**回退到 OpenAI：回退会把「provider 漏声明 WireFormat」「新协议漏注册适配器」
// 这类漏配静默变成「按 OpenAI 形状构建请求体」，错误现象飘到上游 400 才暴露，离根因很远。
func getAdapter(format consts.WireFormat) (FormatAdapter, error) {
	adapter, ok := formatAdapters[format]
	if !ok {
		return nil, fmt.Errorf("transform adapter %q is not registered", format)
	}
	return adapter, nil
}
