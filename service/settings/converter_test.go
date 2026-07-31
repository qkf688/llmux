package settings

import (
	"reflect"
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestToString_FromString_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		typ   models.SettingType
		value any
	}{
		{"bool-true", models.SettingTypeBool, true},
		{"bool-false", models.SettingTypeBool, false},
		{"int", models.SettingTypeInt, 42},
		{"string", models.SettingTypeString, "hello"},
		{"string-slice", models.SettingTypeStringSlice, []string{"a", "b", "c"}},
		{"json", models.SettingTypeJSON, models.RawLogOptions{RequestBody: true, ResponseBody: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			str, err := ToString(tc.value, tc.typ)
			if err != nil {
				t.Fatalf("ToString error: %v", err)
			}
			parsed, err := FromString(str, tc.typ)
			if err != nil {
				t.Fatalf("FromString error: %v", err)
			}
			if !reflect.DeepEqual(parsed, tc.value) {
				t.Fatalf("parsed = %v (%T), want %v (%T)", parsed, parsed, tc.value, tc.value)
			}
		})
	}
}

func TestFromString_InvalidInt(t *testing.T) {
	_, err := FromString("not-a-number", models.SettingTypeInt)
	if err == nil {
		t.Fatal("expected error for invalid int")
	}
}

func TestFromString_InvalidJSON(t *testing.T) {
	_, err := FromString("invalid-json", models.SettingTypeJSON)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}
