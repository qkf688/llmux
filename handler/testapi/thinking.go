package testapi

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/tidwall/gjson"
)

var errThinkingNotFound = errors.New("上游未返回推理内容，模型可能不支持 thinking")

// TestThinkingHandler thinking 能力测试（GET /api/test/thinking/:id）。
//
// 测的是上游模型本身是否具备 thinking 能力：请求体按协议形状携带 thinking 参数，
// 响应按协议形状提取推理内容，非空即通过——直连上游、不走网关转换层，因此
// 即使关联 SupportsThinking=false 也允许执行（结果可作为开启该开关的决策依据）。
// 公共流程（链路装配、探针、失败回包）在 runCompletionTestFlow；本 handler 只做领域判定。
func TestThinkingHandler(c *gin.Context) {
	stream := queryStreamMode(c)
	link, outcome, fail, ok := runCompletionTestFlow(c, "thinking", buildThinkingTestBody, stream)
	if !ok {
		return
	}

	// 流式模式下 outcome.body 已是折叠出的非流式形状，extractThinking 一行都不用改。
	thinkingText, err := extractThinking(outcome.format, outcome.body)
	if err != nil {
		// raw_output 是区分「上游不支持 thinking」与「上游忽略该参数」的唯一排查手段，失败必带。
		fail(common.ErrorTypeValidation, "提取推理内容失败", err.Error(), outcome.rawText)
		return
	}

	result := passedCapabilityResult(link, outcome, "Thinking 能力测试通过"+modeSuffix(stream), map[string]any{
		"thinking_text": thinkingText,
	})
	httpresp.Success(c, result)
}

// extractThinking 按**上游响应体形状**取推理内容：三个协议放推理内容的位置不同。
// 判定用 wire format 而非 provider type——OpenAI 兼容上游的响应体同样是 OpenAI Chat 形状，
// 按「哪一家」分支会让每接一家新上游都要来这里加 case（与 extractStructuredOutputJSON 同款）。
func extractThinking(upstreamFormat consts.WireFormat, responseBody []byte) (thinkingText string, err error) {
	switch upstreamFormat {
	case consts.FormatOpenAIResponses:
		// reasoning item 的正文在 summary[].text（纯推理模型可能只有 encrypted_content，
		// 此时 item 存在即判通过，thinking_text 允许为空）。
		reasoning := gjson.GetBytes(responseBody, `output.#(type=="reasoning")`)
		if !reasoning.Exists() {
			return "", errThinkingNotFound
		}
		var parts []string
		if reasoning.Get("summary").IsArray() {
			for _, part := range reasoning.Get("summary").Array() {
				if text := strings.TrimSpace(part.Get("text").String()); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n"), nil
	case consts.FormatAnthropic:
		thinking := gjson.GetBytes(responseBody, `content.#(type=="thinking").thinking`)
		// 只接受字符串：对象/数组形状是异常信封而非推理内容，按未返回处理（与 OpenAI 分支同口径）。
		if thinking.Type != gjson.String {
			return "", errThinkingNotFound
		}
		text := strings.TrimSpace(thinking.String())
		if text == "" {
			return "", errThinkingNotFound
		}
		return text, nil
	default:
		// OpenAI chat completions（及兼容上游）：DeepSeek 系放 reasoning_content，OpenAI o 系（及兼容网关）放 reasoning。
		// 只接受字符串值——个别网关把该字段回成对象（reasoning_details 类结构），JSON 原文不是推理内容，按未返回处理。
		for _, path := range []string{"choices.0.message.reasoning_content", "choices.0.message.reasoning"} {
			if r := gjson.GetBytes(responseBody, path); r.Type == gjson.String {
				if text := strings.TrimSpace(r.String()); text != "" {
					return text, nil
				}
			}
		}
		return "", errThinkingNotFound
	}
}
