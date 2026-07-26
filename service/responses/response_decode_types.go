package responses

import "encoding/json"

type responsesResponseDecode struct {
	Object    string                `json:"object"`
	ID        string                `json:"id"`
	Model     string                `json:"model"`
	CreatedAt int64                 `json:"created_at"`
	Output    []responsesItemDecode `json:"output"`
	Status    *string               `json:"status,omitempty"`
	Usage     *ResponsesUsage       `json:"usage,omitempty"`
	Error     *ResponsesError       `json:"error,omitempty"`
}

type responsesItemDecode struct {
	ID        string                      `json:"id,omitempty"`
	Type      string                      `json:"type"`
	Role      string                      `json:"role,omitempty"`
	Text      *string                     `json:"text,omitempty"`
	Name      *string                     `json:"name,omitempty"`
	Arguments *string                     `json:"arguments,omitempty"`
	CallID    *string                     `json:"call_id,omitempty"`
	Output    *string                     `json:"output,omitempty"`
	Summary   []ResponsesReasoningSummary `json:"summary,omitempty"`
	Content   json.RawMessage             `json:"content,omitempty"`
	Error     *ResponsesError             `json:"error,omitempty"`
}

type responsesContentItem struct {
	Type        string                `json:"type"`
	Text        *string               `json:"text,omitempty"`
	Annotations []ResponsesAnnotation `json:"annotations,omitempty"`
	ImageURL    *string               `json:"image_url,omitempty"`
	Detail      *string               `json:"detail,omitempty"`
}

type responsesContentItemDecode struct {
	Type     string           `json:"type"`
	Text     *string          `json:"text,omitempty"`
	ImageURL *json.RawMessage `json:"image_url,omitempty"`
	URL      *string          `json:"url,omitempty"`
	Detail   *string          `json:"detail,omitempty"`
}

type responsesImageURLDecode struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}
