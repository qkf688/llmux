package shared

import (
	"encoding/json"
	"reflect"
	"testing"
)

// tagVariantsFixture 覆盖 encoding/json 的各种 tag 形态，用于验证反射派生的
// 「已认领键集合」与标准库实际序列化/反序列化认领的键完全一致。
type tagVariantsFixture struct {
	Plain   string `json:"plain"`
	Omit    int    `json:"omit,omitempty"`
	Skipped string `json:"-"`
	NoTag   bool

	// 未导出字段：encoding/json 不认领。
	unexported string

	// 匿名且无 tag：encoding/json 把内部字段提升到外层。
	promotedFixture

	// 匿名但有 tag：encoding/json 当作嵌套对象，只认领这一个顶层键。
	// 类型必须是导出的——匿名字段的类型未导出且带 tag 时标准库会整个跳过它。
	TaggedEmbedFixture `json:"tagged_embed"`

	// 具名嵌套：只认领 nested 这一个顶层键，不展开内部。
	Nested nestedFixture `json:"nested"`
}

type promotedFixture struct {
	Promoted string `json:"promoted"`
}

type TaggedEmbedFixture struct {
	Inner string `json:"inner"`
}

type nestedFixture struct {
	Deep string `json:"deep"`
}

func fullTagVariantsFixture() tagVariantsFixture {
	return tagVariantsFixture{
		Plain:              "p",
		Omit:               1,
		Skipped:            "s",
		NoTag:              true,
		unexported:         "u",
		promotedFixture:    promotedFixture{Promoted: "pr"},
		TaggedEmbedFixture: TaggedEmbedFixture{Inner: "in"},
		Nested:             nestedFixture{Deep: "d"},
	}
}

// 这是本文件最核心的断言：把「我们反射出的键集合」与「encoding/json 实际吐出的
// 顶层键」对齐。手写期望集合只能验证实现符合作者的理解，对齐标准库才能验证
// 实现符合真实的解析行为。
func TestClaimedJSONKeys_MatchesEncodingJSONBehavior(t *testing.T) {
	raw, err := json.Marshal(fullTagVariantsFixture())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	actualKeys, err := TopLevelJSONKeys(raw)
	if err != nil {
		t.Fatalf("TopLevelJSONKeys: %v", err)
	}
	actual := make(map[string]struct{}, len(actualKeys))
	for _, k := range actualKeys {
		actual[k] = struct{}{}
	}

	claimed := ClaimedJSONKeys(tagVariantsFixture{})
	if !reflect.DeepEqual(claimed, actual) {
		t.Fatalf("claimed keys diverge from encoding/json output\nclaimed: %v\nactual:  %v", claimed, actual)
	}

	// 冗余但明确：钉住几个容易实现错的形态，失败信息比集合 diff 更好读。
	for _, want := range []string{"plain", "omit", "NoTag", "promoted", "tagged_embed", "nested"} {
		if _, ok := claimed[want]; !ok {
			t.Errorf("expected claimed key %q", want)
		}
	}
	for _, notWant := range []string{"-", "Skipped", "unexported", "inner", "deep"} {
		if _, ok := claimed[notWant]; ok {
			t.Errorf("key %q must not be claimed", notWant)
		}
	}
}

func TestClaimedJSONKeys_AcceptsStructPointer(t *testing.T) {
	byValue := ClaimedJSONKeys(tagVariantsFixture{})
	byPointer := ClaimedJSONKeys(&tagVariantsFixture{})
	if !reflect.DeepEqual(byValue, byPointer) {
		t.Fatalf("pointer and value must claim the same keys\nvalue:   %v\npointer: %v", byValue, byPointer)
	}
}

func TestClaimedJSONKeys_PanicsOnNonStruct(t *testing.T) {
	cases := []struct {
		name  string
		input any
	}{
		{name: "nil", input: nil},
		{name: "string", input: "not a struct"},
		{name: "map", input: map[string]any{}},
		{name: "slice", input: []tagVariantsFixture{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic for non-struct input")
				}
			}()
			ClaimedJSONKeys(tc.input)
		})
	}
}

func TestTopLevelJSONKeys_Sorted(t *testing.T) {
	keys, err := TopLevelJSONKeys([]byte(`{"zeta":1,"alpha":2,"mid":3}`))
	if err != nil {
		t.Fatalf("TopLevelJSONKeys: %v", err)
	}
	if want := []string{"alpha", "mid", "zeta"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v (sorted for stable output)", keys, want)
	}
}

func TestTopLevelJSONKeys_RejectsNonObject(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "array", raw: `[{"a":1}]`},
		{name: "string", raw: `"a"`},
		{name: "number", raw: `1`},
		{name: "null", raw: `null`},
		{name: "empty", raw: ``},
		{name: "malformed", raw: `{"a":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := TopLevelJSONKeys([]byte(tc.raw)); err == nil {
				t.Fatalf("expected error for %s payload", tc.name)
			}
		})
	}
}

func TestUnknownTopLevelKeys(t *testing.T) {
	claimed := ClaimedJSONKeys(tagVariantsFixture{})

	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "unknown keys reported in sorted order",
			raw:  `{"plain":"p","zzz_vendor":1,"aaa_vendor":2}`,
			want: []string{"aaa_vendor", "zzz_vendor"},
		},
		{
			name: "fully claimed payload reports nothing",
			raw:  `{"plain":"p","omit":1,"NoTag":true,"promoted":"x","tagged_embed":{},"nested":{}}`,
			want: []string{},
		},
		{
			name: "empty object reports nothing",
			raw:  `{}`,
			want: []string{},
		},
		{
			name: "nested unknown keys are not inspected",
			raw:  `{"nested":{"totally_unknown":1}}`,
			want: []string{},
		},
		{
			name: "keys the DTO explicitly drops count as unknown",
			raw:  `{"Skipped":"s","unexported":"u"}`,
			want: []string{"Skipped", "unexported"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := UnknownTopLevelKeys([]byte(tc.raw), claimed)
			if err != nil {
				t.Fatalf("UnknownTopLevelKeys: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unknown = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUnknownTopLevelKeys_PropagatesParseError(t *testing.T) {
	if _, err := UnknownTopLevelKeys([]byte(`[1,2]`), ClaimedJSONKeys(tagVariantsFixture{})); err == nil {
		t.Fatal("expected error for non-object payload")
	}
}
