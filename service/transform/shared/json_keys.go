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
	collectClaimedJSONKeys(t, keys)
	return keys
}

func collectClaimedJSONKeys(t reflect.Type, keys map[string]struct{}) {
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

		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				collectClaimedJSONKeys(embedded, keys)
				continue
			}
		}

		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		keys[name] = struct{}{}
	}
}

// TopLevelJSONKeys 返回 raw 这个 JSON 对象的顶层键，已排序以保证输出稳定。
//
// 只看顶层：嵌套层（messages[].xxx 之类）的键归属需要逐个子 DTO 的映射知识，
// 不在本函数职责内。
func TopLevelJSONKeys(raw []byte) ([]string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parse top-level JSON object: %w", err)
	}
	if obj == nil {
		// 合法 JSON 但不是对象（例如字面量 null），键的概念不成立。
		return nil, errors.New("payload is not a JSON object")
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

// UnknownTopLevelKeys 返回 raw 里出现、但 claimed 未认领的顶层键，已排序。
func UnknownTopLevelKeys(raw []byte, claimed map[string]struct{}) ([]string, error) {
	keys, err := TopLevelJSONKeys(raw)
	if err != nil {
		return nil, err
	}

	unknown := make([]string, 0, len(keys))
	for _, key := range keys {
		if _, ok := claimed[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	return unknown, nil
}
