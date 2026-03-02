package virtualmodel

import "errors"

var (
	errNoEnabledMappings               = errors.New("no enabled mappings found for virtual model")
	errNoRealModels                    = errors.New("no real models found")
	errNoAvailableModelsAfterBlacklist = errors.New("no available models after filtering blacklisted providers")
	errModelNotFoundInMap              = errors.New("model not found in map")
	errSelectedModelNotFound           = errors.New("selected model not found in map")
)
