package chat

import (
	"errors"

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
}

type Beforer func(data []byte) (*Before, error)

// capabilityDetector 配置各协议 Beforer 的差异点，供 detectCapabilities 使用。
type capabilityDetector struct {
	imageContentPath string // 查找 image 的数组路径："messages" 或 "input"
	imageTypeValue   string // image 内容项的 type 值："image_url", "input_image", "image"
	// structuredOutputCheck 检测是否为 structured output；返回 true 表示是。
	structuredOutputCheck func(data []byte) bool
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

	gjson.GetBytes(data, cfg.imageContentPath).ForEach(func(_, value gjson.Result) bool {
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

	_, _, toolCall, structuredOutput, image := detectCapabilities(data, capabilityDetector{
		imageContentPath: "messages",
		imageTypeValue:   "image_url",
		structuredOutputCheck: func(data []byte) bool {
			return gjson.GetBytes(data, "response_format").Exists()
		},
	})

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

	_, stream, toolCall, structuredOutput, image := detectCapabilities(data, capabilityDetector{
		imageContentPath: "input",
		imageTypeValue:   "input_image",
		structuredOutputCheck: func(data []byte) bool {
			return gjson.GetBytes(data, "text.format.type").String() == "json_schema"
		},
	})
	return &Before{
		Model:            model,
		Stream:           stream,
		toolCall:         toolCall,
		structuredOutput: structuredOutput,
		image:            image,
		raw:              data,
	}, nil
}

func BeforerAnthropic(data []byte) (*Before, error) {
	model := gjson.GetBytes(data, "model").String()
	if model == "" {
		return nil, errors.New("model is empty")
	}

	_, stream, toolCall, _, image := detectCapabilities(data, capabilityDetector{
		imageContentPath:      "messages",
		imageTypeValue:        "image",
		structuredOutputCheck: nil, // Anthropic 的 structuredOutput 直接等于 toolCall
	})
	return &Before{
		Model:            model,
		Stream:           stream,
		toolCall:         toolCall,
		structuredOutput: toolCall,
		image:            image,
		raw:              data,
	}, nil
}

func init() {
	RegisterBeforer(consts.StyleOpenAI, BeforerOpenAI)
	RegisterBeforer(consts.StyleOpenAIRes, BeforerOpenAIRes)
	RegisterBeforer(consts.StyleAnthropic, BeforerAnthropic)
}
