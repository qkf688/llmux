# 撤销原始请求响应记录功能计划

## 概述

本文档描述如何安全地撤销提交 `4f085f4 - feat(日志): 增加原始请求响应记录功能`。

## 提交信息

- **提交哈希**: 4f085f4
- **提交信息**: feat(日志): 增加原始请求响应记录功能
- **撤销方式**: git revert（保留历史记录）
- **数据处理**: 保留已有数据，不删除数据库字段

## 影响范围分析

### 1. 数据库模型变更 (models/model.go)

**位置**: [`models/model.go:68-73`](models/model.go:68)

**变更内容**:
```go
// ChatLog 结构体中添加的字段
RequestHeaders     string // 请求头JSON字符串
RequestBody        string // 请求体
ResponseHeaders    string // 响应头JSON字符串
ResponseBody       string // 响应体（转换后）
RawResponseBody    string // 原始响应体（转换前）
```

**撤销操作**: 
- 从 ChatLog 结构体中移除这5个字段
- **注意**: 不删除数据库中的实际列，保留已有数据

### 2. 设置常量 (models/model.go)

**位置**: [`models/model.go:138`](models/model.go:138)

**变更内容**:
```go
SettingKeyLogRawRequestResponse = "log_raw_request_response" // 是否记录原始请求响应内容
```

**撤销操作**:
- 从常量定义中移除此行
- **注意**: 不删除数据库中的设置记录，保留配置

### 3. 默认设置初始化 (models/init.go)

**位置**: [`models/init.go:66`](models/init.go:66)

**变更内容**:
```go
{Key: SettingKeyLogRawRequestResponse, Value: "false"},   // 默认关闭原始请求响应记录
```

**撤销操作**:
- 从 initDefaultSettings 函数中移除此行

### 4. 业务逻辑 (service/chat.go)

**涉及位置**:
- [`service/chat.go:122-138`](service/chat.go:122): 检查是否启用记录并准备请求数据
- [`service/chat.go:183-194`](service/chat.go:183): 错误响应时记录请求响应
- [`service/chat.go:213-227`](service/chat.go:213): 记录原始响应体
- [`service/chat.go:247-271`](service/chat.go:247): 更新日志记录原始请求响应内容
- [`service/chat.go:455-471`](service/chat.go:455): RecordLog 函数中记录响应体
- [`service/chat.go:714-723`](service/chat.go:714): getLogRawRequestResponse 辅助函数

**撤销操作**:
- 移除所有与原始请求响应记录相关的代码块
- 移除 getLogRawRequestResponse 函数
- 清理相关变量声明

### 5. API 接口 (handler/api.go)

**涉及位置**:
- [`handler/api.go:905`](handler/api.go:905): SettingsResponse 结构体中的字段
- [`handler/api.go:927`](handler/api.go:927): UpdateSettingsRequest 结构体中的字段
- [`handler/api.go:1013-1014`](handler/api.go:1013): GetSettings 函数中的处理
- [`handler/api.go:1207-1217`](handler/api.go:1207): UpdateSettings 函数中的更新逻辑

**撤销操作**:
- 从 SettingsResponse 和 UpdateSettingsRequest 中移除 LogRawRequestResponse 字段
- 从 GetSettings 和 UpdateSettings 函数中移除相关处理逻辑

### 6. 前端类型定义 (webui/src/lib/api.ts)

**位置**: [`webui/src/lib/api.ts:362-367`](webui/src/lib/api.ts:362)

**变更内容**:
```typescript
// ChatLog 接口中添加的字段
RequestHeaders?: string;
RequestBody?: string;
ResponseHeaders?: string;
ResponseBody?: string;
RawResponseBody?: string;
```

**位置**: [`webui/src/lib/api.ts:444`](webui/src/lib/api.ts:444)

**变更内容**:
```typescript
// Settings 接口中添加的字段
log_raw_request_response: boolean;
```

**撤销操作**:
- 从 ChatLog 接口中移除这5个字段
- 从 Settings 接口中移除 log_raw_request_response 字段

### 7. 前端日志显示 (webui/src/routes/logs.tsx)

