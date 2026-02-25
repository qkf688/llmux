# UnifiedRequest 扩展实施清单

## 阶段 1: 基础字段扩展

### 1.1 类型定义 (service/transformer.go)

#### 新增辅助类型
- [ ] `UnifiedStop` - 停止序列 (支持 string 或 []string)
  ```go
  type UnifiedStop struct {
      Single   *string
      Multiple []string
  }
  ```

#### 扩展 UnifiedRequest
- [ ] `FrequencyPenalty *float64` - 频率惩罚 (-2.0 到 2.0)
- [ ] `PresencePenalty *float64` - 存在惩罚 (-2.0 到 2.0)
- [ ] `Seed *int64` - 随机种子
- [ ] `LogitBias map[string]int64` - Token 偏置
- [ ] `Stop *UnifiedStop` - 停止序列
- [ ] `User *string` - 用户标识符
- [ ] `Metadata map[string]string` - 元数据

---

### 1.2 辅助函数 (service/transform_openai.go)

#### 新增辅助函数
- [ ] `getInt(m map[string]interface{}, key string) int`
- [ ] `getInt64(m map[string]interface{}, key string) int64`
- [ ] `getFloat64Ptr(m map[string]interface{}, key string) *float64`
- [ ] `getStringPtr(m map[string]interface{}, key string) *string`
- [ ] `getInt64Ptr(m map[string]interface{}, key string) *int64`
- [ ] `getBoolPtr(m map[string]interface{}, key string) *bool`
- [ ] `getStringArray(m map[string]interface{}, key string) []string`
- [ ] `getIntMap(m map[string]interface{}, key string) map[string]int64`

---

### 1.3 OpenAI 转换逻辑

#### TransformOpenAIToUnified (service/transform_openai.go:19+)
- [ ] 解析 `frequency_penalty` → `FrequencyPenalty`
- [ ] 解析 `presence_penalty` → `PresencePenalty`
- [ ] 解析 `seed` → `Seed`
- [ ] 解析 `logit_bias` → `LogitBias`
- [ ] 解析 `stop` → `Stop` (处理 string 和 []string)
- [ ] 解析 `user` → `User`
- [ ] 解析 `metadata` → `Metadata`

#### TransformUnifiedToOpenAI (service/transform_openai.go:100+)
- [ ] 输出 `FrequencyPenalty` → `frequency_penalty`
- [ ] 输出 `PresencePenalty` → `presence_penalty`
- [ ] 输出 `Seed` → `seed`
- [ ] 输出 `LogitBias` → `logit_bias`
- [ ] 输出 `Stop` → `stop` (处理 Single 和 Multiple)
- [ ] 输出 `User` → `user`
- [ ] 输出 `Metadata` → `metadata`

---

### 1.4 Anthropic 转换逻辑

#### TransformAnthropicToUnified (service/transform_anthropic.go:12+)
- [ ] 解析 `stop_sequences` → `Stop.Multiple`
- [ ] 解析 `metadata` → `Metadata`
- [ ] 忽略不支持的字段 (frequency_penalty, presence_penalty, seed, logit_bias, user)

#### TransformUnifiedToAnthropic (service/transform_anthropic.go:102+)
- [ ] 输出 `Stop` → `stop_sequences` (只支持数组)
- [ ] 输出 `Metadata` → `metadata`
- [ ] 忽略不支持的字段

---

### 1.5 Responses API 转换逻辑

#### TransformResponsesToUnified (service/transform_responses.go:13+)
- [ ] 解析 `frequency_penalty` → `FrequencyPenalty`
- [ ] 解析 `presence_penalty` → `PresencePenalty`
- [ ] 解析 `seed` → `Seed`
- [ ] 解析 `logit_bias` → `LogitBias`
- [ ] 解析 `stop` → `Stop`
- [ ] 解析 `user` → `User`
- [ ] 解析 `metadata` → `Metadata`

#### TransformUnifiedToResponses (service/transform_responses.go)
- [ ] 输出所有新字段到 Responses 格式

---

### 1.6 测试用例

