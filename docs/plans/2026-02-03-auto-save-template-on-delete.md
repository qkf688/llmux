# 删除关联模型时自动保存到模板功能设计

## 需求背景

当前系统中，模板项（ModelTemplateItem）是动态合并展示的，来源包括：
1. 模型名称（model.Name）- 标记为 "model_name"
2. 关联的 ProviderModel（associations 中的 assoc.ProviderModel）- 标记为 "association"
3. 手动模板项（manualItems 中的 item.Name）- 标记为 "manual"

**问题**：当删除 ModelWithProvider 时，如果该 ProviderModel 只存在于 "association" 来源（即只来自关联关系，没有被手动添加到 ModelTemplateItem），它会从模板项列表中消失。

**需求**：增加一个开关，让用户可以选择在删除 ModelWithProvider 时，自动将该 ProviderModel 保存到 ModelTemplateItem 中，这样即使关联被删除，这个模型名称也会保留在模板中。

## 功能设计

### 1. 新增设置项

**设置键名**：`auto_save_template_on_delete`

**设置说明**：删除关联模型时自动保存到模板

**默认值**：`false`（关闭）

**行为**：
- **开启时**：删除 ModelWithProvider 前，自动将该 ProviderModel 保存到 ModelTemplateItem 中（如果尚未存在）
- **关闭时**：保持当前行为，直接删除 ModelWithProvider，不保存到模板

### 2. 实现逻辑

#### 2.1 单个删除（DeleteModelProvider）

```
1. 获取要删除的 ModelWithProvider 记录
2. 检查 auto_save_template_on_delete 设置
3. 如果开启：
   a. 检查该 ProviderModel 是否已存在于 ModelTemplateItem 中
   b. 如果不存在，创建新的 ModelTemplateItem 记录
4. 执行删除操作
```

#### 2.2 批量删除（BatchDeleteModelProviders）

```
1. 获取要删除的所有 ModelWithProvider 记录
2. 检查 auto_save_template_on_delete 设置
3. 如果开启：
   a. 遍历每个 ModelWithProvider
   b. 检查该 ProviderModel 是否已存在于 ModelTemplateItem 中
   c. 如果不存在，批量创建 ModelTemplateItem 记录
4. 执行批量删除操作
```

### 3. 数据库操作

**查询是否存在**：
```sql
SELECT COUNT(*) FROM model_template_items
WHERE model_id = ? AND name = ?
```

**插入模板项**：
```sql
INSERT INTO model_template_items (model_id, name, created_at, updated_at)
VALUES (?, ?, ?, ?)
```

### 4. 边界情况处理

1. **重复检查**：保存前检查是否已存在，避免唯一索引冲突
2. **空值处理**：ProviderModel 为空时跳过保存
3. **事务处理**：保存和删除操作不需要在同一事务中（保存失败不影响删除）
4. **错误处理**：保存失败记录日志，但不阻止删除操作

## 实施计划

### 阶段 1：后端数据模型修改

**文件**：`models/model.go`

**修改内容**：
```go
const (
    // ... 现有常量 ...

    // 模型关联相关设置
    SettingKeyAutoAssociateOnAdd       = "auto_associate_on_add"        // 添加模型时自动关联
    SettingKeyAutoCleanOnDelete        = "auto_clean_on_delete"         // 删除模型时自动清理关联
    SettingKeyAutoSaveTemplateOnDelete = "auto_save_template_on_delete" // 删除关联时自动保存到模板
)
```

### 阶段 2：数据库默认设置

**文件**：`models/init.go`

**修改内容**：
```go
func initDefaultSettings(ctx context.Context) {
    defaultSettings := []Setting{
        // ... 现有设置 ...

        // 模型关联相关默认设置
        {Key: SettingKeyAutoAssociateOnAdd, Value: "false"},       // 默认关闭自动关联
        {Key: SettingKeyAutoCleanOnDelete, Value: "false"},        // 默认关闭自动清理
        {Key: SettingKeyAutoSaveTemplateOnDelete, Value: "false"}, // 默认关闭自动保存到模板
    }
    // ...
}
```

