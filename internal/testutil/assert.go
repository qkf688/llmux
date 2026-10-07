package testutil

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// APIResponseKey 是「响应键契约」表的一行：字面量键名 + 有值态 JSON 类型 + 可空标记。
//
// 表覆盖**响应全键**（键集合逐字相等要求如此），不只是前端会读的键：键名与前端
// webui/src/lib/api/modules/** 的 interface 字段名逐字一致，但前端不消费的键
// （ORM 托管键如 CreatedAt/UpdatedAt/DeletedAt、后端专用计数键如 ConsecutiveFailures）
// 同样必须列行，否则集合同等断言会报「多键」。键名失配不会报错，前端只会静默读到
// undefined，这类断层唯一可见的地方是响应体 JSON，故契约测试必须打在响应体上
// （AGENTS §5「直返实体 JSON 形状两侧同源」）。
type APIResponseKey struct {
	// Key 响应体里的键名（字面量，禁止从结构体反射推导——结构体一改断言跟着改，
	// 恰好放过要抓的「形状变了没同步前端」）。
	Key string
	// WantSetType 该键有值时的 JSON 类型（bool 类型字段的用例统一置 true，写 gjson.True）。
	WantSetType gjson.Type
	// Nilable 标记该键承载「继承 / 未设置」这一有效语义：未设置时必须序列化成
	// JSON null 且键仍存在，禁止 omitempty 省略整键（省略会让前端把继承误判成
	// override 并在保存时回写错误值）。
	Nilable bool
	// AlwaysNull 标记该键在存活行上**恒为 JSON null**（`gorm.Model` 的 DeletedAt：
	// 软删标记位，未软删行恒 null）。列进表是为了让键集合逐字相等成立；它不参与
	// 三态方向校验——恒 null 是 ORM 托管形态，不是「继承 / 未设置」的有效语义，
	// 没有「有值态」可言。标 AlwaysNull 必须同时标 Nilable（表自相矛盾会被判红）。
	//
	// **误标代价**：把一条真三态键误标 AlwaysNull（例如从 DeletedAt 行复制），该键的
	// 方向校验就被永久豁免——它在两态用例里都恰好为 null 时全绿，方向漂移无人发现。
	// 无反射推导就无法机械闭合，标之前先确认「该键在任何存活行上都不可能非 null」。
	AlwaysNull bool
}

// KeyState 指定本次断言观测的是「有值态」还是「空值态」，把三态字段的两个方向
// 都钉死——同一张契约表按两种态各断言一次。
//
// 为什么是调用级参数而不是表里的 flag：态是**用例（前置数据）**的属性，不是键的属性。
// 同一个键在有值态用例里必须非 null、在空值态用例里必须为 null——写进表里就说不通。
type KeyState int

const (
	// KeyStateAny 不校验三态方向：键为 null 时只要求该键在表里标了 Nilable。
	// 这是既有调用方的语义（它们按「有值 / 空值」两张字面量表各断言一次键集合）。
	KeyStateAny KeyState = iota
	// KeyStateSet 有值态：Nilable 键必须有值（为 null 即红），且类型 == WantSetType。
	// 缺了它，把该键的值写成 null 也能全绿——「有值」这个前提就没人守。
	// 恒 null 键（AlwaysNull）不受这一向约束。
	KeyStateSet
	// KeyStateUnset 空值态：Nilable 键必须为 JSON null（非 null 即红）。
	// 缺了它，该键悄悄被赋上值（继承语义被破坏）也能全绿。恒 null 键不受这一向约束。
	KeyStateUnset
)

// AssertAPIResponseKeys 断言 body 在 path 处对象的键集合与 want 表**逐字相等**：
// 缺键、多键、改名都红。响应形状的任何变化必须先改表、再改代码，不允许静默漂移。
//
//   - 逐键校验用「键在不在」而非「取值是否为空」：键缺失与键存在且为 null 在取值上
//     无法区分，只断取值会让 omitempty 类断层全部漏过。值为 null 时要求 Nilable，
//     否则要求类型 == WantSetType。
//   - 可选键（omitempty 契约，如 models.Model 的 thinking_levels：空值态整键不出现）
//     不进本工具的可选态——有值态与空值态各一张字面量表分别断言，工具只认
//     「键集合逐字相等」，让「键到底出不出现」显式写在测试里而不是藏在 flag 里。
//   - path 用 gjson 路径：根对象传 ""，数组元素传 "data.0" 这类形式。
//
// 三态字段（Nilable）的当前态由 AssertAPIResponseKeysInState 指定；本函数等价于
// 传 KeyStateAny（不校验方向），既有调用方语义与结论都不变。
func AssertAPIResponseKeys(t *testing.T, body, path string, want []APIResponseKey) {
	t.Helper()
	AssertAPIResponseKeysInState(t, body, path, want, KeyStateAny)
}

