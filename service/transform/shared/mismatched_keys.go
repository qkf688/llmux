package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// 本文件回答「哪些顶层键被本网关**认领了、但值的类型不对而被静默丢弃**」。
//
// 与 json_keys.go 的未认领键检测是互补的两类静默：未认领是「DTO 里根本没这个键」，
// 本文件是「DTO 有这个键，但 optional_fields.go 的宽容容器把类型不匹配当成没传」。
// 后者对客户端更隐蔽——请求 200 通过、参数却没生效，且日志里的入站 body 明明写着
// 那个值。宽容度本身是刻意的（见 optional_fields.go 顶部），所以这里的解法不是改成
// 400，而是**让静默可见**。
//
// 判据直接复用容器已有的 Set 三态，不新增字段：body 顶层有该键、值不是 null、而
// 对应字段 Set 仍为 false ⇒ 只可能是容器吞掉了类型不匹配。

// optionalSetFieldName 是容器暴露「有没有成功取到值」的字段名。
//
// 靠字段名而非接口断言识别容器：容器是泛型类型（Optional[T] 等），为它们定义统一
// 接口要么得给每个实例化类型写方法、要么得把 Set 提升成方法——前者重复、后者会让
// 容器多出一个仅为检测存在的导出方法。字段名约定只在本包内成立（容器就在隔壁文件），
// 加新容器时保持 Set 字段即自动被覆盖。
const optionalSetFieldName = "Set"

// MismatchedTopLevelKeys 返回 raw 里出现、被 dto 认领、但值类型不匹配而被丢弃的顶层键，已排序。
//
// dto 只作**类型样板**：本函数内部另建一个零值实例来承接解码，不写入调用方传进来的
// 实例。这样它是纯函数——签名只返回 []string，调用方不必知道「解码需要一块暂存区」，
// 也不会因为复用同一个 DTO 而被静默改写字段或在并发下互相污染 Set 位。
//
// dto 必须是 struct 或 struct 指针（可传零值），理由同 ClaimedJSONKeys：传其他类型是
// 调用方的编程错误，直接 panic——静默返回空集合会让调用方把「查不了」当成「没问题」。
//
// 覆盖范围只有带 Set 三态的容器字段。`json.RawMessage` / `RawArray` 类型的字段
// **不在范围内**：RawMessage 对任何合法 JSON 都不失败（谈不上丢弃），RawArray 的
// 「非数组」与「空数组」都落在零值附近、无三态可查。这是已知缺口而非遗漏，报告出来
// 的是真丢弃、不含假阳性。
//
// null 不算类型不匹配：本项目里 null 与「键不存在」同义（见 IsJSONNull），客户端传
// null 就是表达「不设置」，容器 Set=false 是正确结果。
func MismatchedTopLevelKeys(raw []byte, dto any) ([]string, error) {
	t := reflect.TypeOf(dto)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("shared: MismatchedTopLevelKeys requires a struct or struct pointer, got %T", dto))
	}

	obj, err := decodeTopLevelObject(raw)
	if err != nil {
		return nil, err
	}

	probe := reflect.New(t)

	// DTO 解析失败仍返回错误而非空集合：入站容器按契约全宽容（吞掉类型不匹配），
	// 所以这里的 err 只可能来自某个**非容器**字段的自定义 UnmarshalJSON。那种字段
	// 会把整条请求打成错误、根本不存在「静默丢弃」，此时给不出可信结论，必须让调用
	// 方知道「查不了」而不是「已检查、无问题」。
	if err := json.Unmarshal(raw, probe.Interface()); err != nil {
		return nil, fmt.Errorf("decode request into inbound DTO: %w", err)
	}

	fields := claimedFieldsByJSONKey(probe.Elem())

	mismatched := make([]string, 0)
	for key, rawValue := range obj {
		if IsJSONNull(rawValue) {
			continue
		}
		field, ok := lookupJSONKeyFold(fields, key)
		if !ok {
			// 未认领的键归 UnknownTopLevelKeys 管，不在本函数重复上报。
			continue
		}
		if isUnsetOptionalContainer(field) {
			mismatched = append(mismatched, key)
		}
	}
	sort.Strings(mismatched)
	return mismatched, nil
}

// claimedFieldsByJSONKey 建立「json 键名 → 字段值」映射。键名与嵌入提升规则复用
// walkClaimedJSONFields，与 ClaimedJSONKeys 同源——否则同一个 DTO 在「认领了哪些键」
// 与「哪些键被丢弃」上会给出矛盾的答案。
//
// 匿名嵌入指针为 nil 时该字段取不到值（`FieldByIndexErr` 返回错误），直接跳过：那
// 说明整个嵌入结构没被 Unmarshal 触达过，等同「键没传」，报出来只会是假阳性。
func claimedFieldsByJSONKey(structValue reflect.Value) map[string]reflect.Value {
	fields := make(map[string]reflect.Value)
	walkClaimedJSONFields(structValue.Type(), nil, func(name string, index []int) {
		field, err := structValue.FieldByIndexErr(index)
		if err != nil {
			return
		}
		fields[name] = field
	})
	return fields
}

// isUnsetOptionalContainer 判断该字段是否为「三态容器且未取到值」。
// 非容器字段一律返回 false——它们没有三态，无从区分丢弃与正常。
func isUnsetOptionalContainer(field reflect.Value) bool {
	if field.Kind() != reflect.Struct {
		return false
	}
	set := field.FieldByName(optionalSetFieldName)
	if !set.IsValid() || set.Kind() != reflect.Bool {
		return false
	}
	return !set.Bool()
}
