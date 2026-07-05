package maputil

import (
	"reflect"
	"testing"
)

func TestString(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"string": "value",
		"bool":   true,
	}

	if got := String(m, "string"); got != "value" {
		t.Fatalf("String matching type: want %q, got %q", "value", got)
	}
	if got := String(m, "bool"); got != "" {
		t.Fatalf("String wrong type: want empty string, got %q", got)
	}
	if got := String(m, "missing"); got != "" {
		t.Fatalf("String missing key: want empty string, got %q", got)
	}
}

func TestBool(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"bool":   true,
		"string": "value",
	}

	if got := Bool(m, "bool"); got != true {
		t.Fatalf("Bool matching type: want true, got %v", got)
	}
	if got := Bool(m, "string"); got != false {
		t.Fatalf("Bool wrong type: want false, got %v", got)
	}
	if got := Bool(m, "missing"); got != false {
		t.Fatalf("Bool missing key: want false, got %v", got)
	}
}

func TestFloat64(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"float":  12.5,
		"string": "value",
	}

	if got := Float64(m, "float"); got != 12.5 {
		t.Fatalf("Float64 matching type: want 12.5, got %v", got)
	}
	if got := Float64(m, "string"); got != 0 {
		t.Fatalf("Float64 wrong type: want 0, got %v", got)
	}
	if got := Float64(m, "missing"); got != 0 {
		t.Fatalf("Float64 missing key: want 0, got %v", got)
	}
}

func TestInt64(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"float":  12.9,
		"string": "value",
	}

	if got := Int64(m, "float"); got != 12 {
		t.Fatalf("Int64 matching type: want 12, got %v", got)
	}
	if got := Int64(m, "string"); got != 0 {
		t.Fatalf("Int64 wrong type: want 0, got %v", got)
	}
	if got := Int64(m, "missing"); got != 0 {
		t.Fatalf("Int64 missing key: want 0, got %v", got)
	}
}

func TestStringSlice(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"slice":  []interface{}{"one", 2.0, "two", false},
		"string": "value",
	}

	if got, want := StringSlice(m, "slice"), []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("StringSlice matching type: want %#v, got %#v", want, got)
	}
	if got := StringSlice(m, "string"); got != nil {
		t.Fatalf("StringSlice wrong type: want nil, got %#v", got)
	}
	if got := StringSlice(m, "missing"); got != nil {
		t.Fatalf("StringSlice missing key: want nil, got %#v", got)
	}
}

func TestStringMap(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"map":    map[string]interface{}{"one": "1", "two": 2.0, "three": "3"},
		"string": "value",
	}

	if got, want := StringMap(m, "map"), map[string]string{"one": "1", "three": "3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("StringMap matching type: want %#v, got %#v", want, got)
	}
	if got := StringMap(m, "string"); got != nil {
		t.Fatalf("StringMap wrong type: want nil, got %#v", got)
	}
	if got := StringMap(m, "missing"); got != nil {
		t.Fatalf("StringMap missing key: want nil, got %#v", got)
	}
}