// AssertAPIResponseKeysInState 是 AssertAPIResponseKeys 加三态方向校验的版本：
// 同一张契约表在有值态用例里传 KeyStateSet、在空值态用例里传 KeyStateUnset，
// 两个方向都钉死。传 KeyStateAny 时行为与 AssertAPIResponseKeys 完全一致。
func AssertAPIResponseKeysInState(t *testing.T, body, path string, want []APIResponseKey, state KeyState) {
	t.Helper()
	if err := apiResponseKeysErrInState(body, path, want, state); err != nil {
		t.Fatalf("AssertAPIResponseKeys: %v", err)
	}
}

// apiResponseKeysErrInState 是断言判据本体（返回 error 以便对门本身做冒烟覆盖）。
func apiResponseKeysErrInState(body, path string, want []APIResponseKey, state KeyState) error {
	// 未知态必须当场报错，不得静默退化成「不校验方向」：否则将来新增或误算的 KeyState
	// 值会让方向校验整体 fail-open，正是本门要消灭的「该红却绿」形态。
	switch state {
	case KeyStateAny, KeyStateSet, KeyStateUnset:
	default:
		return fmt.Errorf("未知 KeyState(%d)：拒绝静默降级为「不校验三态方向」", int(state))
	}

	var obj gjson.Result
	if strings.TrimSpace(path) == "" {
		// gjson.Get(body, "") 取不到根：根对象直接 Parse。
		obj = gjson.Parse(body)
	} else {
		obj = gjson.Get(body, path)
	}
	if !obj.Exists() || !obj.IsObject() {
		return fmt.Errorf("%s 处不是对象（或不存在），body=%s", pathLabel(path), body)
	}

	gotTypes := map[string]gjson.Type{}
	obj.ForEach(func(k, v gjson.Result) bool {
		gotTypes[k.String()] = v.Type
		return true
	})

	wantKeys := make([]string, 0, len(want))
	wantByKey := make(map[string]APIResponseKey, len(want))
	for _, k := range want {
		if _, dup := wantByKey[k.Key]; dup {
			return fmt.Errorf("契约表键 %q 重复", k.Key)
		}
		// 恒 null 键不给「null 是否合法」留后门：没标 Nilable 就等于要工具放行一次
		// 未声明的 null，而这正是本工具要抓的那类静默漂移。
		if k.AlwaysNull && !k.Nilable {
			return fmt.Errorf("契约表键 %q 标了 AlwaysNull 却未标 Nilable（自相矛盾：恒 null 的键必须允许 null）", k.Key)
		}
		wantByKey[k.Key] = k
		wantKeys = append(wantKeys, k.Key)
	}

	var missing, extra []string
	for _, k := range wantKeys {
		if _, ok := gotTypes[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range gotTypes {
		if _, ok := wantByKey[k]; !ok {
			extra = append(extra, k)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		return fmt.Errorf("%s 键集合与契约表不一致（契约漂移：改形状必须先改表）\n缺键: %v\n多键: %v\nbody=%s",
			pathLabel(path), missing, extra, body)
	}

	for _, kc := range want {
		gotType := gotTypes[kc.Key]
		if gotType == gjson.Null {
			if !kc.Nilable {
				return fmt.Errorf("%s.%s 为 null，但契约表未标记 Nilable（三态字段才允许 null；"+
					"键必须始终在、null 与缺键是两回事），body=%s", pathLabel(path), kc.Key, body)
			}
			if state == KeyStateSet && !kc.AlwaysNull {
				return fmt.Errorf("%s.%s 为 null，但本用例观测有值态（KeyStateSet）：三态键在有值态下必须有值，"+
					"否则「有值」这个前提没人守、键类型漂移会从这条出口漏过，body=%s", pathLabel(path), kc.Key, body)
			}
			continue
		}
		if kc.Nilable && !kc.AlwaysNull && state == KeyStateUnset {
			return fmt.Errorf("%s.%s 有值（type = %v），但本用例观测空值态（KeyStateUnset）：三态键在空值态下"+
				"必须为 JSON null（继承语义被悄悄写成显式值会让前端误判 override），body=%s",
				pathLabel(path), kc.Key, gotType, body)
		}
		if kc.AlwaysNull {
			// 恒 null 键突然有值 = ORM 托管形态变了（软删位被写值，或被换成了别的字段），
			// 与键集合漂移一样该红——表里写着恒 null，就不许悄悄变成有值。
			return fmt.Errorf("%s.%s 有值（type = %v），但契约表标了 AlwaysNull（恒为 null 的 ORM 托管键）；"+
				"形态变了就先改表再说，body=%s", pathLabel(path), kc.Key, gotType, body)
		}
		if gotType != kc.WantSetType {
			return fmt.Errorf("%s.%s type = %v, want %v, body=%s",
				pathLabel(path), kc.Key, gotType, kc.WantSetType, body)
		}
	}
	return nil
}

// pathLabel 让根对象的失败信息可读（gjson 根路径是空串）。
func pathLabel(path string) string {
	if strings.TrimSpace(path) == "" {
		return "<root>"
	}
	return strings.TrimSuffix(path, ".")
}
