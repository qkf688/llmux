# 虚拟模型功能使用指南

## 功能概述

虚拟模型功能允许你将多个真实模型组合成一个虚拟模型，提供两层负载均衡能力。客户端可以直接请求虚拟模型名称，系统会自动根据配置的路由策略选择合适的真实模型和提供商。

## 核心概念

### 两层负载均衡与故障转移

```
客户端请求 → 虚拟模型 → 真实模型 → 提供商模型 → 实际提供商
            (第一层)      (第二层)
```

- **第一层（真实模型级别）**：虚拟模型根据路由策略选择真实模型，支持故障转移
- **第二层（提供商级别）**：真实模型根据优先级和权重选择提供商（复用现有逻辑）

**故障转移机制**：
- 当一个真实模型的所有提供商都失败后，系统会自动切换到下一个真实模型
- 每个真实模型都会尝试其所有可用的提供商
- 全局超时控制：虚拟模型的 TimeOut 作为所有真实模型的总超时时间
- 每模型重试：虚拟模型的 MaxRetry 覆盖每个真实模型的重试次数

**执行流程示例**：
```
虚拟模型 "balanced-gpt" (关联: gpt-4, claude-3.5)
  ↓ 选择 gpt-4
  ├─ OpenAI 提供商 ❌
  ├─ Azure 提供商 ❌
  └─ Cloudflare 提供商 ❌
  ↓ gpt-4 所有提供商失败，切换真实模型
  ↓ 选择 claude-3.5
  ├─ Anthropic 提供商 ✅
  → 返回成功
```

### 路由策略

1. **priority**（优先级 + 权重）
   - 按优先级降序依次尝试真实模型
   - 同优先级按权重降序排序
   - 每个真实模型失败后自动切换到下一个
   - 适合：成本优化、性能分级、故障转移

2. **round_robin**（轮询）
   - 从当前索引开始依次尝试真实模型
   - 成功后更新索引（下次从下一个开始）
   - 每个真实模型失败后自动切换到下一个
   - 适合：负载均衡、均匀分配

3. **random**（随机）
   - 随机打乱真实模型顺序后依次尝试
   - 每个真实模型失败后自动切换到下一个
   - 适合：简单场景、测试

## 快速开始

### 1. 创建虚拟模型

通过前端界面：
1. 访问 http://localhost:7070
2. 点击左侧菜单"虚拟模型"
3. 点击"创建虚拟模型"按钮
4. 填写表单：
   - 名称：`gpt-4-balanced`
   - 描述：`多提供商负载均衡`
   - 路由策略：`优先级+权重`
   - 最大重试次数：`10`
   - 超时时间：`60`
   - 启用：`是`
5. 点击"创建"

通过 API：
```bash
curl -X POST http://localhost:7070/api/virtual-models \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "name": "gpt-4-balanced",
    "description": "多提供商负载均衡",
    "strategy": "priority",
    "max_retry": 10,
    "time_out": 60,
    "io_log": false,
    "enabled": true
  }'
```

### 2. 添加映射关系

通过前端界面：
1. 在虚拟模型列表中，点击"管理映射"按钮
2. 点击"添加映射"
3. 选择一个或多个真实模型（如 `gpt-4-openai`、`gpt-4-azure`）
4. 设置统一参数：优先级、权重、启用状态
5. 点击"添加映射（N）"
6. 如不同真实模型需要不同参数，可添加后在映射列表中逐条编辑

通过 API：
```bash
# 添加第一个映射：gpt-4-openai
curl -X POST http://localhost:7070/api/virtual-models/1/mappings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "real_model_id": 1,
    "priority": 10,
    "weight": 5,
    "enabled": true
  }'

# 添加第二个映射：gpt-4-azure
curl -X POST http://localhost:7070/api/virtual-models/1/mappings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "real_model_id": 2,
    "priority": 10,
    "weight": 5,
    "enabled": true
  }'

# 添加第三个映射：gpt-4-proxy（备用）
curl -X POST http://localhost:7070/api/virtual-models/1/mappings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "real_model_id": 3,
    "priority": 5,
    "weight": 3,
    "enabled": true
  }'
```

### 3. 使用虚拟模型

客户端请求时，直接使用虚拟模型名称：

```bash
curl -X POST http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "gpt-4-balanced",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

系统会自动：
1. 识别 `gpt-4-balanced` 是虚拟模型
2. 根据 `priority` 策略选择真实模型（如 `gpt-4-openai`）
3. 从真实模型的提供商中选择一个（第二层负载均衡）
4. 发起实际请求

## 应用场景

### 场景 1：多提供商负载均衡

**需求**：有多个 GPT-4 提供商，希望均匀分配请求

**配置**：
```
虚拟模型: "gpt-4-balanced"
策略: priority
映射:
  - gpt-4-openai   (Priority: 10, Weight: 5)
  - gpt-4-azure    (Priority: 10, Weight: 5)
  - gpt-4-proxy    (Priority: 10, Weight: 5)
