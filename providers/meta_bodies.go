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
)