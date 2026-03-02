package unified

// UnifiedMessage 统一消息格式。
type UnifiedMessage struct {
	Role       string            `json:"role"`
	Content    interface{}       `json:"content,omitempty"` // 支持 string 或 []UnifiedMessageContentPart
	ToolCalls  []UnifiedToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`

	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// Extended Thinking 支持
	ReasoningContent   *string `json:"reasoning_content,omitempty"`
	Reasoning          *string `json:"reasoning,omitempty"`
	ReasoningSignature *string `json:"reasoning_signature,omitempty"`

	// 帮助字段
	MessageIndex    *int    `json:"-"`
	ToolCallName    *string `json:"-"`
	ToolCallIsError *bool   `json:"-"`
}

// ClearHelpFields 清除帮助字段。
func (m *UnifiedMessage) ClearHelpFields() {
	m.ReasoningContent = nil
	m.Reasoning = nil
	m.ReasoningSignature = nil
	m.MessageIndex = nil
	m.ToolCallName = nil
	m.ToolCallIsError = nil
}

// GetReasoningContent 获取推理内容。
func (m *UnifiedMessage) GetReasoningContent() string {
	if m.ReasoningContent != nil {
		return *m.ReasoningContent
	}
	if m.Reasoning != nil {
		return *m.Reasoning
	}
	return ""
}

// SetReasoningContent 设置推理内容。
func (m *UnifiedMessage) SetReasoningContent(content string) {
	m.ReasoningContent = &content
}

// GetContentAsString 获取纯文本内容。
func (m *UnifiedMessage) GetContentAsString() string {
	if m.Content == nil {
		return ""
	}
	if str, ok := m.Content.(string); ok {
		return str
	}
	if parts, ok := m.Content.([]UnifiedMessageContentPart); ok {
		var texts []string
		for _, part := range parts {
			if part.Type == "text" && part.Text != nil {
				texts = append(texts, *part.Text)
			}
		}
		return joinStrings(texts, "")
	}
	return ""
}

// GetContentParts 获取多模态内容部分。
func (m *UnifiedMessage) GetContentParts() []UnifiedMessageContentPart {
	if m.Content == nil {
		return nil
	}
	if parts, ok := m.Content.([]UnifiedMessageContentPart); ok {
		return parts
	}
	if str, ok := m.Content.(string); ok && str != "" {
		return []UnifiedMessageContentPart{
			{Type: "text", Text: &str},
		}
	}
	return nil
}

// SetContentString 设置纯文本内容。
func (m *UnifiedMessage) SetContentString(content string) {
	m.Content = content
}

// SetContentParts 设置多模态内容。
func (m *UnifiedMessage) SetContentParts(parts []UnifiedMessageContentPart) {
	m.Content = parts
}

// CacheControl 缓存控制 (Anthropic 特有)。
type CacheControl struct {
	Type string `json:"-"`
	TTL  string `json:"-"`
}

// UnifiedMessageContentPart 消息内容部分 (支持多种类型)。
type UnifiedMessageContentPart struct {
	Type         string             `json:"type"`
	Text         *string            `json:"text,omitempty"`
	ImageURL     *UnifiedImageURL   `json:"image_url,omitempty"`
	InputAudio   *UnifiedInputAudio `json:"input_audio,omitempty"`
	CacheControl *CacheControl      `json:"-"`
}

// UnifiedImageURL 图像 URL 配置。
type UnifiedImageURL struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}

// UnifiedInputAudio 音频输入配置。
type UnifiedInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