```

**效果**：请求会按权重随机分配到三个提供商

### 场景 2：成本优化与故障转移

**需求**：优先使用便宜的模型，失败时自动降级到贵的模型

**配置**：
```
虚拟模型: "gpt-4-cost-optimized"
策略: priority
映射:
  - gpt-4-turbo    (Priority: 10, Weight: 5)  # 便宜但快
  - gpt-4          (Priority: 5,  Weight: 5)  # 贵但稳定
```

**效果**：
1. 优先尝试 gpt-4-turbo 的所有提供商
2. 如果 gpt-4-turbo 的所有提供商都失败，自动切换到 gpt-4
3. 尝试 gpt-4 的所有提供商
4. 实现成本优化和故障转移的双重目标

### 场景 3：多模型故障转移

**需求**：主提供商故障时自动切换到备用提供商，支持多个备用

**配置**：
```
虚拟模型: "claude-3-ha"
策略: priority
映射:
  - claude-3-opus-main     (Priority: 10)  # 主
  - claude-3-opus-backup1  (Priority: 5)   # 备用 1
  - claude-3-opus-backup2  (Priority: 1)   # 备用 2
```

**效果**：
1. 优先尝试 claude-3-opus-main 的所有提供商
2. 如果主模型的所有提供商都失败，切换到 backup1
3. 如果 backup1 的所有提供商都失败，切换到 backup2
4. 实现多层故障转移，确保高可用

### 场景 4：轮询负载均衡

**需求**：请求均匀分配到多个提供商

**配置**：
```
虚拟模型: "gpt-4-round-robin"
策略: round_robin
映射:
  - gpt-4-provider1  (Priority: 10, Weight: 5)
  - gpt-4-provider2  (Priority: 10, Weight: 5)
  - gpt-4-provider3  (Priority: 10, Weight: 5)
```

**效果**：请求依次分配到三个提供商（1 → 2 → 3 → 1 → ...）

## API 参考

### 虚拟模型管理

#### 获取虚拟模型列表
```
GET /api/virtual-models
```

#### 创建虚拟模型
```
POST /api/virtual-models
Content-Type: application/json

{
  "name": "string",           // 虚拟模型名称（必填）
  "description": "string",    // 描述
  "strategy": "string",       // 路由策略：priority, round_robin, random
  "max_retry": 10,            // 最大重试次数
  "time_out": 60,             // 超时时间（秒）
  "io_log": false,            // 是否记录 IO 日志
  "enabled": true             // 是否启用
}
```

#### 更新虚拟模型
```
PUT /api/virtual-models/:id
Content-Type: application/json

{
  "name": "string",
  "description": "string",
  "strategy": "string",
  "max_retry": 10,
  "time_out": 60,
  "io_log": false,
  "enabled": true
}
```

#### 删除虚拟模型
```
DELETE /api/virtual-models/:id
```

### 映射关系管理

#### 获取映射关系
```
GET /api/virtual-models/:id/mappings
```

#### 创建映射
```
POST /api/virtual-models/:id/mappings
Content-Type: application/json

{
  "real_model_id": 1,    // 真实模型 ID（必填）
  "priority": 10,        // 优先级
  "weight": 5,           // 权重
  "enabled": true        // 是否启用
}
```

#### 更新映射
```
PUT /api/virtual-models/:id/mappings/:mapping_id
Content-Type: application/json