### 阶段 3：API 响应结构修改

**文件**：`handler/api.go`

**修改内容**：

1. **SettingsResponse 结构体**：
```go
type SettingsResponse struct {
    // ... 现有字段 ...

    // 模型关联相关设置
    AutoAssociateOnAdd       bool `json:"auto_associate_on_add"`
    AutoCleanOnDelete        bool `json:"auto_clean_on_delete"`
    AutoSaveTemplateOnDelete bool `json:"auto_save_template_on_delete"` // 新增
}
```

2. **GetSettings 函数**：
```go
case models.SettingKeyAutoSaveTemplateOnDelete:
    response.AutoSaveTemplateOnDelete = setting.Value == "true"
```

3. **UpdateSettings 函数**：
```go
// 更新 auto_save_template_on_delete 设置
autoSaveTemplateValue := "false"
if req.AutoSaveTemplateOnDelete {
    autoSaveTemplateValue = "true"
}
if _, err := gorm.G[models.Setting](models.DB).
    Where("key = ?", models.SettingKeyAutoSaveTemplateOnDelete).
    Update(ctx, "value", autoSaveTemplateValue); err != nil {
    common.InternalServerError(c, "Failed to update settings: "+err.Error())
    return
}
```

### 阶段 4：删除逻辑修改

**文件**：`handler/api.go`

#### 4.1 新增辅助函数

```go
// saveProviderModelToTemplate 将 ProviderModel 保存到模板项
func saveProviderModelToTemplate(ctx context.Context, modelID uint, providerModel string) error {
    if providerModel == "" {
        return nil
    }

    // 检查是否已存在
    count, err := gorm.G[models.ModelTemplateItem](models.DB).
        Where("model_id = ? AND name = ?", modelID, providerModel).
        Count(ctx, "id")
    if err != nil {
        return err
    }

    // 如果不存在则创建
    if count == 0 {
        item := models.ModelTemplateItem{
            ModelID: modelID,
            Name:    providerModel,
        }
        if err := gorm.G[models.ModelTemplateItem](models.DB).Create(ctx, &item); err != nil {
            return err
        }
        slog.Info("auto-saved provider model to template",
            "model_id", modelID,
            "provider_model", providerModel)
    }

    return nil
}
```

#### 4.2 修改 DeleteModelProvider 函数

```go
func DeleteModelProvider(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        common.BadRequest(c, "Invalid ID format")
        return
    }

    ctx := c.Request.Context()

    // 检查是否启用自动保存到模板
    autoSave := getSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnDelete)
    if autoSave {
        // 获取要删除的记录
        assoc, err := gorm.G[models.ModelWithProvider](models.DB).
            Where("id = ?", id).
            First(ctx)
        if err == nil {
            // 保存到模板（失败不影响删除）
            if err := saveProviderModelToTemplate(ctx, assoc.ModelID, assoc.ProviderModel); err != nil {
                slog.Warn("failed to save provider model to template",
                    "error", err,
                    "model_id", assoc.ModelID,
                    "provider_model", assoc.ProviderModel)
            }
        }
    }

    // 执行删除
    result, err := gorm.G[models.ModelWithProvider](models.DB).
        Where("id = ?", id).
        Delete(ctx)
    if err != nil {
        common.InternalServerError(c, "Failed to delete model-provider association: "+err.Error())
        return
    }

    if result == 0 {
        common.NotFound(c, "Model-provider association not found")
        return
    }

    common.Success(c, nil)
}
```

#### 4.3 修改 BatchDeleteModelProviders 函数

