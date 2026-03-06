package common

import (
	"encoding/base64"
	"strings"
)

// ParseBase64DataURL parses `data:<media_type>;base64,<data>` URLs and returns the media type and the base64 payload.
// It performs best-effort base64 validation to avoid emitting invalid provider requests.
func ParseBase64DataURL(raw string) (mediaType string, data string, ok bool) {
	if !strings.HasPrefix(raw, "data:") {
		return "", "", false
	}

	headerAndData := strings.TrimPrefix(raw, "data:")
	header, rest, found := strings.Cut(headerAndData, ",")
	if !found {
		return "", "", false
	}

	header = strings.TrimSpace(header)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", "", false
	}

	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return "", "", false
	}

	mediaType = strings.TrimSpace(parts[0])
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	isBase64 := false
	for _, p := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(p), "base64") {
			isBase64 = true
			break
		}
	}
	if !isBase64 {
		return "", "", false
	}

	if !isValidBase64(rest) {
		return "", "", false
	}

	return mediaType, rest, true
}

func BuildBase64DataURL(mediaType string, base64Data string) string {
	mt := strings.TrimSpace(mediaType)
	if mt == "" {
		mt = "application/octet-stream"
	}
	return "data:" + mt + ";base64," + base64Data
}

func isValidBase64(s string) bool {
	if _, err := base64.StdEncoding.DecodeString(s); err == nil {
		return true
	}
	if _, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return true
	}
	if _, err := base64.URLEncoding.DecodeString(s); err == nil {
		return true
	}
	if _, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return true
	}
	return false
}
