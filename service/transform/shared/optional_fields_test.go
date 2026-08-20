package shared

import (
	"encoding/json"
	"testing"
)

// optionalFieldsProbe 汇集全部容器类型，用一个 struct 覆盖「宽容 + 三态」的所有分支。
type optionalFieldsProbe struct {
	Model         Optional[string]        `json:"model"`
	Stream        Optional[bool]          `json:"stream"`
	MaxTokens     OptionalNumber[int]     `json:"max_tokens"`
	Temperature   OptionalNumber[float64] `json:"temperature"`
	TopP          OptionalNumber[float64] `json:"top_p"`
	StopSequences OptionalStringSeq       `json:"stop_sequences"`
	Metadata      OptionalStringMap       `json:"metadata"`
	LogitBias     OptionalInt64Map        `json:"logit_bias"`
	Messages      RawArray                `json:"messages"`
}

// TestOptionalFields_LenientContract 钉死容器刻意反直觉的解析语义。
//
// 这些断言存在的唯一目的是拦住「顺手把 UnmarshalJSON 改成返回 err」「顺手把
// OptionalNumber 换成 Optional[int]」这类看似清理实则改变对外行为的重构——
// 网关的入站宽容度（类型不对 = 当没传，而不是 400）全靠本文件的实现兜着，
// 理由见 optional_fields.go 文件头。
func TestOptionalFields_LenientContract(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, got optionalFieldsProbe)
	}{
		{
			name: "整型字段接受浮点写法",
			body: `{"max_tokens":8192.0}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.MaxTokens.Set || got.MaxTokens.Value != 8192 {
					t.Fatalf("max_tokens = %v (set=%v), want 8192 (set=true) —— 换成 Optional[int] 会让标准库拒收 8192.0，字段静默丢失", got.MaxTokens.Value, got.MaxTokens.Set)
				}
			},
		},
		{
			name: "整型字段接受整数写法",
			body: `{"max_tokens":8192}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.MaxTokens.Set || got.MaxTokens.Value != 8192 {
					t.Fatalf("max_tokens = %v (set=%v), want 8192 (set=true)", got.MaxTokens.Value, got.MaxTokens.Set)
				}
			},
		},
		{
			name: "类型不匹配等同未传且不报错",
			body: `{"temperature":"0.5","model":123,"stream":"yes"}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if got.Temperature.Set || got.Temperature.Value != 0 {
					t.Fatalf(`temperature = %v (set=%v), want 0 (set=false) —— 让它报错会把 {"temperature":"0.5"} 整条请求打成 400`, got.Temperature.Value, got.Temperature.Set)
				}
				if got.Model.Set || got.Stream.Set {
					t.Fatalf("model.Set=%v stream.Set=%v, want 都为 false", got.Model.Set, got.Stream.Set)
				}
			},
		},
		{
			name: "null 等同未传",
			body: `{"top_p":null,"stop_sequences":null,"messages":null}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if got.TopP.Set {
					t.Fatalf("top_p.Set = true, want false（null 与键不存在同义）")
				}
				if got.StopSequences.Set {
					t.Fatalf("stop_sequences.Set = true, want false")
				}
				if got.Messages != nil {
					t.Fatalf("messages = %v, want nil", got.Messages)
				}
			},
		},
		{
			name: "显式零值与未传可分",
			body: `{"temperature":0,"stream":false,"model":""}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.Temperature.Set {
					t.Fatalf("temperature.Set = false, want true —— 少了 Set 第三态，temperature:0 出站时会被当成没传而静默丢失")
				}
				if !got.Stream.Set || !got.Model.Set {
					t.Fatalf("stream.Set=%v model.Set=%v, want 都为 true", got.Stream.Set, got.Model.Set)
				}
			},
		},
		{
			name: "空 body 全部保持未设置",
			body: `{}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if got.Model.Set || got.MaxTokens.Set || got.Temperature.Set || got.StopSequences.Set || got.Metadata.Set || got.LogitBias.Set {
					t.Fatalf("空 body 不应有任何字段 Set=true: %+v", got)
				}
				if got.Messages != nil {
					t.Fatalf("messages = %v, want nil", got.Messages)
				}
			},
		},
		{
			name: "字符串数组逐元素过滤坏元素",
			body: `{"stop_sequences":["a",1,"b",null]}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.StopSequences.Set {
					t.Fatalf("stop_sequences.Set = false, want true")
				}
				want := []string{"a", "b"}
				if len(got.StopSequences.Value) != len(want) {
					t.Fatalf("stop_sequences = %v, want %v —— 整体失败会让坏元素连带吃掉好元素", got.StopSequences.Value, want)
				}
				for i, item := range want {
					if got.StopSequences.Value[i] != item {
						t.Fatalf("stop_sequences = %v, want %v", got.StopSequences.Value, want)
					}
				}
			},
		},
		{
			name: "空数组保留 Set 但值为空",
			body: `{"stop_sequences":[]}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.StopSequences.Set || len(got.StopSequences.Value) != 0 {
					t.Fatalf("stop_sequences = %v (set=%v), want 空切片 (set=true)", got.StopSequences.Value, got.StopSequences.Set)
				}
			},
		},
		{
			name: "字典逐值过滤坏值",
			body: `{"metadata":{"a":"x","b":1},"logit_bias":{"a":1,"b":"x"}}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if !got.Metadata.Set || len(got.Metadata.Value) != 1 || got.Metadata.Value["a"] != "x" {
					t.Fatalf("metadata = %v (set=%v), want {a:x} (set=true)", got.Metadata.Value, got.Metadata.Set)
				}
				if !got.LogitBias.Set || len(got.LogitBias.Value) != 1 || got.LogitBias.Value["a"] != 1 {
					t.Fatalf("logit_bias = %v (set=%v), want {a:1} (set=true)", got.LogitBias.Value, got.LogitBias.Set)
				}
			},
		},
		{
			name: "RawArray 遇非数组保持 nil 而非空切片",
			body: `{"messages":"notarray"}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if got.Messages != nil {
					t.Fatalf("messages = %#v, want nil —— 置空切片会让下游「有 messages 但为空」与「没传 messages」不可分", got.Messages)
				}
			},
		},
		{
			name: "RawArray 元素原样保留不做元素级解析",
			body: `{"messages":[{"role":"user"},"scalar",42]}`,
			check: func(t *testing.T, got optionalFieldsProbe) {
				if len(got.Messages) != 3 {
					t.Fatalf("messages len = %d, want 3（元素形态判别属各协议子包职责，容器不过滤）", len(got.Messages))
				}
				if string(got.Messages[1]) != `"scalar"` {
					t.Fatalf("messages[1] = %s, want \"scalar\"", got.Messages[1])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got optionalFieldsProbe
			// 容器的 UnmarshalJSON 一律不返错，故任何字段级类型错误都不该冒到这里。
			if err := json.Unmarshal([]byte(tt.body), &got); err != nil {
				t.Fatalf("Unmarshal(%s) 返回错误 %v，want nil —— 容器必须吞掉字段级类型错误", tt.body, err)
			}
			tt.check(t, got)
		})
	}
}

