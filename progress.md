# UnifiedRequest 扩展实现 - 进度日志

## 会话信息
- **开始时间**: 2026-02-25
- **当前阶段**: 规划阶段
- **状态**: 等待用户确认

---

## 2026-02-25 规划阶段

### 已完成
- ✅ 读取并分析 `docs/UNIFIED_REQUEST_EXTENSION.md` 文档
- ✅ 分析现有代码库结构
  - 读取 `service/transformer.go` - 核心类型定义
  - 搜索 `UnifiedRequest` 相关代码
  - 分析转换逻辑分布
- ✅ 创建 `task_plan.md` - 7 阶段实施计划
- ✅ 创建 `findings.md` - 研究发现和技术洞察
- ✅ 创建 `progress.md` - 本文件
- ✅ 深度分析转换逻辑
  - 读取 `service/transform_anthropic.go` - Anthropic 转换实现
  - 读取 `service/transform_responses.go` - Responses API 转换实现
  - 分析辅助函数 (getString, getBool 等)
  - 分析测试框架和测试模式
- ✅ 补充实施细节到 `findings.md`
  - 辅助函数分析和补充建议
  - 阶段 1 详细实施步骤
  - 兼容性矩阵
  - 性能考虑和优化策略
- ✅ **参考 Octopus 项目实现**
  - 读取 `E:\a-2025_12-projects\octopus\internal\transformer\model\model.go`
  - 分析 `InternalLLMRequest` 完整结构 (971 行)
  - 学习自定义 JSON 序列化模式 (Stop, MessageContent, ToolChoice, EmbeddingInput)
  - 更新 `findings.md` 添加 Octopus 参考分析
  - 更新 `task_plan.md` 阶段 1 任务
- ✅ 创建功能分支 `feature/unified-request-phase1`
- ✅ **阶段 1: 基础字段扩展 (完成)**
  - 添加 `UnifiedStop` 类型（实现自定义 JSON 序列化）
  - 扩展 `UnifiedRequest` 添加 11 个新字段
  - 添加 9 个辅助函数支持新字段类型
  - 更新 OpenAI 转换逻辑（解析和输出）
  - 更新 Anthropic 转换逻辑（映射兼容字段）
  - 编写完整的单元测试（12 个测试用例）
  - 所有测试通过 ✅

### 关键发现
1. **现有结构**: `UnifiedRequest` 有 9 个字段，已支持基础参数和 `ReasoningEffort`
2. **转换架构**: 使用 `TransformerManager` 管理三种格式转换 (openai, openai-res, anthropic)
3. **扩展需求**: 需要添加约 30+ 个新字段，分 7 个阶段实施
4. **风险点**: Content 字段重构、流式响应逻辑、提供商兼容性
5. **辅助函数**: 已有 6 个辅助函数，需补充 8 个新函数支持新字段类型
6. **测试框架**: 现有 4 个测试文件，建议改用表驱动测试提高覆盖率
7. **兼容性**: OpenAI 支持最全，Anthropic 部分支持，需要优雅降级策略
8. **Octopus 参考**:
   - 使用自定义 JSON 序列化处理复杂类型 (Stop, MessageContent, ToolChoice, EmbeddingInput)
   - 使用 `omitzero` 标签而非 `omitempty` 处理复杂类型
   - 提供 Validate(), IsXXXRequest() 等辅助方法
   - 帮助字段使用 `json:"-"` 标记不序列化

### 实施计划概览
```
阶段 1: 基础字段扩展 (2-3天)
  ├─ FrequencyPenalty, PresencePenalty, Seed
  ├─ LogitBias, Stop, User, Metadata
  └─ 更新 OpenAI/Anthropic 转换逻辑

阶段 2: 响应格式和工具增强 (2-3天)
  ├─ ResponseFormat (JSON Schema)
  ├─ ToolChoice, ParallelToolCalls
  └─ StreamOptions

阶段 3: 多模态内容支持 (3-4天)
  ├─ 重构 Content 为复杂类型
  ├─ 支持图像、音频输入
  └─ Modalities, Audio 字段

阶段 4: Embedding API 集成 (2-3天)
  ├─ EmbeddingInput, Dimensions
  └─ Embedding 请求/响应转换

阶段 5: 缓存和推理优化 (2-3天)
  ├─ PromptCacheKey, CacheControl
  └─ ReasoningBudget, EnableThinking

阶段 6: 扩展性和元数据 (1-2天)
  ├─ ExtraBody, RawRequest
  └─ TransformerMetadata, TransformOptions

阶段 7: 测试和文档完善 (2-3天)
  ├─ 单元测试、集成测试
  └─ 文档更新
```

### 优先级建议
**高优先级**: 阶段 1, 2 (基础参数 + 响应格式)
**中优先级**: 阶段 3, 4 (多模态 + Embedding)
**低优先级**: 阶段 5, 6 (缓存优化 + 扩展性)

---

