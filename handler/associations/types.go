package associations

// ModelWithProviderRequest represents the request body for creating/updating a model-provider association.
type ModelWithProviderRequest struct {
	ModelID          uint              `json:"model_id"`
	ProviderModel    string            `json:"provider_name"`
	ProviderID       uint              `json:"provider_id"`
	ToolCall         bool              `json:"tool_call"`
	StructuredOutput bool              `json:"structured_output"`
	Image            bool              `json:"image"`
	WithHeader       bool              `json:"with_header"`
	CustomerHeaders  map[string]string `json:"customer_headers"`
	Weight           int               `json:"weight"`
	Priority         int               `json:"priority"`
}

// ModelProviderStatusRequest represents the request body for updating provider status.
type ModelProviderStatusRequest struct {
	Status bool `json:"status"`
}

// BatchDeleteModelProvidersRequest represents the request body for batch deleting model-provider associations.
type BatchDeleteModelProvidersRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateModelProvidersStatusRequest represents the request body for batch updating status.
type BatchUpdateModelProvidersStatusRequest struct {
	IDs    []uint `json:"ids"`
	Status bool   `json:"status"`
}

// BatchUpdateModelProvidersCapabilitiesRequest represents the request body for batch updating capabilities.
// A nil capability field means "keep unchanged".
type BatchUpdateModelProvidersCapabilitiesRequest struct {
	IDs              []uint `json:"ids"`
	ToolCall         *bool  `json:"tool_call"`
	StructuredOutput *bool  `json:"structured_output"`
	Image            *bool  `json:"image"`
}