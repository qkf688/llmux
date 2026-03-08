package chat

// clientRequestError marks an error that should be returned to the caller as-is
// (HTTP status), rather than being treated as a provider failure/retryable error.
type clientRequestError struct {
	statusCode int
	message    string
}

func (e *clientRequestError) Error() string { return e.message }

func (e *clientRequestError) StatusCode() int { return e.statusCode }

func newClientRequestError(statusCode int, message string) error {
	return &clientRequestError{statusCode: statusCode, message: message}
}
