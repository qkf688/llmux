package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// 本文件提供「入站请求里有哪些键是本网关根本没在解析的」这一问题的计算原语。
//
// 为什么检测点放在入口侧而不是「入站 body 与出站 body 求键差」：改名与丢失在
// 输出侧不可区分（unified.Stop 出到 anthropic 叫 stop_sequences、出到 openai 叫
// stop），靠手工白名单补这个信息缺口等于维护转换逻辑的第二份副本，必然漂移成
// 误报源。而「DTO 认领了哪些键」是从 struct tag 直接反射出来的事实，DTO 改了它
// 自动跟着改，零维护且不受出站改名影响。
//
// 三个函数都是纯函数：无 ctx、无 IO、无包级状态。反射结果刻意不缓存——缓存会
// 引入包级可变 map，而调用方是低频的诊断/展示路径，反射成本可忽略。

// ClaimedJSONKeys 返回 v 的类型在 encoding/json 下会认领的顶层键集合。
//
// 语义与 encoding/json 对齐：`json:"-"` 与未导出字段不认领；无 tag 的导出字段
// 用字段名；匿名嵌入且无 tag 名时字段被提升到外层（递归展开），带 tag 名时只
// 认领该 tag 本身。
//
// v 必须是 struct 或 struct 指针（可传零值）。传其他类型是调用方的编程错误，
// 直接 panic——静默返回空集合会让调用方把「所有键都未知」当成正常结果。
func ClaimedJSONKeys(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("shared: ClaimedJSONKeys requires a struct or struct pointer, got %T", v))
	}

	keys := make(map[string]struct{})
	walkClaimedJSONFields(t, nil, func(name string, _ []int) {
		keys[name] = struct{}{}
	})
	return keys
}

// walkClaimedJSONFields 按 encoding/json 的可见性规则遍历 t，对每个会认领顶层键的
// 字段回调一次 `(键名, 字段索引路径)`。索引路径供需要取字段值的调用方用
// `reflect.Value.FieldByIndexErr` 定位。
//
// 键名推导与嵌入提升规则只在此处实现一份：本包有两个按 json 键反射 DTO 的检测
// （认领键集合与类型不匹配），各写一遍必然分叉——曾经就分叉过，Type 侧对 nil 匿名
// 嵌入指针递归、Value 侧不递归，同一个 DTO 在「认领了哪些键」与「哪些键被丢弃」上
// 给出矛盾答案。
//
// 规则：`json:"-"` 与未导出字段不认领；无 tag 名的导出字段用字段名；匿名嵌入且无
// tag 名时递归展开到外层，但嵌入的不是 struct 时退回按普通字段认领。
func walkClaimedJSONFields(t reflect.Type, prefix []int, visit func(name string, index []int)) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := tag
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}

		index := append(append([]int(nil), prefix...), i)

		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				walkClaimedJSONFields(embedded, index, visit)
				continue
			}
		}

		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		visit(name, index)
	}
}

// TopLevelJSONKeys 返回 raw 这个 JSON 对象的顶层键，已排序以保证输出稳定。
//
// 只看顶层：嵌套层（messages[].xxx 之类）的键归属需要逐个子 DTO 的映射知识，
// 不在本函数职责内。
func TopLevelJSONKeys(raw []byte) ([]string, error) {
	obj, err := decodeTopLevelObject(raw)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

// decodeTopLevelObject 把 raw 解成顶层键 → 原始值的映射，是本包所有「按顶层键
// 检查」函数的共同入口（TopLevelJSONKeys 与 MismatchedTopLevelKeys）。
//
// 抽出来是为了让两者对「什么算不合法输入」给出同一个答案：合法 JSON 但不是对象
// （例如字面量 null）时，键的概念不成立，必须报错而不是返回空集合——空集合会让
// 调用方把「问不了」当成「已检查、没问题」。
func decodeTopLevelObject(raw []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parse top-level JSON object: %w", err)
	}
	if obj == nil {
		return nil, errors.New("payload is not a JSON object")
	}
	return obj, nil
}

// UnknownTopLevelKeys 返回 raw 里出现、但 claimed 未认领的顶层键，已排序。
//
// 比对规则刻意与 encoding/json 的字段匹配对齐：**先精确、精确未命中再忽略大小写**。
// claimed 是按精确 tag 名派生的，若这里只比精确名，`{"Temperature":0.7}` 这类会被
// 标准库正常解析进字段的键会被报成未知——诊断功能一上线就假报，比没有更糟。
//
// 用 strings.EqualFold 线性兜底而非预建小写索引：折叠语义直接借标准库的实现，不用
// 自己造一套可能与之漂移的归一化；且本函数是低频诊断路径，键数量级下的 O(n*m) 无意义。
func UnknownTopLevelKeys(raw []byte, claimed map[string]struct{}) ([]string, error) {
	keys, err := TopLevelJSONKeys(raw)
	if err != nil {
		return nil, err
	}

	unknown := make([]string, 0, len(keys))
	for _, key := range keys {
		if !isClaimedKey(claimed, key) {
			unknown = append(unknown, key)
		}
	}
	return unknown, nil
}

func isClaimedKey(claimed map[string]struct{}, key string) bool {
	_, ok := lookupJSONKeyFold(claimed, key)
	return ok
}

// lookupJSONKeyFold 按 encoding/json 的键匹配口径查 m：先精确、精确未命中再 EqualFold。
//
// 泛型而非各处手写：本包两个检测都要按同一口径查（认领键集合查 struct{}、类型不匹配
// 查字段索引），口径一分叉就会出现「A 说这个键已认领、B 说查不到对应字段」的矛盾。
func lookupJSONKeyFold[V any](m map[string]V, key string) (V, bool) {
	if value, ok := m[key]; ok {
		return value, true
	}
	for name, value := range m {
		if strings.EqualFold(name, key) {
			return value, true
		}
	}
	var zero V
	return zero, false
}
