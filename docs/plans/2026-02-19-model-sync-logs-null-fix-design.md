# 模型同步日志空值崩溃修复设计

## 问题描述

`model-sync-logs.tsx` 页面在查看同步详情时崩溃，错误信息：
```
Uncaught TypeError: Cannot read properties of null (reading 'length')
```

**根本原因**：后端 `ModelSyncLog` 的 `AddedModels` 和 `RemovedModels` 字段在数据库中为 null 时，前端直接调用 `.map()` 导致崩溃。

## 修复方案

仅修改前端，添加防御性空值检查。

### 代码修改

**文件**：`webui/src/routes/model-sync-logs.tsx`

**位置**：第 652-654 行 和 第 664-666 行

**修改前**：
```tsx
{detailLog.AddedModels.map((model, idx) => (
  <div key={idx} className="text-sm">{model}</div>
))}
```

**修改后**：
```tsx
{(detailLog.AddedModels ?? []).map((model, idx) => (
  <div key={idx} className="text-sm">{model}</div>
))}
```

同理处理 `RemovedModels`。

## 验收标准

1. 当 `AddedModels` 或 `RemovedModels` 为 null 时，页面不再崩溃
2. 空数组时正常渲染（不显示列表）
3. 有数据时正常渲染列表项

## 影响范围

- 仅影响前端 `model-sync-logs.tsx` 文件
- 不影响后端、数据库或其他页面