package shared

import (
	"encoding/json"
	"reflect"
	"testing"
)

// mismatchProbe 覆盖全部 5 种三态容器 + 两种无三态的字段形态（RawMessage / RawArray），
// 用来钉死「哪些字段类型在检测范围内」这条边界。
type mismatchProbe struct {
	Model       Optional[string]        `json:"model"`
	Stream      Optional[bool]          `json:"stream"`
	MaxTokens   OptionalNumber[int]     `json:"max_tokens"`
	Temperature OptionalNumber[float64] `json:"temperature"`
	Stops       OptionalStringSeq       `json:"stops"`
	Metadata    OptionalStringMap       `json:"metadata"`
	LogitBias   OptionalInt64Map        `json:"logit_bias"`
	Raw         json.RawMessage         `json:"raw"`
	Items       RawArray                `json:"items"`
	Ignored     Optional[string]        `json:"-"`
	unexported  Optional[string]        //nolint:unused // 只为验证未导出字段不参与匹配
}

func TestMismatchedTopLevelKeys(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "类型全部匹配时无命中",
			raw:  `{"model":"m","stream":true,"max_tokens":8,"temperature":0.7}`,
			want: []string{},
		},
		{
			// 全宽容容器把类型不匹配当成没传，客户端拿不到任何反馈——正是本检测要暴露的。
			name: "标量类型不匹配被报出",
			raw:  `{"model":123,"stream":"yes","temperature":"0.5"}`,
			want: []string{"model", "stream", "temperature"},
		},
		{
			// OptionalNumber 走 float64 中转，8192.0 解进 int 字段是刻意保留的宽容度，
			// 不算不匹配。
			name: "浮点写法的整数字段不算不匹配",
			raw:  `{"max_tokens":8192.0}`,
			want: []string{},
		},
		{
			name: "数组与字典容器的类型不匹配被报出",
			raw:  `{"stops":"x","metadata":[1],"logit_bias":"y"}`,
			want: []string{"logit_bias", "metadata", "stops"},
		},
		{
			// 逐元素过滤是既有语义（保留合法项、丢掉非法项），容器 Set=true，不算不匹配。
			name: "数组逐元素过滤不算不匹配",
			raw:  `{"stops":["a",1],"logit_bias":{"a":1,"b":"x"}}`,
			want: []string{},
		},
		{
			// null 与「键不存在」在本项目里同义，客户端传 null 就是表达不设置。
			name: "null 不算类型不匹配",
			raw:  `{"model":null,"temperature":null}`,
			want: []string{},
		},
		{
			// 未认领键归 UnknownTopLevelKeys 管，本函数不重复上报。
			name: "未认领键不在本检测范围",
			raw:  `{"top_k":40,"vendor_knob":"x"}`,
			want: []string{},
		},
		{
			// RawMessage / RawArray 无三态可查：前者对任何合法 JSON 都不失败，
			// 后者「非数组」与「空数组」都落在零值附近。已知缺口，不报假阳性。
			name: "无三态的字段类型不在检测范围",
			raw:  `{"raw":"anything","items":"not-an-array"}`,
			want: []string{},
		},
		{
			// 匹配规则与未认领检测共用 lookupJSONKeyFold：先精确、未命中再 EqualFold。
			// 口径必须同源，否则大小写变体会被判成已认领却查不到字段、漏报丢弃。
			name: "大小写变体也能定位到字段",
			raw:  `{"Temperature":"0.5"}`,
			want: []string{"Temperature"},
		},
		{
			// `json:"-"` 的字段不认领任何键，同名键属于未认领而非丢弃。
			name: "json 忽略的字段不参与匹配",
			raw:  `{"Ignored":123}`,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MismatchedTopLevelKeys([]byte(tt.raw), &mismatchProbe{})
			if err != nil {
				t.Fatalf("MismatchedTopLevelKeys: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MismatchedTopLevelKeys = %v, want %v", got, tt.want)
			}
		})
	}
}

// 输出必须排序，否则同一份 body 每次调用给出的顺序不同（map 迭代无序），
// 前端展示与测试断言都会抖动。
func TestMismatchedTopLevelKeys_Sorted(t *testing.T) {
	got, err := MismatchedTopLevelKeys([]byte(`{"temperature":"x","model":1,"stream":"y"}`), &mismatchProbe{})
	if err != nil {
		t.Fatalf("MismatchedTopLevelKeys: %v", err)
	}
	want := []string{"model", "stream", "temperature"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MismatchedTopLevelKeys = %v, want %v", got, want)
	}
}

// 非对象 body 必须报错而非返回空集合：空集合与「已检查、无命中」不可区分。
// 判据须与 TopLevelJSONKeys 一致（两者共用 decodeTopLevelObject）。
func TestMismatchedTopLevelKeys_RejectsNonObject(t *testing.T) {
	for _, raw := range []string{`null`, `[1,2]`, `"str"`, `5`, ``, `{"model":`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := MismatchedTopLevelKeys([]byte(raw), &mismatchProbe{}); err == nil {
				t.Fatalf("非 JSON 对象必须报错，raw=%q", raw)
			}
		})
	}
}