#### 单元测试 (service/transformer_test.go)
- [ ] `TestTransformOpenAIToUnified_FrequencyPenalty`
- [ ] `TestTransformOpenAIToUnified_PresencePenalty`
- [ ] `TestTransformOpenAIToUnified_Seed`
- [ ] `TestTransformOpenAIToUnified_LogitBias`
- [ ] `TestTransformOpenAIToUnified_Stop_String`
- [ ] `TestTransformOpenAIToUnified_Stop_Array`
- [ ] `TestTransformOpenAIToUnified_User`
- [ ] `TestTransformOpenAIToUnified_Metadata`
- [ ] `TestTransformUnifiedToOpenAI_AllNewFields`
- [ ] `TestTransformUnifiedToAnthropic_CompatibleFields`

#### 边界测试
- [ ] `TestNewFields_NilValues`
- [ ] `TestNewFields_ZeroValues`
- [ ] `TestNewFields_EmptyValues`
- [ ] `TestNewFields_InvalidTypes`

#### 集成测试
- [ ] `TestEndToEnd_OpenAI_WithNewFields`
- [ ] `TestEndToEnd_Anthropic_WithNewFields`
- [ ] `TestEndToEnd_Responses_WithNewFields`

---

### 1.7 文档更新

- [ ] 更新 `CLAUDE.md` - 添加新字段说明
- [ ] 更新 API 文档 - 说明新参数用法
- [ ] 更新兼容性矩阵 - 标注提供商支持情况
- [ ] 添加迁移指南 - 如何使用新字段

---

## 阶段 2: 响应格式和工具增强

### 2.1 类型定义

#### 新增类型
- [ ] `UnifiedResponseFormat` - 响应格式控制
  ```go
  type UnifiedResponseFormat struct {
      Type       string          `json:"type"` // "text" | "json_object" | "json_schema"
      JSONSchema json.RawMessage `json:"json_schema,omitempty"`
  }
  ```

- [ ] `UnifiedToolChoice` - 工具选择控制
  ```go
  type UnifiedToolChoice struct {
      StringValue *string                  // "auto" | "none" | "required"
      ObjectValue *UnifiedToolChoiceObject // {type: "function", function: {name: "..."}}
  }

  type UnifiedToolChoiceObject struct {
      Type     string                      `json:"type"`
      Function *UnifiedToolChoiceFunction  `json:"function,omitempty"`
  }

  type UnifiedToolChoiceFunction struct {
      Name string `json:"name"`
  }
  ```

- [ ] `UnifiedStreamOptions` - 流式选项
  ```go
  type UnifiedStreamOptions struct {
      IncludeUsage bool `json:"include_usage,omitempty"`
  }
  ```

#### 扩展 UnifiedRequest
- [ ] `ResponseFormat *UnifiedResponseFormat` - 响应格式
- [ ] `ToolChoice *UnifiedToolChoice` - 工具选择
- [ ] `ParallelToolCalls *bool` - 并行工具调用
- [ ] `StreamOptions *UnifiedStreamOptions` - 流式选项

---

### 2.2 转换逻辑
- [ ] OpenAI: 解析和输出 ResponseFormat
- [ ] OpenAI: 解析和输出 ToolChoice (string 和 object)
- [ ] OpenAI: 解析和输出 ParallelToolCalls
- [ ] OpenAI: 解析和输出 StreamOptions
- [ ] Anthropic: 映射兼容的工具选择
- [ ] Responses: 完整支持所有字段

---

### 2.3 测试用例
- [ ] ResponseFormat - text 模式
- [ ] ResponseFormat - json_object 模式
- [ ] ResponseFormat - json_schema 模式
- [ ] ToolChoice - string 值 ("auto", "none", "required")
- [ ] ToolChoice - object 值 (指定工具名)
- [ ] ParallelToolCalls - true/false
- [ ] StreamOptions - include_usage

---

## 阶段 3: 多模态内容支持

### 3.1 类型重构

#### 重构 UnifiedMessage.Content
- [ ] 创建 `UnifiedMessageContent` 类型
  ```go
  type UnifiedMessageContent struct {
      Text            *string                     `json:"text,omitempty"`
      ContentParts    []UnifiedMessageContentPart `json:"content_parts,omitempty"`
  }
  ```

- [ ] 创建 `UnifiedMessageContentPart` 类型
  ```go
  type UnifiedMessageContentPart struct {
      Type         string               `json:"type"` // "text" | "image_url" | "input_audio"
      Text         *string              `json:"text,omitempty"`
      ImageURL     *UnifiedImageURL     `json:"image_url,omitempty"`
      InputAudio   *UnifiedInputAudio   `json:"input_audio,omitempty"`
      CacheControl *UnifiedCacheControl `json:"-"` // Anthropic 特有
  }
  ```