**涉及位置**:
- [`webui/src/routes/logs.tsx:563-573`](webui/src/routes/logs.tsx:563): 移动端显示请求响应内容大小
- [`webui/src/routes/logs.tsx:670-717`](webui/src/routes/logs.tsx:670): 详情对话框中显示请求响应内容

**撤销操作**:
- 移除移动端显示请求响应内容大小的代码块
- 移除详情对话框中显示请求响应内容的整个部分

### 8. 前端设置页面 (webui/src/routes/settings.tsx)

需要检查是否有相关的设置开关界面需要移除。

## 撤销步骤

### 步骤 1: 备份数据库（可选但推荐）

```bash
# 如果有重要数据，先备份数据库
cp db/llmio.db db/llmio.db.backup.$(date +%Y%m%d_%H%M%S)
```

### 步骤 2: 使用 git revert 撤销提交

```bash
# 撤销指定提交
git revert 4f085f4

# 如果有冲突，需要手动解决
# 解决冲突后继续
git revert --continue
```

### 步骤 3: 验证变更

检查以下文件是否正确撤销：
- [ ] models/model.go - ChatLog 结构体和常量
- [ ] models/init.go - 默认设置初始化
- [ ] service/chat.go - 业务逻辑
- [ ] handler/api.go - API 接口
- [ ] webui/src/lib/api.ts - 前端类型定义
- [ ] webui/src/routes/logs.tsx - 日志显示界面
- [ ] webui/src/routes/settings.tsx - 设置界面（如有）

### 步骤 4: 编译测试

```bash
# 后端编译测试
go build -o llmio

# 前端编译测试
cd webui
pnpm install
pnpm run build
cd ..
```

### 步骤 5: 运行测试

```bash
# 运行后端测试
go test ./...

# 启动服务测试
./llmio
```

### 步骤 6: 功能验证

- [ ] 日志记录功能正常工作
- [ ] 日志详情页面正常显示（不包含原始请求响应）
- [ ] 设置页面正常工作（不包含原始请求响应开关）
- [ ] 现有日志数据仍然可以正常访问

### 步骤 7: 提交变更

```bash
# 查看变更
git status
git diff

# 提交
git commit -m "revert: 撤销原始请求响应记录功能

撤销提交 4f085f4，移除原始请求响应记录功能。
保留数据库中已有的数据和字段，仅移除代码中的相关逻辑。

影响范围：
- 移除 ChatLog 模型中的原始请求响应字段定义
- 移除相关设置常量和初始化代码
- 移除业务逻辑中的记录代码
- 移除 API 接口中的相关处理
- 移除前端界面中的显示代码"

# 推送到远程
git push origin master
```

## 注意事项

1. **数据保留**: 此撤销方案保留数据库中已有的数据和字段，不会删除任何历史记录
2. **向后兼容**: 由于保留了数据库字段，旧数据仍然可以通过直接数据库查询访问
3. **Git 历史**: 使用 git revert 保留了完整的提交历史，可以随时恢复
4. **测试验证**: 撤销后务必进行完整的功能测试，确保不影响其他功能

## 回滚方案

如果撤销后发现问题，可以通过以下方式回滚：

```bash
# 查看最近的提交
git log --oneline -5

# 撤销 revert 提交（假设 revert 提交的哈希是 abc1234）
git revert abc1234

# 或者直接 reset 到 revert 之前的状态（慎用，会改写历史）
git reset --hard HEAD~1
```

## 后续清理（可选）

如果确认不再需要这些数据，可以在未来某个时间点执行数据库清理：

```sql
-- 清理 chat_logs 表中的原始请求响应字段（可选）
UPDATE chat_logs SET 
  request_headers = NULL,
  request_body = NULL,
  response_headers = NULL,
  response_body = NULL,
  raw_response_body = NULL;

-- 删除设置记录（可选）
DELETE FROM settings WHERE key = 'log_raw_request_response';
```

**警告**: 执行上述 SQL 操作前请务必备份数据库！

## 完成检查清单

- [ ] 代码变更已完成
- [ ] 编译测试通过
- [ ] 功能测试通过
- [ ] Git 提交已完成
- [ ] 变更已推送到远程仓库
- [ ] 团队成员已通知

---

**创建时间**: 2025-12-14  
**创建人**: AI Assistant  
**状态**: 待执行