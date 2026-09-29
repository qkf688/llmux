package testutil

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// APIResponseKey 是「响应键契约」表的一行：字面量键名 + 有值态 JSON 类型 + 三态可空标记。
//
// 键名必须与前端 webui/src/lib/api/modules/** 的 interface 字段名逐字一致——键名
// 失配不会报错，前端只会静默读到 undefined，这类断层唯一可见的地方是响应体 JSON，
// 故契约测试必须打在响应体上（AGENTS §5「直返实体 JSON 形状两侧同源」）。
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
}

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
func AssertAPIResponseKeys(t *testing.T, body, path string, want []APIResponseKey) {
	t.Helper()
	if err := apiResponseKeysErr(body, path, want); err != nil {
		t.Fatalf("AssertAPIResponseKeys: %v", err)
	}
}

// apiResponseKeysErr 是断言判据本体（返回 error 以便对门本身做冒烟覆盖）。
func apiResponseKeysErr(body, path string, want []APIResponseKey) error {
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
			continue
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
