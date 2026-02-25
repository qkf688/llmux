# 批量设置模型参数功能设计

## 概述

在模型管理页面 (`/models`) 添加批量设置功能，允许用户一次性修改多个模型的重试次数和超时时间。

## 功能需求

### 核心特性

1. **触发方式**：选中模型后，在"批量删除"按钮旁显示"批量设置"按钮
2. **设置模式**：统一设置所有选中模型为相同的值
3. **可选字段**：支持只更新重试次数或超时时间，或两者都更新
4. **信息展示**：显示选中数量和当前值范围统计
5. **默认值**：重试次数默认 10，超时时间默认 60 秒

### 用户交互流程

1. 用户通过复选框选中一个或多个模型
2. 点击"批量设置"按钮
3. 弹出对话框，显示：
   - 选中模型数量
   - 当前重试次数范围（最小值 - 最大值）
   - 当前超时时间范围（最小值 - 最大值）
4. 用户可以：
   - 勾选/取消勾选要更新的字段
   - 修改输入框的值
5. 点击"确认设置"提交（至少勾选一个字段）
6. 成功后：显示成功提示、关闭对话框、刷新列表、清空选中状态
7. 失败后：显示错误提示，对话框保持打开

## API 设计

### 新增端点

**路径**：`PUT /api/v1/models/batch-update`

**请求体**：
```json
{
  "ids": [1, 2, 3],
  "max_retry": 10,      // 可选，只有勾选时才传
  "time_out": 60        // 可选，只有勾选时才传
}
```

**响应体（成功）**：
```json
{
  "success": true,
  "data": {
    "updated": 3
  },
  "message": "成功更新 3 个模型"
}
```

**响应体（失败）**：
```json
{
  "success": false,
  "error": "VALIDATION_ERROR",
  "message": "参数验证失败"
}
```

**HTTP 状态码**：
- 200: 更新成功
- 400: 参数验证失败（ids 为空、值为负数、至少一个字段必填等）
- 404: 部分模型 ID 不存在
- 500: 数据库更新失败

## 前端设计

### UI 组件

#### 1. 批量设置按钮

- **位置**：在"批量删除"按钮旁边
- **显示条件**：`selectedIds.length > 0`
- **样式**：`variant="default"` 的 Button 组件
- **文本**：`批量设置 ({selectedIds.length})`

#### 2. 批量设置对话框

**对话框结构**：

```
┌─────────────────────────────────────┐
│ 批量设置模型参数                      │
│ 为选中的 X 个模型统一设置参数          │
├─────────────────────────────────────┤
│ [统计信息卡片]                        │
│ • 已选中：X 个模型                    │
│ • 当前重试次数范围：min - max         │
│ • 当前超时时间范围：min - max         │
├─────────────────────────────────────┤
│ ☑ 重试次数限制：[10        ]         │
│ ☑ 超时时间(秒)：[60        ]         │
│                                      │
│ 提示：只有勾选的字段才会被更新         │
├─────────────────────────────────────┤
│              [取消] [确认设置]        │
└─────────────────────────────────────┘
```

**字段说明**：
- 两个复选框默认勾选
- 输入框默认值：重试次数 10，超时时间 60
- "确认设置"按钮禁用条件：两个复选框都未勾选

#### 3. 表单验证

使用 Zod schema：
```typescript
const batchUpdateSchema = z.object({
  enableMaxRetry: z.boolean(),
  enableTimeOut: z.boolean(),
  max_retry: z.number().min(0),
  time_out: z.number().min(0),
}).refine(
  data => data.enableMaxRetry || data.enableTimeOut,
  { message: "至少选择一个字段进行更新" }
);
```

### 状态管理

新增状态：
```typescript
const [batchSettingsDialogOpen, setBatchSettingsDialogOpen] = useState(false);
const [batchUpdating, setBatchUpdating] = useState(false);
```

表单状态（React Hook Form）：
```typescript
{
  enableMaxRetry: true,
  enableTimeOut: true,
  max_retry: 10,
  time_out: 60
}
```

### 统计信息计算

```typescript
const selectedModels = models.filter(m => selectedIds.includes(m.ID));
const maxRetryRange = {
  min: Math.min(...selectedModels.map(m => m.MaxRetry)),
  max: Math.max(...selectedModels.map(m => m.MaxRetry))
};
const timeOutRange = {
  min: Math.min(...selectedModels.map(m => m.TimeOut)),
  max: Math.max(...selectedModels.map(m => m.TimeOut))
};
```

