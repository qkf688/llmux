package responses

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

// 本文件测试 function_call_output 多模态 output 的编解码 helper 与往返保真。
// 对应 .local/next-do.md 待办 46。

// unifiedPartsToToolOutput 生成 input_text/input_image 数组。
func TestUnifiedPartsToToolOutput_TextAndImage(t *testing.T) {
	text := "结果如下"
	parts := []models.UnifiedMessageContentPart{
		{Type: "text", Text: &text},
		{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
	}
	arr := unifiedPartsToToolOutput(parts)
	if arr == nil {
		t.Fatal("应返回非 nil 数组")
	}
	if len(arr) != 2 {
		t.Fatalf("应产出 2 个块，实际 %d", len(arr))
	}
	if arr[0]["type"] != "input_text" || arr[0]["text"] != "结果如下" {
		t.Fatalf("第一个块应为 input_text，实际 %#v", arr[0])
	}
	if arr[1]["type"] != "input_image" || arr[1]["image_url"] != "data:image/png;base64,AAAA" {
		t.Fatalf("第二个块应为 input_image 且保留 URL，实际 %#v", arr[1])
	}
}

// 含 detail 的 image 块应保留 detail 字段。
func TestUnifiedPartsToToolOutput_ImageWithDetail(t *testing.T) {
	detail := "high"
	parts := []models.UnifiedMessageContentPart{
		{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "https://example.com/x.png", Detail: &detail}},
	}
	arr := unifiedPartsToToolOutput(parts)
	if arr == nil || len(arr) != 1 {
		t.Fatalf("应产出 1 个块，实际 %#v", arr)
	}
	if arr[0]["detail"] != "high" {
		t.Fatalf("应保留 detail 字段，实际 %#v", arr[0]["detail"])
	}
}

// 空块数组或仅含无效块时返回 nil（调用方据此降级）。
func TestUnifiedPartsToToolOutput_EmptyOrInvalid(t *testing.T) {
	if got := unifiedPartsToToolOutput(nil); got != nil {
		t.Fatalf("nil 输入应返回 nil，实际 %#v", got)
	}
	if got := unifiedPartsToToolOutput([]models.UnifiedMessageContentPart{}); got != nil {
		t.Fatalf("空数组应返回 nil，实际 %#v", got)
	}
	// 仅含空 URL 的 image 块
	parts := []models.UnifiedMessageContentPart{
		{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: ""}},
	}
	if got := unifiedPartsToToolOutput(parts); got != nil {
		t.Fatalf("无效块应返回 nil，实际 %#v", got)
	}
}

// 往返保真：入站 array → UnifiedMessage 块数组 → 出站 array，image URL 原样保留。
func TestToolOutput_Roundtrip_MultimodalPreserved(t *testing.T) {
	text := "截图如下"
	// 入站：Responses array → UnifiedMessage
	raw := []interface{}{
		map[string]interface{}{"type": "input_text", "text": "截图如下"},
		map[string]interface{}{"type": "input_image", "image_url": "data:image/png;base64,BBBB"},
	}
	content, ok := parsePartsToUnifiedContent(raw)
	if !ok {
		t.Fatal("入站解析应成功")
	}
	parts, ok := content.([]models.UnifiedMessageContentPart)
	if !ok {
		t.Fatalf("含图片应解析为块数组，实际 %#v", content)
	}
	if len(parts) != 2 {
		t.Fatalf("应解析出 2 个块，实际 %d", len(parts))
	}
	if parts[0].Type != "text" || *parts[0].Text != text {
		t.Fatalf("第一个块应为 text，实际 %#v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,BBBB" {
		t.Fatalf("第二个块应为 image_url 且保留 URL，实际 %#v", parts[1])
	}

	// 出站：UnifiedMessage 块数组 → Responses array
	arr := unifiedPartsToToolOutput(parts)
	if arr == nil || len(arr) != 2 {
		t.Fatalf("出站应产出 2 个块，实际 %#v", arr)
	}
	if arr[1]["image_url"] != "data:image/png;base64,BBBB" {
		t.Fatalf("往返后 image URL 应原样保留，实际 %#v", arr[1]["image_url"])
	}
}

// 往返保真：入站纯文本 array → UnifiedMessage string → 出站 string（老上游兼容路径）。
func TestToolOutput_Roundtrip_TextOnlyArrayDegradesToString(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{"type": "input_text", "text": "纯文本结果"},
	}
	content, ok := parsePartsToUnifiedContent(raw)
	if !ok {
		t.Fatal("入站解析应成功")
	}
	s, ok := content.(string)
	if !ok {
		t.Fatalf("纯文本 array 应降级为 string（与 Anthropic 路径一致），实际 %#v", content)
	}
	if s != "纯文本结果" {
		t.Fatalf("文本内容应保留，实际 %#v", s)
	}
}