func TestMismatchedTopLevelKeys_PanicsOnNonStruct(t *testing.T) {
	tests := []struct {
		name string
		dto  any
	}{
		{name: "nil", dto: nil},
		{name: "非 struct 指针", dto: new(string)},
		{name: "非 struct 值", dto: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("非 struct 类型必须 panic：静默返回空集合会被当成「已检查、无问题」")
				}
			}()
			_, _ = MismatchedTopLevelKeys([]byte(`{}`), tt.dto)
		})
	}
}

// dto 只作类型样板：值、非 nil 指针、nil 指针三种写法必须给出同一结果，
// 且调用方传进来的实例不被写入——否则复用同一个 DTO 的调用方会被静默改写字段。
func TestMismatchedTopLevelKeys_TreatsDTOAsTypeTemplateOnly(t *testing.T) {
	raw := []byte(`{"model":123}`)
	want := []string{"model"}

	for name, dto := range map[string]any{
		"值":      mismatchProbe{},
		"指针":     &mismatchProbe{},
		"nil 指针": (*mismatchProbe)(nil),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := MismatchedTopLevelKeys(raw, dto)
			if err != nil {
				t.Fatalf("MismatchedTopLevelKeys: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("MismatchedTopLevelKeys = %v, want %v", got, want)
			}
		})
	}

	caller := &mismatchProbe{}
	if _, err := MismatchedTopLevelKeys([]byte(`{"model":"m"}`), caller); err != nil {
		t.Fatalf("MismatchedTopLevelKeys: %v", err)
	}
	if caller.Model.Set {
		t.Fatal("调用方传入的实例不应被写入：签名只承诺返回键名，改写字段是隐藏副作用")
	}
}

// 匿名嵌入的字段被提升到外层，键名与提升规则复用 walkClaimedJSONFields，
// 与 ClaimedJSONKeys 同源——否则同一个 DTO 在「认领了哪些键」与「哪些键被丢弃」
// 上给出矛盾的答案。
func TestMismatchedTopLevelKeys_PromotesEmbeddedFields(t *testing.T) {
	type inner struct {
		Nested Optional[string] `json:"nested"`
	}
	type outer struct {
		inner
		Own Optional[string] `json:"own"`
	}

	got, err := MismatchedTopLevelKeys([]byte(`{"nested":1,"own":"ok"}`), &outer{})
	if err != nil {
		t.Fatalf("MismatchedTopLevelKeys: %v", err)
	}
	want := []string{"nested"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MismatchedTopLevelKeys = %v, want %v", got, want)
	}
}

// 认领键集合与丢弃键检测走同一个字段遍历器，故对同一 DTO 的键推导必须完全一致。
// 本测试是那条同源约束的可执行断言：认领集合里没有的键，丢弃检测也不该认得。
func TestMismatchedTopLevelKeys_AgreesWithClaimedJSONKeys(t *testing.T) {
	claimed := ClaimedJSONKeys(&mismatchProbe{})

	// mismatchProbe 的每个键都必须两侧一致：认领集合有的，类型写错时能被报出。
	for key := range claimed {
		raw := []byte(`{"` + key + `":` + mismatchingValueFor(key) + `}`)
		got, err := MismatchedTopLevelKeys(raw, &mismatchProbe{})
		if err != nil {
			t.Fatalf("MismatchedTopLevelKeys(%s): %v", key, err)
		}
		// RawMessage / RawArray 无三态，是文档化的缺口，不参与断言。
		if key == "raw" || key == "items" {
			continue
		}
		if !reflect.DeepEqual(got, []string{key}) {
			t.Fatalf("认领了 %q 却没报出类型不匹配: got %v", key, got)
		}
	}

	// 反向：`json:"-"` 与未导出字段两侧都不认领。
	if _, ok := claimed["Ignored"]; ok {
		t.Fatal(`json:"-"` + " 的字段不应进认领集合")
	}
}

// mismatchingValueFor 给出必然与该字段类型不符的 JSON 值。
func mismatchingValueFor(key string) string {
	switch key {
	case "model":
		return `123`
	default:
		// 其余字段分别是 bool / 数值 / 数组 / 字典，字符串对它们一律不匹配。
		return `"x"`
	}
}

// 前提校验：断言这些 body 确实被容器静默吞掉（Set=false），而不是 json.Unmarshal
// 直接报错。若哪天容器改成上报错误，本测试会先失败，提醒检测的判据已失效。
func TestMismatchedTopLevelKeys_ContainersSwallowMismatchSilently(t *testing.T) {
	var probe mismatchProbe
	if err := json.Unmarshal([]byte(`{"model":123,"temperature":"0.5"}`), &probe); err != nil {
		t.Fatalf("容器应吞掉类型不匹配而非报错: %v", err)
	}
	if probe.Model.Set {
		t.Fatal("model 类型不匹配时 Set 应为 false")
	}
	if probe.Temperature.Set {
		t.Fatal("temperature 类型不匹配时 Set 应为 false")
	}
}