## 后端实现

### 处理函数

**文件**：`handler/api.go`

```go
// BatchUpdateModels 批量更新模型参数
func BatchUpdateModels(c *gin.Context) {
    var req struct {
        IDs      []uint `json:"ids" binding:"required,min=1"`
        MaxRetry *int   `json:"max_retry"` // 指针类型，nil 表示不更新
        TimeOut  *int   `json:"time_out"`  // 指针类型，nil 表示不更新
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, common.ErrorResponse("参数验证失败"))
        return
    }

    // 验证至少有一个字段需要更新
    if req.MaxRetry == nil && req.TimeOut == nil {
        c.JSON(http.StatusBadRequest, common.ErrorResponse("至少需要更新一个字段"))
        return
    }

    // 验证值的合法性
    if req.MaxRetry != nil && *req.MaxRetry < 0 {
        c.JSON(http.StatusBadRequest, common.ErrorResponse("重试次数不能为负数"))
        return
    }
    if req.TimeOut != nil && *req.TimeOut < 0 {
        c.JSON(http.StatusBadRequest, common.ErrorResponse("超时时间不能为负数"))
        return
    }

    // 构建更新 map
    updates := make(map[string]interface{})
    if req.MaxRetry != nil {
        updates["max_retry"] = *req.MaxRetry
    }
    if req.TimeOut != nil {
        updates["time_out"] = *req.TimeOut
    }

    // 执行批量更新
    result := models.DB.Model(&models.Model{}).
        Where("id IN ?", req.IDs).
        Updates(updates)

    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, common.ErrorResponse("更新失败"))
        return
    }

    c.JSON(http.StatusOK, common.SuccessResponse(map[string]interface{}{
        "updated": result.RowsAffected,
    }, fmt.Sprintf("成功更新 %d 个模型", result.RowsAffected)))
}
```

### 路由注册

**文件**：`handler/home.go` 或路由配置文件

```go
v1.PUT("/models/batch-update", BatchUpdateModels)
```

## 实现清单

### 后端变更

| 文件 | 变更内容 |
|------|----------|
| `handler/api.go` | 新增 `BatchUpdateModels` 函数 |
| 路由配置文件 | 注册 `PUT /api/v1/models/batch-update` 路由 |

### 前端变更

| 文件 | 变更内容 |
|------|----------|
| `webui/src/lib/api.ts` | 新增 `batchUpdateModels` API 函数 |
| `webui/src/routes/models.tsx` | 新增批量设置按钮、对话框、表单逻辑 |

## 测试计划

### 功能测试

1. **基本功能**
   - 选中单个模型，批量设置成功
   - 选中多个模型，批量设置成功
   - 只更新重试次数
   - 只更新超时时间
   - 同时更新两个字段

2. **边界情况**
   - 未选中任何模型时，按钮不显示
   - 两个复选框都未勾选时，提交按钮禁用
   - 输入负数时，显示验证错误
   - 输入 0 时，更新成功

3. **错误处理**
   - 网络错误时显示错误提示
   - 部分模型 ID 不存在时的处理
   - 数据库更新失败时的处理

### UI 测试

1. **响应式布局**
   - 桌面端显示正常
   - 移动端显示正常

2. **交互体验**
   - 统计信息显示正确
   - 加载状态显示正确
   - 成功/失败提示显示正确
   - 对话框关闭后状态重置

## 安全考虑

1. **输入验证**
   - 后端必须验证所有输入参数
   - 防止 SQL 注入（使用 GORM 参数化查询）
   - 验证 IDs 数组不为空
   - 验证数值范围合法

2. **权限控制**
   - 如果未来添加权限系统，需要验证用户是否有权限批量修改模型

## 性能考虑

1. **批量更新优化**
   - 使用单条 SQL 语句批量更新，避免循环更新
   - 限制单次批量更新的模型数量（建议最多 100 个）

2. **前端性能**
   - 统计信息计算在选中模型变化时进行
   - 避免不必要的重渲染

## 未来扩展

1. **更多批量操作**
   - 批量启用/禁用 IO 记录
   - 批量修改备注
   - 批量关联提供商

2. **批量操作历史**
   - 记录批量操作日志
   - 支持撤销操作

3. **预设配置**
   - 保存常用的配置组合
   - 快速应用预设配置

---

**文档版本**：1.0
**创建日期**：2026-02-04
**作者**：Claude Code
