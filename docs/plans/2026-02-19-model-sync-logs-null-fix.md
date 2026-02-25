# 模型同步日志空值崩溃修复实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 model-sync-logs 页面在 AddedModels/RemovedModels 为 null 时的崩溃问题

**Architecture:** 在前端添加空值合并运算符 (??) 进行防御性编程，确保 .map() 调用前数组不为 null

**Tech Stack:** React 19, TypeScript

---

## Task 1: 修复 AddedModels 空值崩溃

**Files:**
- Modify: `webui/src/routes/model-sync-logs.tsx:652`

**Step 1: 修改 AddedModels.map() 调用**

在第 652 行，将：
```tsx
{detailLog.AddedModels.map((model, idx) => (
```

改为：
```tsx
{(detailLog.AddedModels ?? []).map((model, idx) => (
```

**完整上下文（第 648-657 行）：**
```tsx
{/* 新增模型 */}
{detailLog && detailLog.AddedCount > 0 && (
  <div>
    <h3 className="font-semibold text-green-600 mb-2">新增模型 ({detailLog.AddedCount})</h3>
    <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
      {(detailLog.AddedModels ?? []).map((model, idx) => (
        <div key={idx} className="text-sm">{model}</div>
      ))}
    </div>
  </div>
)}
```

---

## Task 2: 修复 RemovedModels 空值崩溃

**Files:**
- Modify: `webui/src/routes/model-sync-logs.tsx:664`

**Step 1: 修改 RemovedModels.map() 调用**

在第 664 行，将：
```tsx
{detailLog.RemovedModels.map((model, idx) => (
```

改为：
```tsx
{(detailLog.RemovedModels ?? []).map((model, idx) => (
```

**完整上下文（第 660-669 行）：**
```tsx
{/* 删除模型 */}
{detailLog && detailLog.RemovedCount > 0 && (
  <div>
    <h3 className="font-semibold text-red-600 mb-2">删除模型 ({detailLog.RemovedCount})</h3>
    <div className="max-h-40 overflow-y-auto border rounded p-2 space-y-1">
      {(detailLog.RemovedModels ?? []).map((model, idx) => (
        <div key={idx} className="text-sm">{model}</div>
      ))}
    </div>
  </div>
)}
```

---

## Task 3: 验证修复

**Step 1: 启动前端开发服务器**

```bash
cd webui
pnpm dev
```

**Step 2: 手动测试场景**

1. 打开浏览器访问模型同步日志页面
2. 点击任意日志的"查看详情"按钮
3. 验证：
   - 页面不再崩溃
   - 当 AddedModels/RemovedModels 为 null 时，不显示对应区块或显示空列表
   - 当有数据时，正常显示模型列表

**Expected:** 无控制台错误，详情弹窗正常显示

---

## Task 4: 提交修复

**Step 1: 检查修改**

```bash
git diff webui/src/routes/model-sync-logs.tsx
```

**Expected:** 只有两处 .map() 调用添加了 `?? []`

**Step 2: 提交**

```bash
git add webui/src/routes/model-sync-logs.tsx
git commit -m "$(cat <<'EOF'
fix(webui): 修复模型同步日志详情页空值崩溃

在 AddedModels 和 RemovedModels 为 null 时，直接调用 .map() 会导致
"Cannot read properties of null (reading 'length')" 错误。

添加空值合并运算符 (??) 进行防御性检查，确保始终对数组调用 .map()。

修改位置：
- model-sync-logs.tsx:652 (AddedModels)
- model-sync-logs.tsx:664 (RemovedModels)
EOF
)"
```

---

## 验收标准

- [ ] AddedModels 为 null 时不崩溃
- [ ] RemovedModels 为 null 时不崩溃
- [ ] 有数据时正常显示列表
- [ ] 无控制台错误
- [ ] 代码已提交

---

## 注意事项

1. **不需要修改类型定义**：虽然实际可能返回 null，但保持类型为 `string[]` 可以避免在其他地方添加过多空值检查
2. **不需要后端修改**：前端防御性编程已足够处理此问题
3. **不需要单元测试**：这是简单的 UI 防御性修复，手动测试即可验证