package chat

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/consts"
	preprocessopenai "github.com/qkf688/llmux/service/chat/preprocess/openai"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Before struct {
	Model            string
	Stream           bool
	toolCall         bool
	structuredOutput bool
	image            bool
	raw              []byte
	// affinityKey 对话身份键（缓存亲和）：由 Beforer 从请求语义值派生（见
	// affinityKeyFromRequest），空 = 未识别对话身份（不粘）。
	affinityKey string
}

type Beforer func(data []byte) (*Before, error)

// capabilityDetector 配置各协议 Beforer 的差异点，供 detectCapabilities 使用。
type capabilityDetector struct {
	// messageArrayPath 消息数组路径："messages" 或 "input"（图片扫描与亲和键的首项提取共用）。
	messageArrayPath string
	imageTypeValue   string // image 内容项的 type 值："image_url", "input_image", "image"
	// structuredOutputCheck 检测是否为 structured output；返回 true 表示是。
	structuredOutputCheck func(data []byte) bool
}

// affinityPartCapBytes 键区材料的单段上限（字节）：超长段（多为 base64 图片等超大首条
// 消息）只按头部字节参与——头部跨轮稳定不漂移，代价是超长段失去字段顺序免疫、
// 「超长同头部对话」过合并（纯偏好的无害退化方向），换来每请求的规范化/哈希开销常数级。
const affinityPartCapBytes = 8 << 10

// affinityKeyFromRequest 提取「对话身份」语义值并派生缓存亲和键：
//   - system 段：顶层 system（anthropic）+ 消息数组头部连续的 system/developer 项
//     （openai 系的 system 前导），逐段独立参与；
//   - 首条消息：system 段之后的第一条消息（不论 role——few-shot/预填对话取到的可能是
//     assistant 消息，如实描述；其后多轮只在尾部追加，头部恒稳定）；字符串形 input
//     无 role，取其内容本身参与身份，否则所有单条 input 同键互钉。
//
// 各段经 affinityPart 规范化/截断后交 balancer.AffinityKey。取语义值而非原始字节前缀：
// 字段顺序、注入字段不改变语义值，键不随序列化形态漂移（AC-7）。
func affinityKeyFromRequest(data []byte, cfg capabilityDetector) string {
	msgs := gjson.GetBytes(data, cfg.messageArrayPath).Array()

	var systemSegs [][]byte
	if s := gjson.GetBytes(data, "system"); s.Exists() {
		systemSegs = append(systemSegs, affinityPart(s.Raw))
	}
	first := gjson.Result{}
	for i, m := range msgs {
		if role := m.Get("role").String(); role == "system" || role == "developer" {
			systemSegs = append(systemSegs, affinityPart(m.Raw))
			continue
		}
		first = msgs[i]
		break
	}
	return balancer.AffinityKey(
		gjson.GetBytes(data, "model").String(),
		systemSegs,
		affinityPart(gjson.GetBytes(data, "tools").Raw),
		affinityPart(first.Raw),
	)
}

// affinityPart 键区材料单段：截断到上限后做 JSON 规范化；非法 JSON 原样参与——键退化为
// 按字面区分，**绝不静默丢段**（丢段会让只在该段不同的两个对话并键，与注释承诺相反）。
// 先截断后规范化：超大段的解析/重序列化开销被钉在常数级；截断字面（头部前缀）跨轮
// 稳定，同对话不因此漂移。
func affinityPart(raw string) []byte {
	b := []byte(raw)
	if len(b) > affinityPartCapBytes {
		b = b[:affinityPartCapBytes]
	}
	if c := canonicalJSON(b); c != nil {
		return c
	}
	return b
}

// canonicalJSON JSON 规范化：解析后重新序列化（encoding/json 对 map 键排序），消除对象内
// 字段书写顺序的形态差异——对话身份键认「值」不认「字面顺序」（AC-7）。数字走
// json.Number 保真：不经 float64，>2^53 的整数不丢精度、不与相邻值并键（代价：1 与 1.0
// 不再并键，属无害方向的少合并）。非法/不完整 JSON（如截断产物）返回 nil，由调用方
// 决定按字面参与。
func canonicalJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// detectCapabilities 提取三个 Beforer 共有的 model/stream/tool/image/structuredOutput 检测逻辑。
// 返回检测结果和可能被修改的 data（如 OpenAI 的 stream_options 注入由调用方在调用前完成）。
func detectCapabilities(data []byte, cfg capabilityDetector) (model string, stream bool, toolCall bool, structuredOutput bool, image bool) {
	model = gjson.GetBytes(data, "model").String()
	stream = gjson.GetBytes(data, "stream").Bool()

	tools := gjson.GetBytes(data, "tools")
	if tools.Exists() && len(tools.Array()) != 0 {
		toolCall = true
	}

	if cfg.structuredOutputCheck != nil {
		structuredOutput = cfg.structuredOutputCheck(data)
	}

	gjson.GetBytes(data, cfg.messageArrayPath).ForEach(func(_, value gjson.Result) bool {
		if image {
			return false
		}
		if value.Get("role").String() == "user" {
			value.Get("content").ForEach(func(_, value gjson.Result) bool {
				if value.Get("type").String() == cfg.imageTypeValue {
					image = true
					return false
				}
				return true
			})
		}
		return true
	})

	return
}

