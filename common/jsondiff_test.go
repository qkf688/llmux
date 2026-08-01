package common

import (
	"testing"
)

func TestJSONDiff_NoDiff(t *testing.T) {
	raw := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"max_tokens":8192}`
	after := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"max_tokens":8192}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LostFields) != 0 || len(result.AddedFields) != 0 || len(result.ChangedValues) != 0 {
		t.Fatalf("expected empty diff, got lost=%d added=%d changed=%d",
			len(result.LostFields), len(result.AddedFields), len(result.ChangedValues))
	}
}

func TestJSONDiff_LostField(t *testing.T) {
	raw := `{"model":"gpt-4","system":"be helpful","max_tokens":8192}`
	after := `{"model":"gpt-4","max_tokens":8192}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LostFields) != 1 {
		t.Fatalf("expected 1 lost field, got %d: %+v", len(result.LostFields), result.LostFields)
	}
	if result.LostFields[0].Path != "system" {
		t.Fatalf("expected lost path=system, got %s", result.LostFields[0].Path)
	}
}

func TestJSONDiff_AddedField(t *testing.T) {
	raw := `{"model":"gpt-4"}`
	after := `{"model":"gpt-4","max_tokens":8192}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AddedFields) != 1 {
		t.Fatalf("expected 1 added field, got %d: %+v", len(result.AddedFields), result.AddedFields)
	}
	if result.AddedFields[0].Path != "max_tokens" {
		t.Fatalf("expected added path=max_tokens, got %s", result.AddedFields[0].Path)
	}
	if result.AddedFields[0].After != float64(8192) {
		t.Fatalf("expected added after=8192, got %v", result.AddedFields[0].After)
	}
}

func TestJSONDiff_ChangedValue(t *testing.T) {
	raw := `{"max_tokens":1048576}`
	after := `{"max_tokens":8192}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ChangedValues) != 1 {
		t.Fatalf("expected 1 changed value, got %d: %+v", len(result.ChangedValues), result.ChangedValues)
	}
	e := result.ChangedValues[0]
	if e.Path != "max_tokens" {
		t.Fatalf("expected path=max_tokens, got %s", e.Path)
	}
	if e.Raw != float64(1048576) || e.After != float64(8192) {
		t.Fatalf("expected raw=1048576 after=8192, got raw=%v after=%v", e.Raw, e.After)
	}
}

func TestJSONDiff_NestedMap(t *testing.T) {
	raw := `{"messages":[{"role":"user","content":"hello"}]}`
	after := `{"messages":[{"role":"user","content":"hi"}]}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ChangedValues) != 1 {
		t.Fatalf("expected 1 changed value, got %d: %+v", len(result.ChangedValues), result.ChangedValues)
	}
	if result.ChangedValues[0].Path != "messages.0.content" {
		t.Fatalf("expected path=messages.0.content, got %s", result.ChangedValues[0].Path)
	}
}

func TestJSONDiff_SliceLengthChange(t *testing.T) {
	raw := `{"items":[1,2,3]}`
	after := `{"items":[1,2]}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LostFields) != 1 {
		t.Fatalf("expected 1 lost field (index 2), got %d: %+v", len(result.LostFields), result.LostFields)
	}
	if result.LostFields[0].Path != "items.2" {
		t.Fatalf("expected lost path=items.2, got %s", result.LostFields[0].Path)
	}
}

func TestJSONDiff_TypeMismatchScalar(t *testing.T) {
	// raw 是 string "123"，after 是 number 123 → changed
	raw := `{"value":"123"}`
	after := `{"value":123}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ChangedValues) != 1 {
		t.Fatalf("expected 1 changed value for type mismatch, got %d: %+v", len(result.ChangedValues), result.ChangedValues)
	}
}

func TestJSONDiff_EmptyObjects(t *testing.T) {
	result, err := JSONDiff([]byte(`{}`), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LostFields) != 0 || len(result.AddedFields) != 0 || len(result.ChangedValues) != 0 {
		t.Fatalf("expected empty diff for empty objects")
	}
}

func TestJSONDiff_NullVsBool(t *testing.T) {
	// null vs false → changed
	raw := `{"flag":null}`
	after := `{"flag":false}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ChangedValues) != 1 {
		t.Fatalf("expected 1 changed value (null→false), got %d", len(result.ChangedValues))
	}
}

func TestJSONDiff_StableOrder(t *testing.T) {
	// 多字段 lost，验证输出顺序稳定（按 key 排序）
	raw := `{"zebra":1,"apple":2,"mango":3}`
	after := `{}`

	result, err := JSONDiff([]byte(raw), []byte(after))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.LostFields) != 3 {
		t.Fatalf("expected 3 lost fields, got %d", len(result.LostFields))
	}
	expected := []string{"apple", "mango", "zebra"}
	for i, path := range expected {
		if result.LostFields[i].Path != path {
			t.Fatalf("LostFields[%d].Path = %s, want %s (sorted order)", i, result.LostFields[i].Path, path)
		}
	}
}

func TestJSONDiff_InvalidJSON(t *testing.T) {
	_, err := JSONDiff([]byte(`{invalid`), []byte(`{}`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
