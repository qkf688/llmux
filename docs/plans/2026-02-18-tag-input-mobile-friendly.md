# TagInput 移动端友好改造实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 改造 TagInput 组件为移动端友好的输入框+按钮+列表布局

**Architecture:** 将原有的内嵌式标签输入改为分离式布局：上方输入框+添加按钮，下方独立列表展示已添加内容

**Tech Stack:** React 19, TypeScript, Tailwind CSS, Lucide Icons

---

## Task 1: 重构 TagInput 组件布局

**Files:**
- Modify: `webui/src/components/ui/tag-input.tsx`

**Step 1: 备份当前实现**

创建备份注释，保留原有逻辑作为参考。

**Step 2: 重构组件结构**

将组件分为两个区域：
1. 输入区域（Input + Button）
2. 列表区域（已添加内容展示）

```tsx
import * as React from "react"
import { X, Plus } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

export interface TagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
  className?: string
}

export function TagInput({ value, onChange, placeholder, className }: TagInputProps) {
  const [inputValue, setInputValue] = React.useState("")

  const handleAdd = () => {
    const trimmed = inputValue.trim()
    if (trimmed && !value.includes(trimmed)) {
      onChange([...value, trimmed])
    }
    setInputValue("")
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault()
      handleAdd()
    }
  }

  const removeTag = (tagToRemove: string) => {
    onChange(value.filter(tag => tag !== tagToRemove))
  }

  return (
    <div className={cn("space-y-2", className)}>
      {/* 输入区域 */}
      <div className="flex gap-2">
        <Input
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="flex-1"
        />
        <Button
          type="button"
          onClick={handleAdd}
          size="default"
          className="shrink-0"
        >
          <Plus className="h-4 w-4 mr-1" />
          添加
        </Button>
      </div>

      {/* 列表区域 */}
      <div className="min-h-[100px] rounded-md border border-input bg-transparent p-3">
        {value.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            暂无内容，请在上方输入添加
          </p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {value.map((tag) => (
              <div
                key={tag}
                className="flex items-center gap-1.5 rounded-md bg-secondary px-2.5 py-1 text-sm"
              >
                <span>{tag}</span>
                <button
                  type="button"
                  onClick={() => removeTag(tag)}
                  className="rounded-sm text-muted-foreground hover:text-destructive focus:outline-none focus:ring-1 focus:ring-ring transition-colors"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
```

**Step 3: 验证组件接口兼容性**

确认：
- `value: string[]` - 保持不变 ✓
- `onChange: (tags: string[]) => void` - 保持不变 ✓
- `placeholder?: string` - 保持不变 ✓
- `className?: string` - 保持不变 ✓

调用方（`routing-settings.tsx`）无需修改。

**Step 4: 手动测试验证**

启动开发服务器：
```bash
cd webui && pnpm dev
```

测试场景：
1. 打开 `/settings` 页面
2. 找到"模板模糊匹配"部分的"分隔符"和"后缀关键词"
3. 测试添加功能：
   - 输入 `:` 点击"添加"按钮 → 应该出现在列表中
   - 输入 `-` 按 Enter 键 → 应该出现在列表中
   - 输入重复内容 `:` 点击添加 → 应该静默跳过，输入框清空
4. 测试删除功能：
   - 点击列表项右侧的 × 按钮 → 该项应该被移除
5. 测试空状态：
   - 删除所有项 → 应该显示"暂无内容，请在上方输入添加"
6. 测试保存：
   - 添加几个分隔符和后缀关键词
   - 点击"保存"按钮 → 应该成功保存
   - 刷新页面 → 数据应该保持

**Step 5: 移动端测试**

使用浏览器开发者工具切换到移动设备视图：
1. iPhone SE (375px)
2. iPad Mini (768px)

验证：
- 输入框和按钮布局正常
- 列表项换行正常
- 触摸点击区域足够大
- 虚拟键盘弹出时布局不错乱

**Step 6: 提交更改**

```bash
git add webui/src/components/ui/tag-input.tsx
git commit -m "refactor(ui): redesign TagInput for mobile-friendly interaction

- Split layout: input+button above, list below
- Add explicit 'Add' button for mobile compatibility
- Keep Enter key support for desktop users
- Show empty state hint when list is empty
- Maintain same component interface (no breaking changes)"
```

---

## 验收标准

- [ ] 输入框和"添加"按钮在同一行
- [ ] 已添加内容在下方独立区域展示
- [ ] 点击"添加"按钮可以添加内容
- [ ] 按 Enter 键也可以添加内容（保留快捷键）
- [ ] 重复内容会被静默跳过，输入框自动清空
- [ ] 列表项右侧有 × 按钮可以删除
- [ ] 列表为空时显示提示文字
- [ ] 移动端虚拟键盘可以正常输入和添加
- [ ] 原有调用方（routing-settings.tsx）无需修改
- [ ] 保存功能正常工作

---

## 注意事项

1. **不要修改组件接口**：保持 `TagInputProps` 不变，确保向后兼容
2. **响应式设计**：使用 Tailwind 的响应式类确保移动端和桌面端都正常
3. **无障碍性**：确保按钮有合适的 `type="button"` 避免表单提交
4. **用户体验**：输入框清空逻辑要符合预期（添加后清空，重复时也清空）
