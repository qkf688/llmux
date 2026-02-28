# Provider Blacklist Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为供应商添加拉黑功能，被拉黑的供应商在一键关联/自动关联时会被跳过，并提供类似模板编辑对话框的管理界面。

**Architecture:** 在 `Provider` 模型上添加 `Blacklisted *bool` 字段，通过专用 API 端点管理黑名单，在所有自动关联触发点（handler + service）添加过滤逻辑；前端在 model-providers 页面增加"拉黑管理"按钮和对话框。

**Tech Stack:** Go + GORM（后端），TypeScript + React + Radix UI Dialog（前端）

---

## 变更概览

| 修改点 | 效果 |
|--------|------|
| `models/model.go` | Provider 新增 Blacklisted 字段 |
| `handler/api.go` | 新增 GET/PUT /api/providers/blacklist 端点，AutoAssociateModels & triggerAutoAssociate 跳过黑名单 |
| `service/model_sync.go` | autoAssociateModels 跳过黑名单供应商 |
| `main.go` | 注册新路由 |
| `webui/src/lib/api.ts` | Provider 接口更新，新增黑名单 API 函数 |
| `webui/src/routes/model-providers.tsx` | 新增"拉黑管理"按钮 + Dialog |

---

### Task 1: 后端 - Provider 模型添加 Blacklisted 字段

**Files:**
- Modify: `models/model.go:10-20`

**Step 1: 修改 Provider 结构体**

在 `models/model.go` 的 `Provider` struct 末尾添加字段：

```go
type Provider struct {
    gorm.Model
    Name               string
    Type               string
    Config             string
    Console            string
    Proxy              string
    ModelEndpoint      *bool
    ModelFilterEnabled *bool
    AuthType           *string
    Blacklisted        *bool `gorm:"default:false" json:"blacklisted"` // 是否拉黑（拉黑后不参与自动关联）
}
```

**Step 2: 确认数据库会自动迁移**

GORM 在启动时会自动迁移（`models/init.go` 中有 AutoMigrate），无需手动操作。

---

### Task 2: 后端 - 新增黑名单 API 处理器

**Files:**
- Modify: `handler/api.go`（在文件末尾或 CleanInvalidAssociations 函数附近添加）

**Step 1: 添加两个新 handler 函数**

在 `handler/api.go` 末尾添加：