func TestIsJSONNull(t *testing.T) {
	tests := []struct {
		data string
		want bool
	}{
		{"null", true},
		{" null ", true},
		{"\n\tnull\n", true},
		{`"null"`, false},
		{"NULL", false},
		{"nil", false},
		{"", false},
		{"0", false},
	}
	for _, tt := range tests {
		if got := IsJSONNull([]byte(tt.data)); got != tt.want {
			t.Errorf("IsJSONNull(%q) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestRawJSONValue(t *testing.T) {
	tests := []struct {
		name string
		data string
		want interface{}
	}{
		{name: "空输入返回 nil", data: "", want: nil},
		{name: "语法错误返回 nil", data: "{oops", want: nil},
		{name: "字面量 null 返回 nil", data: "null", want: nil},
		{name: "标量原样返回", data: `"text"`, want: "text"},
		{name: "数字统一为 float64", data: "42", want: float64(42)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RawJSONValue(json.RawMessage(tt.data)); got != tt.want {
				t.Errorf("RawJSONValue(%q) = %#v, want %#v", tt.data, got, tt.want)
			}
		})
	}

	// 对象与数组走类型断言，与调用方（asMap / asSlice）的消费方式一致。
	if _, ok := RawJSONValue(json.RawMessage(`{"a":1}`)).(map[string]interface{}); !ok {
		t.Error(`RawJSONValue({"a":1}) 未解成 map[string]interface{}`)
	}
	if _, ok := RawJSONValue(json.RawMessage(`[1,2]`)).([]interface{}); !ok {
		t.Error("RawJSONValue([1,2]) 未解成 []interface{}")
	}
}
