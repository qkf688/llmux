package providers

// 探针 / 模板字面量集中存放，供各 provider init 注册。
// 注意：TestBody 与 HealthCheckBody 可能存在空白差异，禁止合并为同一常量。

const (
	configTemplateOpenAI = `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`

	configTemplateOpenAIRes = `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`

	configTemplateAnthropic = `{
			"base_url": "https://api.anthropic.com/v1",
			"api_key": "YOUR_API_KEY",
			"beta": "",
			"version": "2023-06-01",
			"auth_type": "x-api-key"
		}`

	// --- OCP-7 testapi 连通性测试体 ---
	testBodyOpenAI = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	testBodyOpenAIRes = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	testBodyAnthropic = `{
     	"model": "claude-sonnet-4-5",
     	"max_tokens": 1000,
     	"messages": [
       		{
         		"role": "user",
         		"content": "Write a one-sentence bedtime story about a unicorn."
       		}
     	]
	}`

	// --- OCP-8 healthcheck 探针体（与 testapi 可能空白不同）---
	healthCheckBodyOpenAI = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	healthCheckBodyOpenAIRes = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	healthCheckBodyAnthropic = `{
    	"model": "claude-sonnet-4-5",
    	"max_tokens": 1000,
    	"messages": [
      		{
        		"role": "user", 
        		"content": "Write a one-sentence bedtime story about a unicorn."
      		}
    	]
 	}`

	// --- #24-B 按凭据测试最小 completion 探针体（不含 model，由 BuildReq sjson 注入指定 test_model）---
	// 刻意压到极小（openai max_tokens:1、anthropic/responses 各带极小上限）：管理员点一次测一条 key，
	// 花费须可忽略，且任何 2xx 即判 key 活性——与 TestBody/HealthCheckBody 的「故事补全」预算不同。
	probeBodyOpenAI = `{
        "max_tokens": 1,
        "messages": [
            {
                "role": "user",
                "content": "hi"
            }
        ]
    }`

	probeBodyOpenAIRes = `{
        "input": "hi",
        "max_output_tokens": 16
    }`

	probeBodyAnthropic = `{
        "max_tokens": 16,
        "messages": [
            {
                "role": "user",
                "content": "hi"
            }
        ]
    }`

	// --- OCP-7 结构化输出测试体 ---
	structuredOutputSchema = `{
		"type": "object",
		"properties": {
			"language": { "type": "string" },
			"version": { "type": "integer" },
			"features": { "type": "array", "items": { "type": "string" } },
			"score": { "type": "number" },
			"passed": { "type": "boolean" }
		},
		"required": ["language", "version", "features", "score", "passed"],
		"additionalProperties": false
	}`

	structuredBodyOpenAI = `{
		"model": "gpt-4.1",
		"temperature": 0,
		"messages": [
			{
				"role": "user",
				"content": "请严格按 JSON Schema 输出一个 JSON 对象，不要输出任何额外文本。"
			}
		],
		"response_format": {
			"type": "json_schema",
			"json_schema": {
				"name": "structured_output_test",
				"strict": true,
				"schema": ` + structuredOutputSchema + `
			}
		}
	}`

	structuredBodyOpenAIRes = `{
		"model": "gpt-5-nano",
		"input": "请严格按 JSON Schema 输出一个 JSON 对象，不要输出任何额外文本。",
		"text": {
			"format": {
				"type": "json_schema",
				"json_schema": {
					"name": "structured_output_test",
					"strict": true,
					"schema": ` + structuredOutputSchema + `
				}
			}
		}
	}`

	structuredBodyAnthropic = `{
		"model": "claude-sonnet-4-5",
		"max_tokens": 1000,
		"messages": [
			{
				"role": "user",
				"content": "请调用 structured_output 工具，并在工具入参中填充 JSON Schema 所需字段。不要输出额外文本。"
			}
		],
		"tools": [
			{
				"name": "structured_output",
				"description": "Emit structured output that conforms to the schema.",
				"input_schema": ` + structuredOutputSchema + `
			}
		],
		"tool_choice": { "type": "tool", "name": "structured_output" }
	}`

	// --- OCP-7 thinking 能力测试体 ---
	// effort 固定 medium：兼容面最广（个别上游对高级档不识别，见测试弹窗方案「已知风险」）。
	// 问题刻意选一个能触发推理过程的小算术题，非空推理内容即判通过。
	thinkingBodyOpenAI = `{
        "model": "gpt-4.1",
        "reasoning_effort": "medium",
        "messages": [
            {
                "role": "user",
                "content": "2 的 10 次方减 8 等于多少？请一步步推理。"
            }
        ]
    }`

	thinkingBodyOpenAIRes = `{
        "model": "gpt-5-nano",
        "input": "2 的 10 次方减 8 等于多少？请一步步推理。",
        "reasoning": {
            "effort": "medium"
        }
    }`

	// Anthropic 硬性约束：max_tokens ≥ budget_tokens + 1024，取 budget 2048 + max 4096。
	// extended thinking 已 GA，无需 beta 头（provider 未配 beta 时 setAnthropicBeta 会删头，不影响本测试）。
	thinkingBodyAnthropic = `{
     	"model": "claude-sonnet-4-5",
     	"max_tokens": 4096,
     	"thinking": {
     		"type": "enabled",
     		"budget_tokens": 2048
     	},
     	"messages": [
       		{
         		"role": "user",
         		"content": "2 的 10 次方减 8 等于多少？请一步步推理。"
       		}
     	]
	}`
)
