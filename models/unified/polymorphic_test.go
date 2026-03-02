package unified

import (
	"encoding/json"
	"testing"
)

func TestUnifiedStopJSON(t *testing.T) {
	single := "END"
	stop := UnifiedStop{Single: &single}
	data, err := json.Marshal(stop)
	if err != nil {
		t.Fatalf("Marshal(single) error: %v", err)
	}
	if string(data) != `"END"` {
		t.Fatalf("Marshal(single) = %s, want \"END\"", data)
	}

	stop = UnifiedStop{Multiple: []string{"END", "STOP"}}
	data, err = json.Marshal(stop)
	if err != nil {
		t.Fatalf("Marshal(multiple) error: %v", err)
	}
	if string(data) != `["END","STOP"]` {
		t.Fatalf("Marshal(multiple) = %s", data)
	}

	var parsed UnifiedStop
	if err := json.Unmarshal([]byte(`"DONE"`), &parsed); err != nil {
		t.Fatalf("Unmarshal(string) error: %v", err)
	}
	if parsed.Single == nil || *parsed.Single != "DONE" {
		t.Fatalf("Unmarshal(string) unexpected result: %+v", parsed)
	}

	if err := json.Unmarshal([]byte(`["A","B"]`), &parsed); err != nil {
		t.Fatalf("Unmarshal(array) error: %v", err)
	}
	if len(parsed.Multiple) != 2 || parsed.Multiple[0] != "A" || parsed.Multiple[1] != "B" {
		t.Fatalf("Unmarshal(array) unexpected result: %+v", parsed)
	}

	if err := json.Unmarshal([]byte(`123`), &parsed); err == nil {
		t.Fatalf("Unmarshal(invalid) expected error")
	}
}

func TestUnifiedToolChoiceJSON(t *testing.T) {
	auto := "auto"
	choice := UnifiedToolChoice{StringValue: &auto}
	data, err := json.Marshal(choice)
	if err != nil {
		t.Fatalf("Marshal(string) error: %v", err)
	}
	if string(data) != `"auto"` {
		t.Fatalf("Marshal(string) = %s, want \"auto\"", data)
	}

	choice = UnifiedToolChoice{
		ObjectValue: &UnifiedToolChoiceObject{
			Type: "function",
			Function: &UnifiedToolChoiceFunction{
				Name: "lookup",
			},
		},
	}
	data, err = json.Marshal(choice)
	if err != nil {
		t.Fatalf("Marshal(object) error: %v", err)
	}

	var parsed UnifiedToolChoice
	if err := json.Unmarshal([]byte(`"required"`), &parsed); err != nil {
		t.Fatalf("Unmarshal(string) error: %v", err)
	}
	if parsed.StringValue == nil || *parsed.StringValue != "required" {
		t.Fatalf("Unmarshal(string) unexpected result: %+v", parsed)
	}

	objRaw := `{"type":"function","function":{"name":"lookup"}}`
	if err := json.Unmarshal([]byte(objRaw), &parsed); err != nil {
		t.Fatalf("Unmarshal(object) error: %v", err)
	}
	if parsed.ObjectValue == nil || parsed.ObjectValue.Type != "function" || parsed.ObjectValue.Function == nil || parsed.ObjectValue.Function.Name != "lookup" {
		t.Fatalf("Unmarshal(object) unexpected result: %+v", parsed)
	}

	if err := json.Unmarshal([]byte(`123`), &parsed); err == nil {
		t.Fatalf("Unmarshal(invalid) expected error")
	}
}

func TestUnifiedEmbeddingInputJSON(t *testing.T) {
	single := "text"
	input := UnifiedEmbeddingInput{Single: &single}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal(single) error: %v", err)
	}
	if string(data) != `"text"` {
		t.Fatalf("Marshal(single) = %s, want \"text\"", data)
	}

	input = UnifiedEmbeddingInput{Multiple: []string{"a", "b"}}
	data, err = json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal(multiple) error: %v", err)
	}
	if string(data) != `["a","b"]` {
		t.Fatalf("Marshal(multiple) = %s", data)
	}

	var parsed UnifiedEmbeddingInput
	if err := json.Unmarshal([]byte(`"item"`), &parsed); err != nil {
		t.Fatalf("Unmarshal(string) error: %v", err)
	}
	if parsed.Single == nil || *parsed.Single != "item" {
		t.Fatalf("Unmarshal(string) unexpected result: %+v", parsed)
	}

	if err := json.Unmarshal([]byte(`["x","y"]`), &parsed); err != nil {
		t.Fatalf("Unmarshal(array) error: %v", err)
	}
	if len(parsed.Multiple) != 2 || parsed.Multiple[0] != "x" || parsed.Multiple[1] != "y" {
		t.Fatalf("Unmarshal(array) unexpected result: %+v", parsed)
	}

	if err := json.Unmarshal([]byte(`{}`), &parsed); err == nil {
		t.Fatalf("Unmarshal(invalid) expected error")
	}
}
