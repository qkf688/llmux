# Anthropic → OpenAI Responses API 转换修复计划

## 问题描述

当前 Anthropic → openai-res 的流式转换输出格式不正确。

### 当前输出（错误）- 目前的response.txt
```
event: response.created
data: {"response":{"id":"...","object":"response",...},"type":"response.created"}

event: response.output_item.added
data: {"item":{"text":"","type":"output_text"},...}

event: response.output_text.delta
data: {"content_index":0,"delta":"刹那","output_index":0,"type":"response.output_text.delta"}

event: response.completed
data: {"response":{"status":"completed","usage":{...}},"type":"response.completed"}
```

### 期望输出（正确）- 正确的response.txt
```
data: {"type":"response.created","sequence_number":0,"response":{"object":"response","id":"","model":"claude-sonnet-4.5","created_at":0,"output":[],"status":"in_progress"}}

data: {"type":"response.in_progress","sequence_number":1,"response":{...}}

data: {"type":"response.output_item.added","sequence_number":2,"output_index":0,"item":{"id":"item_xxx","type":"message","role":"assistant","content":[],"status":"in_progress"}}

data: {"type":"response.content_part.added","sequence_number":3,"output_index":0,"item_id":"item_xxx","content_index":0,"part":{"type":"output_text","text":""}}

data: {"type":"response.output_text.delta","sequence_number":4,"output_index":0,"item_id":"item_xxx","content_index":0,"delta":"星"}

data: {"type":"response.output_text.done","sequence_number":6,"output_index":0,"item_id":"item_xxx","content_index":0,"text":"星辰"}

data: {"type":"response.content_part.done","sequence_number":7,"output_index":0,"item_id":"item_xxx","content_index":0,"part":{"type":"output_text","text":"星辰"}}

data: {"type":"response.output_item.done","sequence_number":8,"output_index":0,"item":{"id":"item_xxx","type":"message","role":"assistant","content":[{"type":"output_text","text":"星辰"}],"status":"completed"}}

data: {"type":"response.completed","sequence_number":9,"response":{"object":"response","id":"","model":"claude-sonnet-4.5","created_at":0,"output":[],"status":"completed","usage":{...}}}
```

### 关键差异
1. ❌ 使用 `event:` + `data:` 格式 → ✅ 只使用 `data:` 格式
2. ❌ 缺少 `sequence_number` 字段
3. ❌ 缺少 `response.in_progress` 事件
4. ❌ 缺少 `response.content_part.added` 事件
5. ❌ 缺少 `item_id` 字段（delta 事件）
6. ❌ 缺少 `response.output_text.done` 事件
7. ❌ 缺少 `response.content_part.done` 事件
8. ❌ 缺少 `response.output_item.done` 事件
9. ❌ 字段顺序不正确（使用 `json.Marshal` 而不是 `marshalWithTypeFirst`）
10. ❌ response 对象缺少必需字段（model, created_at）

## 根本原因

`service/transform_openai.go:711-860` 的 Anthropic → openai-res 转换代码：
- 使用旧的 `event:` + `data:` 格式
- 没有跟踪 `sequence_number`
- 没有生成完整的事件序列
- 使用 `json.Marshal` 而不是 `marshalWithTypeFirst`

## 修复计划

### Phase 1: 添加状态跟踪变量 ✅
- [x] 使用已有的全局状态变量（sequenceNumber, accumulatedText, responseID, itemID）

### Phase 2: 修复 message_start 事件 ✅
- [x] 移除 `event:` 行
- [x] 添加 `sequence_number` 字段
- [x] 发送 `response.created` 事件
- [x] 发送 `response.in_progress` 事件
- [x] 使用 `marshalWithTypeFirst`
- [x] 添加完整的 response 字段（object, id, model, created_at, output, status）

### Phase 3: 修复 content_block_start 事件 ✅
- [x] 发送 `response.output_item.added` 事件
- [x] 发送 `response.content_part.added` 事件
- [x] 添加 `item_id` 字段
- [x] 使用 `marshalWithTypeFirst`
- [x] 重置 `accumulatedText`

### Phase 4: 修复 content_block_delta 事件 ✅
- [x] 添加 `item_id` 字段
- [x] 累积文本内容到 `accumulatedText`
- [x] 使用 `marshalWithTypeFirst`

### Phase 5: 修复 message_delta 事件 ✅
- [x] 发送 `response.output_text.done` 事件
- [x] 发送 `response.content_part.done` 事件
- [x] 发送 `response.output_item.done` 事件
- [x] 发送 `response.completed` 事件
- [x] 使用 `marshalWithTypeFirst`
- [x] 添加完整的 usage 信息（input_tokens_details, output_tokens_details）

### Phase 6: 测试验证 ✅
- [x] 编译测试通过
- [x] 所有单元测试通过
- [x] 字段顺序正确（使用 marshalWithTypeFirst）

## 修复完成

所有修改已完成并通过测试。Anthropic → openai-res 转换现在输出标准的 OpenAI Responses API 格式。

## 关键文件
- `service/transform_openai.go:711-904` - Anthropic → openai-res 转换代码（已修复）
- `正确的response.txt` - 参考标准
- `目前的response.txt` - 当前错误输出（修复后应该正确）

## 代码位置
- 开始行: 711
- 结束行: 904
- 函数: `transformStreamResponseRealtime`


