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

	// closed 记录已发出 response.output_item.done 的 output_index。
	// Responses 协议里每个 output item 都是「开—增—关」三段式，同一个项
	// 只能关一次；关过的项不得被流末兜底逻辑再关一遍（重复的 done 同样是协议违规）。
	closed map[int]bool
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

// markClosed 标记某个 output_index 的项已经发过 response.output_item.done。
func (o *responsesOutputItems) markClosed(index int) {
	if o.closed == nil {
		o.closed = map[int]bool{}
	}
	o.closed[index] = true
}

// isClosed 判断某个 output_index 的项是否已经发过 response.output_item.done。
func (o *responsesOutputItems) isClosed(index int) bool {
	return o.closed[index]
}

// responseOutputItemRef 把 output_index 与其 item 绑在一起：收尾事件要按
// 「开了哪个项就关哪个项」逐项发，遍历口必须带 index（只有 item 拿不回位置）。
type responseOutputItemRef struct {
	index int
	item  map[string]interface{}
}

// pendingItems 按 output_index 升序返回所有「开了还没关」的 item。
//
// 判据是「进了 store 就算开了」：store 只在对应 output_item.added 发出后才写入
// （message / reasoning / function_call 三类皆然），故 store 就是「已开的项」清单。
// 两条协议线（openai 的 tool_calls、anthropic 的 tool_use）产出的项形状一致，
// 差别的只是「项从哪来」，故「给开了的项收尾」这件事可以共用一份实现。
func (o *responsesOutputItems) pendingItems() []responseOutputItemRef {
	indices := o.sortedIndices()
	out := make([]responseOutputItemRef, 0, len(indices))
	for _, idx := range indices {
		item := o.byIndex[idx]
		if item == nil {
			continue
		}
		if o.isClosed(idx) {
			continue
		}
		out = append(out, responseOutputItemRef{index: idx, item: item})
	}
	return out
}

// sortedIndices 返回已占用的 output_index 升序列表。
func (o *responsesOutputItems) sortedIndices() []int {
	indices := make([]int, 0, len(o.byIndex))
	for idx := range o.byIndex {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

// snapshot 按 output_index 升序返回 item 列表，供 response.completed 使用。
func (o *responsesOutputItems) snapshot() []interface{} {
	indices := o.sortedIndices()

	out := make([]interface{}, 0, len(indices))
	for _, idx := range indices {
		if item := o.byIndex[idx]; item != nil {
			out = append(out, item)
		}
	}
	return out
}
