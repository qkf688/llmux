package testapi

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/tidwall/gjson"
)

var errVisionTextNotFound = errors.New("上游未返回文本内容，无法判定识别结果")

// visionRedWordRe 词边界匹配 red：裸子串会误命中 rendered/stored/colored/bordered 等假阳性。
// 中文「红色」无词边界问题，单独走子串匹配（见 visionAnswerMatches）。
var visionRedWordRe = regexp.MustCompile(`\bred\b`)

// visionAnswerMatches 判定模型回答是否命中期望颜色（纯函数）。
// 真在看图的模型必然答「红色/red」；猜颜色撞中概率可忽略（测试图固定纯色）。
// 中文含简繁两形（紅色）：zh-Hant 上游必答繁体，漏收会把有视觉能力的模型误判失败。
func visionAnswerMatches(text string) bool {
	return visionRedWordRe.MatchString(strings.ToLower(text)) ||
		strings.Contains(text, "红色") ||
		strings.Contains(text, "紅色")
}

// TestVisionHandler 图片识别能力测试（GET /api/test/vision/:id）。
//
// 测的是上游模型本身是否具备视觉能力：请求体按协议形状内嵌 64x64 纯红测试图并提问颜色，
// 回答命中 red/红色/紅色 即通过——直连上游、不走网关转换层，因此即使关联未开视觉开关也允许执行
// （结果可作为「该关联是否具备视觉能力」的决策依据）。
// 公共流程（链路装配、探针、失败回包）在 runCompletionTestFlow；本 handler 只做领域判定。
// 测试图不经 API 回显：它是静态常量，前端自内嵌同一 base64 在弹窗预览（vision-test-image.ts）。
func TestVisionHandler(c *gin.Context) {
	stream := queryStreamMode(c)
	link, outcome, fail, ok := runCompletionTestFlow(c, "vision", buildVisionTestBody, stream)
	if !ok {
		return
	}

	// 流式模式下 outcome.body 已是折叠出的非流式形状，extractVisionText 一行都不用改。
	visionText, err := extractVisionText(outcome.format, outcome.body)
	if err != nil {
		// raw_output 是区分「上游不支持视觉」与「上游忽略图片参数」的唯一排查手段，失败必带。
		fail(common.ErrorTypeValidation, "提取识别结果失败", err.Error(), outcome.rawText)
		return
	}
	if !visionAnswerMatches(visionText) {
		fail(common.ErrorTypeValidation, "图片识别未通过", "模型回答未命中期望颜色（red/红色/紅色）: "+visionText, outcome.rawText)
		return
	}

	result := passedCapabilityResult(link, outcome, "图片识别能力测试通过"+modeSuffix(stream), map[string]any{
		"vision_text": visionText,
	})
	httpresp.Success(c, result)
}

// extractVisionText 按**上游响应体形状**取全部人类可读文本：三个协议放正文的位置不同。
// 判定用 wire format 而非 provider type——OpenAI 兼容上游的响应体同样是 OpenAI Chat 形状
// （与 extractThinking / extractStructuredOutputJSON 同款）。
func extractVisionText(upstreamFormat consts.WireFormat, responseBody []byte) (string, error) {
	var parts []string
	switch upstreamFormat {
	case consts.FormatOpenAIResponses:
		// output 数组里 message item 的 output_text 部分；reasoning/refusal 不进判定文本。
		for _, item := range gjson.GetBytes(responseBody, "output").Array() {
			if item.Get("type").String() != "message" {
				continue
			}
			parts = appendTextParts(parts, item.Get("content").Array(), "output_text")
		}
	case consts.FormatAnthropic:
		// thinking 块是推理过程不是回答，只拼接 text 块。
		parts = appendTextParts(parts, gjson.GetBytes(responseBody, "content").Array(), "text")
	default:
		// OpenAI chat completions（及兼容上游）：content 正常是字符串，个别网关回成多模态数组。
		content := gjson.GetBytes(responseBody, "choices.0.message.content")
		switch content.Type {
		case gjson.String:
			parts = append(parts, content.String())
		case gjson.JSON:
			parts = appendTextParts(parts, content.Array(), "text")
		}
	}

	var trimmed []string
	for _, part := range parts {
		if text := strings.TrimSpace(part); text != "" {
			trimmed = append(trimmed, text)
		}
	}
	if len(trimmed) == 0 {
		return "", errVisionTextNotFound
	}
	return strings.Join(trimmed, "\n"), nil
}

// appendTextParts 从多模态 content 数组中收集指定 type 的 text 字段（字符串值才收，
// 对象/数组形状是异常信封而非文本，与 thinking 提取只接受字符串同口径）。
func appendTextParts(parts []string, blocks []gjson.Result, partType string) []string {
	for _, block := range blocks {
		if block.Get("type").String() != partType {
			continue
		}
		if text := block.Get("text"); text.Type == gjson.String {
			parts = append(parts, text.String())
		}
	}
	return parts
}