## 规划成果总结

### 已创建的文档
1. ✅ **task_plan.md** (7 阶段实施计划)
   - 详细的阶段划分和任务分解
   - 风险评估和缓解策略
   - 时间估算: 14-21 天

2. ✅ **findings.md** (技术发现和洞察)
   - 代码库现状分析
   - 辅助函数分析和补充建议
   - 阶段 1 详细实施步骤
   - 兼容性矩阵
   - 性能考虑

3. ✅ **implementation_checklist.md** (详细实施清单)
   - 阶段 1-7 的完整清单
   - 质量检查清单
   - 风险缓解清单

4. ✅ **QUICKSTART.md** (快速开始指南)
   - 阶段 1 完整实施步骤 (代码示例)
   - 预计 2-3 小时完成
   - 验证清单和常见问题

### 规划特点
- **渐进式**: 7 个独立阶段，每个阶段都可独立使用
- **低风险**: 优先实施简单、常用的功能
- **向后兼容**: 所有新字段都是可选的，使用指针类型
- **测试驱动**: 每个阶段都有完整的测试用例

---

## 下一步行动

### 选项 A: 立即开始实施阶段 1 (推荐)
**优势**:
- 阶段 1 最简单，风险最低
- 2-3 小时即可完成
- 立即获得常用功能 (FrequencyPenalty, Seed 等)
- 为后续阶段打好基础

**行动**:
```bash
# 1. 创建功能分支
git checkout -b feature/unified-request-phase1

# 2. 按照 QUICKSTART.md 实施
# 3. 运行测试验证
# 4. 提交代码
```

### 选项 B: 先确认规划细节
如果你想先确认以下问题：

1. **实施范围**
   - 是否实施全部 7 个阶段？
   - 还是只实施高优先级阶段 (1, 2)？

2. **实施顺序**
   - 是否同意建议的阶段顺序？
   - 是否有特定功能需要优先实施？

3. **向后兼容性**
   - 是否必须保持 100% 向后兼容？
   - 是否允许 Content 字段重构 (阶段 3)？

4. **测试要求**
   - 测试覆盖率目标？(建议 >85%)
   - 是否需要性能基准测试？

### 选项 C: 调整规划
如果你对规划有任何疑问或建议，我可以：
- 调整阶段划分
- 修改实施顺序
- 补充更多细节
- 简化某些阶段

---

## 我的建议

**建议从阶段 1 开始实施**，原因：
1. ✅ 风险最低，不需要重构现有结构
2. ✅ 功能实用，FrequencyPenalty/Seed 等是常用参数
3. ✅ 快速完成，2-3 小时即可看到成果
4. ✅ 验证流程，为后续阶段积累经验

完成阶段 1 后，可以根据实际情况决定是否继续。

---

## 准备就绪

所有规划文档已完成，代码实施步骤已详细说明。

**请告诉我你的决定**：
- A. 立即开始实施阶段 1
- B. 先确认规划细节 (请提出你的问题)
- C. 调整规划 (请说明你的需求)

---

## 测试计划

### 单元测试清单
- [ ] `TransformOpenAIToUnified` - 新字段解析
- [ ] `TransformUnifiedToOpenAI` - 新字段输出
- [ ] `TransformUnifiedToAnthropic` - 字段映射
- [ ] 边界情况测试 (nil, 空值, 零值)

### 集成测试清单
- [ ] 端到端转换测试
- [ ] 多提供商兼容性测试
- [ ] 流式和非流式测试

### 性能测试清单
- [ ] 转换延迟基准测试
- [ ] 内存使用测试
- [ ] 并发转换测试

---

## 问题和阻塞

### 当前问题
- 无

### 需要澄清
1. 用户是否希望实施全部阶段还是部分阶段？
2. 是否有时间限制或截止日期？
3. 是否有特定的功能需求优先级？

---

## 资源和参考

### 文档
- ✅ `docs/UNIFIED_REQUEST_EXTENSION.md` - 已读取
- ✅ `CLAUDE.md` - 项目规范
- ✅ `memory/MEMORY.md` - 项目记忆

### 代码文件
- ✅ `service/transformer.go` - 已分析
- ✅ `service/transform_openai.go` - 已浏览
- ⏳ `service/transform_anthropic.go` - 待详细分析
- ⏳ `service/transform_responses.go` - 待详细分析

### 外部参考
- Octopus 项目 (参考来源)
- OpenAI API 文档
- Anthropic API 文档

---

## 会话统计

### 工具调用
- Read: 3 次
- Glob: 3 次
- Grep: 1 次
- Write: 3 次 (task_plan.md, findings.md, progress.md)
- Bash: 1 次 (session-catchup)

### 文件创建
- ✅ `task_plan.md` - 实施计划
- ✅ `findings.md` - 研究发现
- ✅ `progress.md` - 进度日志

---

**最后更新**: 2026-02-25
**下次更新**: 等待用户确认后
