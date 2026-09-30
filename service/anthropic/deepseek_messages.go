package anthropic

// DeepSeek Messages 的**工具变更块**是 Anthropic Messages 规范之外的私有扩展：
// 客户端在会话中途新增/移除工具时，会把一条 developer 消息追加进历史，其适配器序列化成
//
//	{"role":"system","content":[{"type":"tool_addition","tool":{"type":"tool_reference","name":"read_page"}}]}
//
// 严格 Anthropic 上游对 content 块的 type 做白名单校验，遇到这两个 type 直接回
// 400 InvalidParameter "Request body format invalid"；而该消息一旦进入会话历史，之后**每条**
// 请求都带着它，客户端重试也只会重复同一条被拒的 body（2026-09-30 实际事故，日志 11517：
// 连续 12 条 400 来自同一会话的 2 次请求 × 6 次尝试）。
//
// 故本文件对它「认领」：块类型判定与文本渲染是**唯一一份**协议知识，两个出站路径共用——
//   - 转换路径：service/anthropic 的 parseContentBlocks 把块渲染成文本 part（不再静默丢弃）；
//   - 直通路径：service/chat/preprocess/anthropic 在原始 body 上就地降级该块（私有块不得
//     原样泄给严格上游）。
//
// 归一化的语义下限是「块换成文本、工具声明仍由 tools 数组承载」，故信息不丢：客户端在
// 线协议里同时发出 tools 数组（含新增工具的 schema），模型据此仍能调用该工具。
const (
	// BlockTypeToolAddition / BlockTypeToolRemoval 是工具变更块的两个 type 字面量。
	BlockTypeToolAddition = "tool_addition"
	BlockTypeToolRemoval  = "tool_removal"
)

// IsToolChangeBlockType 报告块 type 是否属于 DeepSeek Messages 的工具变更扩展。
func IsToolChangeBlockType(blockType string) bool {
	return blockType == BlockTypeToolAddition || blockType == BlockTypeToolRemoval
}

// ToolChangeNoticeText 返回工具变更块的**文本替身**，供不认该扩展的形状消费。
//
// 文案与客户端自身 UI 的措辞同源（"Tool added: {name}" / "Tool removed: {name}"，见 DSH
// 的 layout.toolAdded / layout.toolRemoved），避免网关另造一套说法让模型在同一事件上看到
// 两种表述。toolName 为空时返回空串：没有名字就没有可承载的信息，调用方据此丢弃该块——
// 留着它只会让严格上游继续判 400。
func ToolChangeNoticeText(blockType, toolName string) string {
	if toolName == "" {
		return ""
	}
	switch blockType {
	case BlockTypeToolAddition:
		return "Tool added: " + toolName
	case BlockTypeToolRemoval:
		return "Tool removed: " + toolName
	default:
		return ""
	}
}
