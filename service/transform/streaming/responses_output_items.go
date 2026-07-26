package streaming

import "sort"

// responsesOutputItems 累积一条 Responses 流已产出的 output item，按 output_index 归位。
//
// 存在的理由：`response.completed` 的 `response.output` 必须反映真实产出——
// 下游 responses_to_openai 靠扫这个数组里有没有 function_call 决定 finish_reason，
// responses_to_anthropic 同样靠它决定 stop_reason。数组恒空会让工具调用被当成普通结束。
//
// 同时兼任 output_index 分配器：同一条流里 message / reasoning / function_call
// 各占一个互不重叠的 output_index。
type responsesOutputItems struct {
	byIndex map[int]map[string]interface{}
	next    int
}

// alloc 返回一个尚未被占用的 output_index。
func (o *responsesOutputItems) alloc() int {
	idx := o.next
	o.next++
	return idx
}

// set 记录（或覆盖）某个 output_index 上的 item 终态。
func (o *responsesOutputItems) set(index int, item map[string]interface{}) {
	if o.byIndex == nil {
		o.byIndex = map[int]map[string]interface{}{}
	}
	o.byIndex[index] = item
	if index >= o.next {
		o.next = index + 1
	}
}

// get 取回某个 output_index 上的 item，未记录时返回 nil。
func (o *responsesOutputItems) get(index int) map[string]interface{} {
	return o.byIndex[index]
}

// appendArguments 往 function_call item 的 arguments 追加增量。
func (o *responsesOutputItems) appendArguments(index int, delta string) {
	item := o.get(index)
	if item == nil {
		return
	}
	prev, _ := item["arguments"].(string)
	item["arguments"] = prev + delta
}

// snapshot 按 output_index 升序返回 item 列表，供 response.completed 使用。
func (o *responsesOutputItems) snapshot() []interface{} {
	indices := make([]int, 0, len(o.byIndex))
	for idx := range o.byIndex {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	out := make([]interface{}, 0, len(indices))
	for _, idx := range indices {
		if item := o.byIndex[idx]; item != nil {
			out = append(out, item)
		}
	}
	return out
}
