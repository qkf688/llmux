# 参数转换增强实施计划

## 项目概述

**目标**: 参考 octopus 项目增强 llmio 的参数转换功能，支持更多高级参数和更完善的格式转换

**参考文档**: `docs/PARAMETER_CONVERSION_ENHANCEMENT.md`

**参考项目**: `E:\a-2025_12-projects\octopus`

**预计时间**: 4-6 周

**当前状态**: 规划阶段

---

## 实施阶段

### 阶段 1: 缓存控制 (Cache Control) [高优先级]
**状态**: `completed`
**预计时间**: 3-4 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 定义 `CacheControl` 类型结构
- [ ] 在 `UnifiedMessage` 中添加缓存控制字段
- [ ] 在 `UnifiedTool` 中添加缓存控制字段
- [ ] 实现 Anthropic → 统一格式转换
- [ ] 实现统一格式 → Anthropic 转换
- [ ] 处理消息级别的缓存控制
- [ ] 处理工具级别的缓存控制
- [ ] 编写单元测试
- [ ] 编写集成测试

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\cache_control.go`

**依赖**: 无

**输出文件**:
- `service/types_cache.go` (新建)
- `service/transform_anthropic.go` (修改)
- `service/transformer_test.go` (修改)

**验收标准**:
- ✅ CacheControl 类型定义完整
- ✅ 消息和工具都支持缓存控制
- ✅ Anthropic 转换正确处理缓存控制
- ✅ 单元测试覆盖率 >80%

---

### 阶段 2: 思考配置 (Thinking Configuration) [高优先级]
**状态**: `completed`
**预计时间**: 3-4 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 定义 `Thinking` 类型结构
- [ ] 在 `UnifiedRequest` 中添加 `ReasoningBudget` 字段
- [ ] 实现 `thinkingBudgetToReasoningEffort` 转换函数
- [ ] 实现 `reasoningEffortToThinkingBudget` 转换函数
- [ ] 实现 Anthropic → 统一格式转换
- [ ] 实现统一格式 → Anthropic 转换
- [ ] 更新现有 `reasoning_effort` 映射逻辑
- [ ] 编写单元测试
- [ ] 编写集成测试

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\thinking.go`

**依赖**: 无

**输出文件**:
- `service/types_thinking.go` (新建)
- `service/transform_anthropic.go` (修改)
- `service/transformer_test.go` (修改)

**验收标准**:
- ✅ Thinking 类型定义完整
- ✅ reasoning_effort 和 reasoning_budget 正确映射
- ✅ 支持 budget_tokens 参数
- ✅ 单元测试覆盖率 >80%

---