```go
func BatchDeleteModelProviders(c *gin.Context) {
    var req BatchDeleteModelProvidersRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        common.BadRequest(c, "Invalid request body: "+err.Error())
        return
    }

    if len(req.IDs) == 0 {
        common.BadRequest(c, "No IDs provided")
        return
    }

    ctx := c.Request.Context()

    // 检查是否启用自动保存到模板
    autoSave := getSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnDelete)
    if autoSave {
        // 获取要删除的所有记录
        assocs, err := gorm.G[models.ModelWithProvider](models.DB).
            Where("id IN ?", req.IDs).
            Find(ctx)
        if err == nil {
            // 批量保存到模板
            for _, assoc := range assocs {
                if err := saveProviderModelToTemplate(ctx, assoc.ModelID, assoc.ProviderModel); err != nil {
                    slog.Warn("failed to save provider model to template",
                        "error", err,
                        "model_id", assoc.ModelID,
                        "provider_model", assoc.ProviderModel)
                }
            }
        }
    }

    // 执行批量删除
    result, err := gorm.G[models.ModelWithProvider](models.DB).
        Where("id IN ?", req.IDs).
        Delete(ctx)
    if err != nil {
        common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
        return
    }

    common.Success(c, map[string]interface{}{
        "deleted": result,
    })
}
```

### 阶段 5：前端设置页面修改

**文件**：`webui/src/routes/settings/index.tsx`（或相应的设置页面组件）

**修改内容**：

在"模型关联相关设置"部分添加新的开关项：

```tsx
<div className="space-y-2">
  <Label htmlFor="auto-save-template-on-delete">
    删除关联时自动保存到模板
  </Label>
  <div className="flex items-center space-x-2">
    <Switch
      id="auto-save-template-on-delete"
      checked={settings.auto_save_template_on_delete}
      onCheckedChange={(checked) =>
        handleSettingChange('auto_save_template_on_delete', checked)
      }
    />
    <span className="text-sm text-muted-foreground">
      删除模型关联时，自动将 ProviderModel 保存到模板项中
    </span>
  </div>
</div>
```

## 测试计划

### 单元测试

1. **saveProviderModelToTemplate 函数测试**
   - 测试保存新模板项
   - 测试重复保存（应跳过）
   - 测试空值处理
   - 测试数据库错误处理

2. **DeleteModelProvider 测试**
   - 测试开关关闭时的行为（不保存）
   - 测试开关开启时的行为（保存）
   - 测试保存失败不影响删除

3. **BatchDeleteModelProviders 测试**
   - 测试批量删除时的保存行为
   - 测试部分保存失败的情况

### 集成测试

1. **完整流程测试**
   - 创建 Model 和 ModelWithProvider
   - 开启 auto_save_template_on_delete 设置
   - 删除 ModelWithProvider
   - 验证 ModelTemplateItem 中存在该记录
   - 验证模板项列表中仍显示该 ProviderModel

2. **边界情况测试**
   - 删除已存在于 ModelTemplateItem 中的 ModelWithProvider
   - 删除 ProviderModel 为空的 ModelWithProvider
   - 批量删除多个 ModelWithProvider

### 手动测试

1. 在设置页面开启/关闭开关
2. 创建测试数据并执行删除操作
3. 验证前端显示是否正确
4. 检查数据库数据是否符合预期

## 风险评估

### 低风险
- 新增功能，不影响现有逻辑
- 默认关闭，用户需主动开启
- 保存失败不影响删除操作

### 需要注意
- 唯一索引冲突处理（已通过查询避免）
- 日志记录保存失败的情况
- 性能影响（批量删除时的额外查询）

## 回滚方案

如果出现问题，可以：
1. 将 auto_save_template_on_delete 设置为 false
2. 手动删除不需要的 ModelTemplateItem 记录
3. 回滚代码到之前版本

## 文档更新

需要更新的文档：
1. API 文档：新增设置项说明
2. 用户手册：说明新功能的使用方法
3. CLAUDE.md：更新项目规范（如有必要）

## 时间估算

- 后端开发：2-3 小时
- 前端开发：1 小时
- 测试：1-2 小时
- 文档更新：30 分钟

**总计**：约 5-7 小时

## 验收标准

1. ✅ 新增设置项 auto_save_template_on_delete
2. ✅ 设置默认值为 false
3. ✅ 前端设置页面显示新开关
4. ✅ 开关关闭时，删除行为不变
5. ✅ 开关开启时，删除前保存到 ModelTemplateItem
6. ✅ 保存失败不影响删除操作
7. ✅ 批量删除支持该功能
8. ✅ 日志记录保存操作
9. ✅ 所有测试通过
10. ✅ 文档更新完成