```go
// GetProviderBlacklist 获取拉黑的供应商ID列表
func GetProviderBlacklist(c *gin.Context) {
    ctx := c.Request.Context()
    var providers []models.Provider
    if err := models.DB.WithContext(ctx).Where("blacklisted = ?", true).Select("id").Find(&providers).Error; err != nil {
        common.InternalServerError(c, "Failed to get blacklist: "+err.Error())
        return
    }
    ids := make([]uint, 0, len(providers))
    for _, p := range providers {
        ids = append(ids, p.ID)
    }
    common.Success(c, map[string]interface{}{"blacklisted_ids": ids})
}

// UpdateProviderBlacklist 更新供应商黑名单（整体替换：将传入的IDs设为拉黑，其余取消拉黑）
func UpdateProviderBlacklist(c *gin.Context) {
    ctx := c.Request.Context()
    var req struct {
        ProviderIDs []uint `json:"provider_ids"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        common.BadRequest(c, "Invalid request: "+err.Error())
        return
    }

    // 先清空所有拉黑
    falseVal := false
    if err := models.DB.WithContext(ctx).Model(&models.Provider{}).Where("blacklisted = ?", true).Update("blacklisted", &falseVal).Error; err != nil {
        common.InternalServerError(c, "Failed to clear blacklist: "+err.Error())
        return
    }

    // 设置新的拉黑列表
    if len(req.ProviderIDs) > 0 {
        trueVal := true
        if err := models.DB.WithContext(ctx).Model(&models.Provider{}).Where("id IN ?", req.ProviderIDs).Update("blacklisted", &trueVal).Error; err != nil {
            common.InternalServerError(c, "Failed to update blacklist: "+err.Error())
            return
        }
    }

    common.Success(c, map[string]interface{}{"updated": len(req.ProviderIDs)})
}
```

---

### Task 3: 后端 - 注册新路由

**Files:**
- Modify: `main.go`（在 providers blacklist 相关位置，约第 78-90 行附近）

**Step 1: 在 main.go 的 api 路由组中添加**

在 `main.go` 中找到 providers 路由区域，添加：

```go
api.GET("/providers/blacklist", handler.GetProviderBlacklist)
api.PUT("/providers/blacklist", handler.UpdateProviderBlacklist)
```

位置参考：在 `api.GET("/model-providers/auto-associate/preview", ...)` 这行之前或之后。

---

### Task 4: 后端 - 自动关联跳过黑名单供应商

**Files:**
- Modify: `handler/api.go:3358`（AutoAssociateModels 函数的 for 循环）
- Modify: `handler/api.go:3524`（triggerAutoAssociate 函数的 for 循环）

**Step 1: 修改 AutoAssociateModels (行 3358)**

在 `for _, provider := range allProviders {` 循环体的第一行添加：

```go
for _, provider := range allProviders {
    // 跳过被拉黑的供应商
    if provider.Blacklisted != nil && *provider.Blacklisted {
        continue
    }
    // ... 原有代码
```

**Step 2: 同样修改 PreviewAutoAssociate (行约 3198)**

PreviewAutoAssociate 函数中也有 `for _, provider := range allProviders {`，同样加上：

```go
for _, provider := range allProviders {
    // 跳过被拉黑的供应商
    if provider.Blacklisted != nil && *provider.Blacklisted {
        continue
    }
    // ... 原有代码
```

**Step 3: 修改 triggerAutoAssociate (行 3524)**

```go
for _, provider := range allProviders {
    // 跳过被拉黑的供应商
    if provider.Blacklisted != nil && *provider.Blacklisted {
        continue
    }
    // ... 原有代码
```

---

### Task 5: 后端 - 修改 model_sync.go 跳过黑名单

**Files:**
- Modify: `service/model_sync.go`（autoAssociateModels 函数中的 for 循环）

**Step 1: 找到 autoAssociateModels 中的 provider 循环**

搜索 `for _, provider := range allProviders` 在 service/model_sync.go 中的位置，同样添加：

```go
for _, provider := range allProviders {
    // 跳过被拉黑的供应商
    if provider.Blacklisted != nil && *provider.Blacklisted {
        continue
    }
    // ... 原有代码
```

---

### Task 6: 前端 - 更新 API 类型与函数

**Files:**
- Modify: `webui/src/lib/api.ts`

**Step 1: 更新 Provider 接口（约第 5-15 行）**

```typescript
export interface Provider {
  ID: number;
  Name: string;
  Type: string;
  Config: string;
  Console: string;
  Proxy: string;
  ModelEndpoint?: boolean;
  ModelFilterEnabled?: boolean;
  AuthType?: string;
  blacklisted?: boolean; // 是否已拉黑
}
```

**Step 2: 在 autoAssociateModels 附近添加两个新 API 函数**

```typescript
export async function getProviderBlacklist(): Promise<{ blacklisted_ids: number[] }> {
  return apiRequest<{ blacklisted_ids: number[] }>('/providers/blacklist');
}

export async function updateProviderBlacklist(providerIds: number[]): Promise<{ updated: number }> {
  return apiRequest<{ updated: number }>('/providers/blacklist', {
    method: 'PUT',
    body: JSON.stringify({ provider_ids: providerIds }),
  });
}
```

---

### Task 7: 前端 - 拉黑管理对话框

**Files:**
- Modify: `webui/src/routes/model-providers.tsx`

**Step 1: 在 import 区域添加新 API 函数引用（约第 62-72 行）**

在现有 import 列表中添加 `getProviderBlacklist` 和 `updateProviderBlacklist`：

```typescript
import {
  // ...现有导入...
  getProviderBlacklist,
  updateProviderBlacklist,
} from "@/lib/api";
```

**Step 2: 在 state 区域（约第 190-228 行）添加新状态**

```typescript
const [blacklistDialogOpen, setBlacklistDialogOpen] = useState(false);
const [blacklistedIds, setBlacklistedIds] = useState<number[]>([]);
const [blacklistLoading, setBlacklistLoading] = useState(false);
const [blacklistSaving, setBlacklistSaving] = useState(false);
```

**Step 3: 添加加载黑名单的 useEffect**

```typescript
useEffect(() => {
  if (!blacklistDialogOpen) return;
  setBlacklistLoading(true);
  getProviderBlacklist()
    .then((data) => setBlacklistedIds(data.blacklisted_ids))
    .catch((err) => toast.error(`加载黑名单失败: ${err instanceof Error ? err.message : String(err)}`))
    .finally(() => setBlacklistLoading(false));
}, [blacklistDialogOpen]);
```

**Step 4: 添加保存黑名单的 handler**

```typescript
const handleSaveBlacklist = async () => {
  setBlacklistSaving(true);
  try {
    await updateProviderBlacklist(blacklistedIds);
    toast.success("黑名单已保存");
    setBlacklistDialogOpen(false);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    toast.error(`保存黑名单失败: ${message}`);
  } finally {
    setBlacklistSaving(false);
  }
};

const handleToggleBlacklist = (providerId: number, checked: boolean) => {
  setBlacklistedIds((prev) =>
    checked ? [...prev, providerId] : prev.filter((id) => id !== providerId)
  );
};
```

**Step 5: 在"一键关联"按钮旁边添加"拉黑管理"按钮（约第 1579-1585 行）**

在现有的"一键关联"按钮之前插入：

```tsx
<Button
  onClick={() => setBlacklistDialogOpen(true)}
  variant="outline"
  className="h-8 text-xs flex-1 sm:flex-initial"
>
  拉黑管理
</Button>
```

**Step 6: 在 templateEditorOpen Dialog 之前（约第 1695 行）添加黑名单 Dialog**

```tsx
<Dialog open={blacklistDialogOpen} onOpenChange={setBlacklistDialogOpen}>
  <DialogContent className="max-w-lg">
    <DialogHeader>
      <DialogTitle>拉黑管理</DialogTitle>
      <DialogDescription>
        被拉黑的供应商在一键关联和自动关联时会被跳过，不会关联其模型。
      </DialogDescription>
    </DialogHeader>

    {blacklistLoading ? (
      <div className="flex items-center justify-center py-8 gap-2 text-sm text-muted-foreground">
        <Spinner className="h-4 w-4" />
        加载中...
      </div>
    ) : (
      <div className="space-y-2">
        {providers.length === 0 ? (
          <div className="text-sm text-muted-foreground py-4 text-center">暂无供应商</div>
        ) : (
          <div className="max-h-80 overflow-auto rounded-md border divide-y">
            {providers.map((provider) => {
              const isBlacklisted = blacklistedIds.includes(provider.ID);
              return (
                <div
                  key={provider.ID}
                  className="flex items-center gap-3 px-4 py-3 hover:bg-muted/50 transition-colors"
                >
                  <Checkbox
                    id={`blacklist-provider-${provider.ID}`}
                    checked={isBlacklisted}
                    onCheckedChange={(checked) =>
                      handleToggleBlacklist(provider.ID, checked === true)
                    }
                    disabled={blacklistSaving}
                  />
                  <label
                    htmlFor={`blacklist-provider-${provider.ID}`}
                    className="flex-1 cursor-pointer select-none"
                  >
                    <span className="text-sm font-medium">{provider.Name}</span>
                    <span className="text-xs text-muted-foreground ml-2">({provider.Type})</span>
                  </label>
                  {isBlacklisted && (
                    <span className="text-xs text-destructive font-medium">已拉黑</span>
                  )}
                </div>
              );
            })}
          </div>
        )}
        <div className="text-xs text-muted-foreground">
          已选 {blacklistedIds.length} 个供应商加入黑名单
        </div>
      </div>
    )}

    <DialogFooter>
      <Button
        variant="outline"
        onClick={() => setBlacklistDialogOpen(false)}
        disabled={blacklistSaving}
      >
        取消
      </Button>
      <Button onClick={handleSaveBlacklist} disabled={blacklistLoading || blacklistSaving}>
        {blacklistSaving ? (
          <>
            <Spinner className="h-4 w-4 mr-2" />
            保存中...
          </>
        ) : (
          "保存"
        )}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

---

## 测试验证

1. 进入供应商管理，确认供应商列表正常显示
2. 打开"拉黑管理"对话框，勾选某个供应商，保存
3. 再次打开对话框，确认勾选状态保留
4. 执行"一键关联"，确认被拉黑供应商的模型不会被关联
5. 添加/更新提供商时（触发自动关联），确认被拉黑供应商不被关联

---

## 注意事项

- `Blacklisted` 字段使用 `*bool` 指针类型，GORM default:false，`nil` 视为未拉黑
- 更新黑名单采用"整体替换"策略，先清空再设置，确保数据一致性
- `UpdateProviderBlacklist` 中传空数组 `[]` 表示清空全部黑名单
- 前端 `providers` 变量已在 `model-providers.tsx` 中存在（由 `fetchProviders()` 填充），可直接使用