{
  "real_model_id": 1,
  "priority": 10,
  "weight": 5,
  "enabled": true
}
```

#### 删除映射
```
DELETE /api/virtual-models/:id/mappings/:mapping_id
```

#### 获取统计信息
```
GET /api/virtual-models/:id/stats
```

响应示例：
```json
{
  "virtual_model_id": 1,
  "virtual_model_name": "gpt-4-balanced",
  "strategy": "priority",
  "total_mappings": 3,
  "enabled_mappings": 3,
  "disabled_mappings": 0
}
```

## 注意事项

### 1. 名称冲突
- 虚拟模型名称不能与真实模型名称冲突
- 创建时会自动检测并返回错误

### 2. 循环依赖
- 虚拟模型不能关联自己
- 创建映射时会自动检测

### 3. 级联删除
- 删除虚拟模型时，会自动删除所有映射关系
- 操作不可撤销，请谨慎操作

### 4. 路由策略选择
- **priority**：适合大多数场景，推荐使用
- **round_robin**：适合需要严格均匀分配的场景
- **random**：适合简单测试场景

### 5. 优先级和权重
- 优先级：数值越大越优先（建议范围：1-100）
- 权重：数值越大被选中概率越高（建议范围：1-10）
- 同优先级时按权重随机选择

## 故障排查

### 问题 1：虚拟模型请求失败

**症状**：客户端请求虚拟模型时返回 404 或错误

**排查步骤**：
1. 检查虚拟模型是否启用
2. 检查是否有启用的映射关系
3. 检查真实模型是否存在
4. 检查真实模型是否有可用的提供商
5. 查看日志中的错误信息（slog 会记录详细的故障转移过程）

### 问题 2：所有真实模型都失败

**症状**：虚拟模型请求返回 "all real models exhausted for virtual model"

**原因**：所有真实模型的所有提供商都失败了

**排查步骤**：
1. 查看日志，确认每个真实模型的失败原因
2. 检查提供商的 API 密钥是否有效
3. 检查提供商的配额是否用完
4. 检查网络连接是否正常
5. 考虑增加更多备用真实模型

### 问题 3：请求超时

**症状**：虚拟模型请求返回 "virtual model global timeout"

**原因**：全局超时时间不足以尝试所有真实模型

**解决方案**：
- 增加虚拟模型的 TimeOut 设置
- 减少真实模型的数量
- 减少每个真实模型的 MaxRetry 次数
- 优化提供商的响应时间

### 问题 4：请求总是路由到同一个模型

**症状**：使用 priority 策略时，请求总是路由到同一个模型

**原因**：可能是优先级设置不当，或者只有一个真实模型可用

**解决方案**：
- 检查映射关系的优先级设置
- 确保多个映射有相同的优先级（如果希望负载均衡）
- 或使用 round_robin 策略
- 检查其他真实模型是否被禁用

### 问题 5：轮询策略不生效

**症状**：使用 round_robin 策略时，请求没有轮询

**原因**：可能是映射关系配置错误，或者某些真实模型失败

**解决方案**：
- 确保有多个启用的映射关系
- 检查真实模型是否都可用
- 查看日志确认轮询索引是否正确更新
- 注意：只有请求成功后才会更新轮询索引

## 最佳实践

### 1. 命名规范
- 虚拟模型名称建议使用描述性名称
- 例如：`gpt-4-balanced`, `claude-3-ha`, `gpt-4-cost-optimized`

### 2. 优先级设置
- 主要提供商：Priority 10
- 备用提供商：Priority 5
- 最后备用：Priority 1
- 建议优先级差距至少为 5，确保故障转移顺序清晰

### 3. 权重设置
- 高性能提供商：Weight 5-10
- 普通提供商：Weight 3-5
- 测试提供商：Weight 1-2
- 同优先级时，权重决定被选中的概率

### 4. 超时和重试配置
- **TimeOut**：建议设置为 60-120 秒
  - 考虑所有真实模型的总尝试时间
  - 流式响应可能需要更长时间
- **MaxRetry**：建议设置为 5-10 次
  - 每个真实模型会独立重试这么多次
  - 过多的重试会增加延迟

### 5. 故障转移策略
- 至少配置 2 个真实模型，确保故障转移
- 主模型和备用模型应该来自不同的提供商
- 定期测试故障转移是否正常工作

### 6. 监控和维护
- 定期查看虚拟模型统计信息
- 根据实际使用情况调整优先级和权重
- 及时禁用不可用的映射关系
- 查看日志了解故障转移的频率和原因

### 7. 日志分析
- 虚拟模型请求会在日志中记录：
  - 虚拟模型名称（Name 字段）
  - 当前尝试的真实模型（slog 日志）
  - 提供商信息（ProviderName 字段）
  - 重试次数（Retry 字段）
- 通过日志可以分析：
  - 哪些真实模型经常失败
  - 故障转移是否按预期工作
  - 是否需要调整优先级和权重

## 技术细节

### 超时机制
- **全局超时**：虚拟模型的 TimeOut 作为所有真实模型的总超时时间
- 超时后立即返回错误，不再尝试其他真实模型
- 在外层循环（真实模型）和内层循环（提供商）中都会检查超时

### 重试机制
- **每模型重试**：虚拟模型的 MaxRetry 覆盖每个真实模型的 MaxRetry
- 每个真实模型独立重试，确保公平的尝试机会
- 真实模型的所有提供商都失败后，才会切换到下一个真实模型

### Round Robin 索引更新
- 只有在请求成功后才会更新轮询索引
- 索引存储在内存中，重启后会重置
- 并发安全：使用 sync.RWMutex 保护

### 权重和优先级衰减
- 失败的提供商会触发权重和优先级衰减
- 衰减只针对提供商级别（ModelWithProvider）
- 不影响虚拟模型的真实模型选择顺序

## 更多信息

- 详细实现文档：`planning-simple/SUMMARY.md`
- 技术发现：`planning-simple/findings.md`
- 项目规范：`CLAUDE.md`
