package common

import "testing"

func TestParseBase64DataURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantType string
		wantData string
		wantOK   bool
	}{
		{
			name:     "base64_png",
			raw:      "data:image/png;base64,AAA=",
			wantType: "image/png",
			wantData: "AAA=",
			wantOK:   true,
		},
		{
			name:     "base64_no_padding",
			raw:      "data:image/png;base64,AAA",
			wantType: "image/png",
			wantData: "AAA",
			wantOK:   true,
		},
		{
			name:     "missing_media_type_defaults",
			raw:      "data:;base64,AAA=",
			wantType: "application/octet-stream",
			wantData: "AAA=",
			wantOK:   true,
		},
		{
			name:   "not_data_url",
			raw:    "https://example.com/a.png",
			wantOK: false,
		},
		{
			name:   "missing_comma",
			raw:    "data:image/png;base64AAA=",
			wantOK: false,
		},
		{
			name:   "not_base64",
			raw:    "data:image/png,AAA=",
			wantOK: false,
		},
		{
			name:   "empty_payload",
			raw:    "data:image/png;base64,",
			wantOK: false,
		},
		{
			name:   "invalid_base64_payload",
			raw:    "data:image/png;base64,!!!",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotType, gotData, gotOK := ParseBase64DataURL(tt.raw)
			if gotOK != tt.wantOK {
				t.Fatalf("ok: want %v, got %v", tt.wantOK, gotOK)
			}
			if !tt.wantOK {
				return
			}
			if gotType != tt.wantType {
				t.Fatalf("media type: want %q, got %q", tt.wantType, gotType)
			}
			if gotData != tt.wantData {
				t.Fatalf("data: want %q, got %q", tt.wantData, gotData)
			}
		})
	}
}

func TestBuildBase64DataURL(t *testing.T) {
	t.Parallel()

	if got := BuildBase64DataURL("image/png", "AAA="); got != "data:image/png;base64,AAA=" {
		t.Fatalf("unexpected data url: %q", got)
	}
	if got := BuildBase64DataURL("", "AAA="); got != "data:application/octet-stream;base64,AAA=" {
		t.Fatalf("unexpected default data url: %q", got)
	}
}