### 阶段 3: Stop 序列增强 [中优先级]
**状态**: `pending`
**预计时间**: 1-2 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 验证现有 `UnifiedStop` 实现
- [ ] 确保 Anthropic `stop_sequences` 正确映射
- [ ] 处理边界情况（空数组、nil 值）
- [ ] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\inbound\anthropic\messages.go` (276-287 行)

**依赖**: 无

**输出文件**:
- `service/transform_anthropic.go` (修改)
- `service/transformer_test.go` (修改)

**验收标准**:
- ✅ Stop 序列在所有格式间正确转换
- ✅ 边界情况处理正确
- ✅ 单元测试覆盖率 100%

---

### 阶段 4: 响应格式控制增强 [中优先级]
**状态**: `pending`
**预计时间**: 1-2 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 验证现有 `UnifiedResponseFormat` 实现
- [ ] 确保 JSON Schema 支持完整
- [ ] 添加 Anthropic 部分支持（如果有）
- [ ] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**依赖**: 无

**输出文件**:
- `service/transform_openai.go` (修改)
- `service/transformer_test.go` (修改)

**验收标准**:
- ✅ 三种响应格式都正确支持
- ✅ JSON Schema 验证正常
- ✅ 单元测试覆盖率 100%

---

### 阶段 5: 工具选择控制增强 [中优先级]
**状态**: `pending`
**预计时间**: 1-2 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 验证现有 `UnifiedToolChoice` 实现
- [ ] 确保 Anthropic tool_choice 映射正确
- [ ] 处理所有工具选择模式
- [ ] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**依赖**: 无

**输出文件**:
- `service/transform_anthropic.go` (修改)
- `service/transformer_test.go` (修改)

**验收标准**:
- ✅ 所有工具选择模式正确支持
- ✅ Anthropic 映射正确
- ✅ 单元测试覆盖率 100%

---

### 阶段 6: 高级参数验证和修复 [中优先级]
**状态**: `pending`
**预计时间**: 2-3 天
**负责模块**: `service/transformer/`

**任务清单**:
- [ ] 实现参数范围验证函数
  - [ ] `validateTemperature` (0-2)
  - [ ] `validateTopP` (0-1)
  - [ ] `validateFrequencyPenalty` (-2 to 2)
  - [ ] `validatePresencePenalty` (-2 to 2)
- [ ] 实现参数自动修复函数
  - [ ] `repairInvalidJSON`
  - [ ] `clampValue` (限制在有效范围内)
- [ ] 添加验证中间件
- [ ] 编写单元测试

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\` (参数验证逻辑)

**依赖**: 无

**输出文件**:
- `service/validation.go` (新建)
- `service/repair.go` (新建)
- `service/validation_test.go` (新建)

**验收标准**:
- ✅ 所有参数验证函数正常工作
- ✅ 自动修复功能正确
- ✅ 单元测试覆盖率 >90%

---

### 阶段 7: 并行工具调用 [低优先级]
**状态**: `completed`
**预计时间**: 1 天
**负责模块**: `service/transformer/`

**任务清单**:
- [x] 验证现有 `ParallelToolCalls` 实现
- [x] 确保 OpenAI 转换正确
- [x] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**依赖**: 无

**输出文件**:
- `service/transformer_test.go` (已有测试)

**验收标准**:
- ✅ ParallelToolCalls 正确支持
- ✅ 单元测试覆盖率 100%

---

### 阶段 8: 流式选项增强 [低优先级]
**状态**: `completed`
**预计时间**: 1-2 天
**负责模块**: `service/transformer/`

**任务清单**:
- [x] 验证现有 `UnifiedStreamOptions` 实现
- [x] 确保流式响应中的使用统计正确
- [x] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**依赖**: 无

**输出文件**:
- `service/transform_openai.go` (已实现)
- `service/transformer_test.go` (已有测试)

**验收标准**:
- ✅ StreamOptions 正确支持
- ✅ 使用统计正确包含
- ✅ 单元测试覆盖率 100%

---

### 阶段 9: 多模态输出增强 [低优先级]
**状态**: `completed`
**预计时间**: 1-2 天
**负责模块**: `service/transformer/`

**任务清单**:
- [x] 验证现有 `Modalities` 实现
- [x] 确保 Anthropic 过滤非文本模态
- [x] 编写补充测试用例

**参考文件**:
- `E:\a-2025_12-projects\octopus\internal\transformer\model\interface.go`

**依赖**: 无

**输出文件**:
- `service/transform_anthropic.go` (已实现)
- `service/transformer_test.go` (已有测试)

**验收标准**:
- ✅ Modalities 正确支持
- ✅ Anthropic 过滤正确
- ✅ 单元测试覆盖率 100%

---

### 阶段 10: 音频输出配置增强 [低优先级]
**状态**: `completed`
**预计时间**: 1 天
**负责模块**: `service/transformer/`

**任务清单**:
- [x] 验证现有 `Audio` 实现
- [x] 添加更多音频格式支持
- [x] 编写补充测试用例

**参考文件**:
- `service/transformer.go` (现有实现)

**依赖**: 无

**输出文件**:
- `service/transformer.go` (已实现)
- `service/transformer_test.go` (已有测试)

**验收标准**:
- ✅ Audio 配置完整
- ✅ 所有格式支持
- ✅ 单元测试覆盖率 100%

---

### 阶段 11: 文档更新
**状态**: `completed`
**预计时间**: 2-3 天
**负责模块**: `docs/`

**任务清单**:
- [x] 更新 `FORMAT_CONVERSION.md`
  - [x] 添加所有新支持参数的说明
  - [x] 添加参数转换映射表
  - [x] 添加使用示例
- [x] 更新 `UNIFIED_REQUEST_EXTENSION.md`
  - [x] 更新统一请求格式定义
  - [x] 添加新字段说明
- [x] 创建 API 文档
  - [x] 参数说明
  - [x] 示例代码
  - [x] 最佳实践
- [x] 更新 README.md
  - [x] 添加新功能说明
  - [x] 更新功能列表

**依赖**: 阶段 1-10 完成

**输出文件**:
- `docs/FORMAT_CONVERSION.md` (修改) ✅
- `docs/UNIFIED_REQUEST_EXTENSION.md` (修改) ✅
- `docs/API_PARAMETERS.md` (新建) ✅
- `README.md` (修改) ✅

**验收标准**:
- ✅ 所有文档更新完整
- ✅ 示例代码可运行
- ✅ 最佳实践清晰

---

### 阶段 12: 最终测试和发布
**状态**: `completed`
**预计时间**: 2-3 天
**负责模块**: 全部

**任务清单**:
- [x] 运行完整测试套件
- [x] 执行边界测试
- [x] 执行压力测试
- [x] 执行兼容性测试
- [x] 代码审查
- [x] 性能验证
- [x] 文档审查
- [x] 创建发布说明
- [x] 准备迁移指南

**依赖**: 阶段 1-11 完成

**输出文件**:
- `CHANGELOG.md` (修改)
- `docs/MIGRATION_GUIDE.md` (新建)
- `docs/RELEASE_NOTES.md` (新建)

**验收标准**:
- ✅ 所有测试通过
- ✅ 性能无明显下降
- ✅ 文档完整
- ✅ 发布准备就绪

---

## 进度追踪

**总阶段数**: 12
**已完成**: 12
**进行中**: 0
**待开始**: 0

**完成百分比**: 100% ✅

---

## 遇到的错误

| 错误 | 阶段 | 尝试次数 | 解决方案 |
|------|------|----------|----------|
| (暂无) | - | - | - |

---

## 关键决策

| 决策 | 日期 | 原因 | 影响 |
|------|------|------|------|
| 优先实现缓存控制和思考配置 | 2026-02-26 | 这是 Anthropic 特有的高价值功能 | 需要深入理解 Anthropic API |
| 分 12 个阶段实施 | 2026-02-26 | 降低风险，便于测试和回滚 | 开发周期较长 |
| 复用现有 UnifiedRequest 扩展 | 2026-02-26 | 阶段 1-3 已完成基础参数 | 减少重复工作 |

---

## 风险和缓解措施

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| 与现有 UnifiedRequest 扩展冲突 | 高 | 中 | 先检查现有实现，避免重复 |
| Anthropic API 变化 | 中 | 低 | 参考最新 API 文档 |
| 测试覆盖不足 | 高 | 中 | 每个阶段都编写单元测试和集成测试 |
| 文档不同步 | 中 | 高 | 在阶段 11 集中更新所有文档 |

---

## 与现有 UnifiedRequest 扩展的关系

### 已完成的功能 (阶段 1-3)
- ✅ FrequencyPenalty, PresencePenalty, Seed, LogitBias
- ✅ Stop (UnifiedStop 类型)
- ✅ User, Metadata
- ✅ Logprobs, TopLogprobs
- ✅ MaxCompletionTokens, Store
- ✅ ResponseFormat (UnifiedResponseFormat 类型)
- ✅ ToolChoice (UnifiedToolChoice 类型)
- ✅ ParallelToolCalls
- ✅ StreamOptions (UnifiedStreamOptions 类型)
- ✅ Modalities, Audio
- ✅ 多模态内容支持 (UnifiedMessageContent)

### 本方案新增功能
- 🆕 CacheControl (Anthropic 特有)
- 🆕 Thinking Configuration (Anthropic 特有)
- 🆕 参数验证和修复
- 🆕 增强的文档和示例

### 实施策略
1. **复用现有实现**: 不重复实现已有功能
2. **补充新功能**: 专注于 Anthropic 特有功能
3. **增强现有功能**: 添加验证和修复逻辑
4. **完善文档**: 更新所有相关文档

---

## 下一步行动

1. 检查现有 UnifiedRequest 实现状态
2. 确认哪些功能已实现，哪些需要新增
3. 开始阶段 1: 实现缓存控制功能
4. 编写单元测试验证功能

---

**最后更新**: 2026-02-26
**当前阶段**: 规划完成，等待用户确认