func BeforerOpenAI(data []byte) (*Before, error) {
	model := gjson.GetBytes(data, "model").String()
	if model == "" {
		return nil, errors.New("model is empty")
	}
	// 对话身份键取自**网关注入前**的请求语义：stream_options 注入与 FillMissingToolCallIDs
	// 补写的随机 tool_call id 都不得进键——后者会让同一请求每次键不同、粘性永不命中。
	cfg := capabilityDetector{
		messageArrayPath: "messages",
		imageTypeValue:   "image_url",
		structuredOutputCheck: func(data []byte) bool {
			return gjson.GetBytes(data, "response_format").Exists()
		},
	}
	affinityKey := affinityKeyFromRequest(data, cfg)

	stream := gjson.GetBytes(data, "stream").Bool()
	if stream {
		// 为processTee记录usage添加选项 PS:很多客户端只会开启stream 而不会开启include_usage
		newData, err := sjson.SetBytes(data, "stream_options", struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true})
		if err != nil {
			return nil, err
		}
		data = newData
	}

	_, _, toolCall, structuredOutput, image := detectCapabilities(data, cfg)

	if err := preprocessopenai.ValidateToolCallFunctionNames(data); err != nil {
		return nil, err
	}
	if patched, changed, err := preprocessopenai.FillMissingToolCallIDs(data); err == nil && changed {
		data = patched
	}
	return &Before{
		Model:            model,
		Stream:           stream,
		toolCall:         toolCall,
		structuredOutput: structuredOutput,
		image:            image,
		raw:              data,
		affinityKey:      affinityKey,
	}, nil
}

func BeforerOpenAIRes(data []byte) (*Before, error) {
	model := gjson.GetBytes(data, "model").String()
	if model == "" {
		return nil, errors.New("model is empty")
	}

	// Cherry Studio 兼容：如果有 messages 字段，转换为 input 字段
	if gjson.GetBytes(data, "messages").Exists() && !gjson.GetBytes(data, "input").Exists() {
		messages := gjson.GetBytes(data, "messages").Array()
		if len(messages) > 0 {
			// 将 OpenAI Chat 格式的 messages 转换为 Responses API 的 input 格式
			var inputItems []map[string]interface{}
			for _, msg := range messages {
				role := msg.Get("role").String()
				content := msg.Get("content")

				// 构建 ResponsesItem
				item := map[string]interface{}{
					"role": role,
				}

				// 处理 name 字段：只有当 name 存在且非空时才添加
				if name := msg.Get("name").String(); name != "" {
					item["name"] = name
				}

				// 处理 content（可能是 string 或 array）
				if content.IsArray() {
					// content 是数组，保持原样
					item["content"] = content.Value()
				} else {
					// content 是字符串，转换为 input_text 格式的数组
					item["content"] = []map[string]interface{}{
						{
							"type": "input_text",
							"text": content.String(),
						},
					}
				}

				inputItems = append(inputItems, item)
			}

			var err error
			data, err = sjson.SetBytes(data, "input", inputItems)
			if err != nil {
				return nil, err
			}

			// 删除原始的 messages 字段，避免混淆
			data, err = sjson.DeleteBytes(data, "messages")
			if err != nil {
				return nil, err
			}
		}
	}

	if patched, changed, err := preprocessopenai.StripEmptyResponsesInputNames(data); err != nil {
		return nil, err
	} else if changed {
		data = patched
	}

	cfg := capabilityDetector{
		messageArrayPath: "input",
		imageTypeValue:   "input_image",
		structuredOutputCheck: func(data []byte) bool {
			return gjson.GetBytes(data, "text.format.type").String() == "json_schema"
		},
	}
	_, stream, toolCall, structuredOutput, image := detectCapabilities(data, cfg)
	return &Before{
		Model:            model,
		Stream:           stream,
		toolCall:         toolCall,
		structuredOutput: structuredOutput,
		image:            image,
		raw:              data,
		affinityKey:      affinityKeyFromRequest(data, cfg),
	}, nil
}

func BeforerAnthropic(data []byte) (*Before, error) {
	model := gjson.GetBytes(data, "model").String()
	if model == "" {
		return nil, errors.New("model is empty")
	}

	cfg := capabilityDetector{
		messageArrayPath:      "messages",
		imageTypeValue:        "image",
		structuredOutputCheck: nil, // Anthropic 的 structuredOutput 直接等于 toolCall
	}
	_, stream, toolCall, _, image := detectCapabilities(data, cfg)
	return &Before{
		Model:            model,
		Stream:           stream,
		toolCall:         toolCall,
		structuredOutput: toolCall,
		image:            image,
		raw:              data,
		affinityKey:      affinityKeyFromRequest(data, cfg),
	}, nil
}

func init() {
	// 注册表键仍是裸 string（chat 内部 Style 尚未端到端换成 consts.Style，
	// 见 chat_attempt_request.go 的 TODO），故此处显式转换。
	RegisterBeforer(string(consts.StyleOpenAI), BeforerOpenAI)
	RegisterBeforer(string(consts.StyleOpenAIRes), BeforerOpenAIRes)
	RegisterBeforer(string(consts.StyleAnthropic), BeforerAnthropic)
}
