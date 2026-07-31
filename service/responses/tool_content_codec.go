package responses

import (
	"github.com/qkf688/llmux/models"
)

// unifiedPartsToToolOutput 把统一模型的多模态内容块转换为 Responses
// function_call_output.output 的数组形式（input_text / input_image）。
//
// 与 unifiedPartsToResponsesContent 的区别：后者服务于 message.content，按 role
// 区分 input_text/output_text 且 image 仅对 user 生成；本函数服务于
// function_call_output.output，按 OpenAI Responses 协议规范始终使用 input_text/input_image，
// 不论来源角色，且只处理 text/image_url 两种块类型（input_file 暂不支持，统一模型无 file 块）。
//
// 纯文本 tool result 不走此函数——调用方直接赋 string 给 Output（老上游兼容）。
// 仅当 content 为 []UnifiedMessageContentPart 且含非文本块时才调用此函数。
func unifiedPartsToToolOutput(parts []models.UnifiedMessageContentPart) []map[string]interface{} {
	if len(parts) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil {
				continue
			}
			out = append(out, map[string]interface{}{
				"type": "input_text",
				"text": *part.Text,
			})
		case "image_url":
			if part.ImageURL == nil || part.ImageURL.URL == "" {
				continue
			}
			item := map[string]interface{}{
				"type":      "input_image",
				"image_url": part.ImageURL.URL,
			}
			if part.ImageURL.Detail != nil && *part.ImageURL.Detail != "" {
				item["detail"] = *part.ImageURL.Detail
			}
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