- [ ] 创建辅助类型
  ```go
  type UnifiedImageURL struct {
      URL    string  `json:"url"`
      Detail *string `json:"detail,omitempty"` // "auto" | "low" | "high"
  }

  type UnifiedInputAudio struct {
      Data   string `json:"data"`   // base64
      Format string `json:"format"` // "wav" | "mp3"
  }

  type UnifiedCacheControl struct {
      Type string `json:"type"` // "ephemeral"
  }
  ```

#### 扩展 UnifiedRequest
- [ ] `Modalities []string` - 支持的模态 (["text", "audio"])
- [ ] `Audio *UnifiedAudio` - 音频配置
  ```go
  type UnifiedAudio struct {
      Voice  string `json:"voice"`  // "alloy" | "echo" | ...
      Format string `json:"format"` // "wav" | "mp3" | "pcm16"
  }
  ```

---

### 3.2 向后兼容迁移
- [ ] 保持 `Content interface{}` 字段
- [ ] 添加转换逻辑: `interface{}` ↔ `UnifiedMessageContent`
- [ ] 渐进式迁移: 先支持新格式，保持旧格式兼容

---

### 3.3 转换逻辑
- [ ] OpenAI: 解析多模态 content (text, image_url, input_audio)
- [ ] OpenAI: 输出多模态 content
- [ ] Anthropic: 解析多模态 content (text, image)
- [ ] Anthropic: 输出多模态 content
- [ ] Responses: 完整支持

---

### 3.4 测试用例
- [ ] 纯文本消息 (向后兼容)
- [ ] 图像 URL 消息
- [ ] 图像 base64 消息
- [ ] 音频消息
- [ ] 混合内容消息 (文本 + 图像)
- [ ] Modalities 字段
- [ ] Audio 配置

---

## 阶段 4-7 清单

### 阶段 4: Embedding API 集成
- [ ] 添加 Embedding 相关字段
- [ ] 实现 Embedding 请求转换
- [ ] 实现 Embedding 响应转换
- [ ] 添加路由处理逻辑
- [ ] 编写 Embedding 测试

### 阶段 5: 缓存和推理优化
- [ ] 添加缓存相关字段
- [ ] 添加推理相关字段
- [ ] 实现 Anthropic 缓存控制
- [ ] 实现推理参数映射
- [ ] 编写测试

### 阶段 6: 扩展性和元数据
- [ ] 添加 ExtraBody 字段
- [ ] 添加内部元数据字段
- [ ] 实现 ExtraBody 合并逻辑
- [ ] 实现元数据传递机制
- [ ] 更新文档

### 阶段 7: 测试和文档完善
- [ ] 完整单元测试套件
- [ ] 集成测试
- [ ] 性能测试
- [ ] API 文档更新
- [ ] 迁移指南
- [ ] CLAUDE.md 更新

---

## 质量检查清单

### 代码质量
- [ ] 所有新代码通过 `golangci-lint`
- [ ] 所有新代码通过 `go vet`
- [ ] 所有新代码通过 `go fmt`
- [ ] 测试覆盖率 >85%

### 功能验证
- [ ] 所有单元测试通过
- [ ] 所有集成测试通过
- [ ] 手动测试主要场景
- [ ] 向后兼容性验证

### 性能验证
- [ ] 转换延迟 <5% 增加
- [ ] 内存使用 <10% 增加
- [ ] 并发性能无退化

### 文档完整性
- [ ] API 文档更新
- [ ] 代码注释完整
- [ ] 示例代码可运行
- [ ] 迁移指南清晰

---

## 风险缓解清单

### 向后兼容性
- [ ] 现有测试全部通过
- [ ] 现有 API 行为不变
- [ ] 新字段都是可选的
- [ ] 默认值合理

### 提供商兼容性
- [ ] 不支持的字段优雅忽略
- [ ] 错误信息清晰
- [ ] 日志记录完整

### 性能影响
- [ ] 性能基准测试
- [ ] 热路径优化
- [ ] 内存分配优化

---

**创建日期**: 2026-02-25
**最后更新**: 2026-02-25
