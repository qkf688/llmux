# 移动"启用格式转换"设置到通用页面 - 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将"启用格式转换"设置项从"日志管理"页面移动到"通用设置"页面，使其位置更符合功能语义。

**Architecture:** 这是一个纯 UI 重构任务，将现有的设置项从一个组件移动到另一个组件，不涉及数据结构或业务逻辑的修改。

**Tech Stack:** React 19, TypeScript, Tailwind CSS, Radix UI

---

## Task 1: 从日志管理页面移除"启用格式转换"设置

**Files:**
- Modify: `webui/src/routes/settings/logs-settings.tsx:351-367`

**Step 1: 读取当前文件内容**

Run: Read tool on `webui/src/routes/settings/logs-settings.tsx`
Expected: 查看第 351-367 行的"启用格式转换"设置项代码

**Step 2: 删除"启用格式转换"设置项**

删除以下代码块（第 351-367 行）：

```tsx
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="enable-format-conversion" className="text-sm md:text-base font-medium">
                启用格式转换
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统允许在不同 API 格式间转换（如 OpenAI ↔ Anthropic）。
                <br />
                关闭后只能使用与提供商类型匹配的格式，可减少转换开销，提升性能。
              </p>
            </div>
            <Switch
              id="enable-format-conversion"
              checked={localSettings?.enable_format_conversion ?? true}
              onCheckedChange={(checked) => updateLocalSettings({ enable_format_conversion: checked })}
            />
          </div>
```

使用 Edit tool 删除这段代码。

**Step 3: 验证文件语法正确**

Run: `cd webui && pnpm run type-check`
Expected: 无 TypeScript 错误

**Step 4: Commit**

```bash
git add webui/src/routes/settings/logs-settings.tsx
git commit -m "refactor(settings): 从日志管理页面移除启用格式转换设置"
```

---

## Task 2: 在通用设置页面添加"格式转换" Card

**Files:**
- Modify: `webui/src/routes/settings/routing-settings.tsx:349` (在 "模型关联自动化" Card 之后添加)

**Step 1: 读取当前文件内容**

Run: Read tool on `webui/src/routes/settings/routing-settings.tsx`
Expected: 查看文件结构，找到最后一个 Card 的位置（"模型关联自动化" Card 结束于第 348 行）

**Step 2: 在文件末尾添加新的"格式转换" Card**

在第 348 行（"模型关联自动化" Card 的 `</Card>` 之后）添加以下代码：

```tsx

      <Card>
        <CardHeader>
          <CardTitle>格式转换</CardTitle>
          <CardDescription>
            配置 API 格式转换功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4 md:space-y-6">
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-0.5">
              <Label htmlFor="enable-format-conversion" className="text-sm md:text-base font-medium">
                启用格式转换
              </Label>
              <p className="text-xs md:text-sm text-muted-foreground">
                开启后，系统允许在不同 API 格式间转换（如 OpenAI ↔ Anthropic）。
                <br />
                关闭后只能使用与提供商类型匹配的格式，可减少转换开销，提升性能。
              </p>
            </div>
            <Switch
              id="enable-format-conversion"
              checked={localSettings?.enable_format_conversion ?? true}
              onCheckedChange={(checked) => {
                if (localSettings) {
                  setLocalSettings({ ...localSettings, enable_format_conversion: checked });
                  setHasChanges(true);
                }
              }}
            />
          </div>
        </CardContent>
      </Card>
```

使用 Edit tool 在正确位置插入代码。

**Step 3: 验证文件语法正确**

Run: `cd webui && pnpm run type-check`
Expected: 无 TypeScript 错误

**Step 4: Commit**

```bash
git add webui/src/routes/settings/routing-settings.tsx
git commit -m "feat(settings): 在通用设置页面添加格式转换设置"
```

---

## Task 3: 手动测试验证

**Files:**
- Test: 浏览器手动测试

**Step 1: 启动开发服务器**

Run: `cd webui && pnpm dev`
Expected: 开发服务器在 http://localhost:5173 启动

**Step 2: 测试"通用设置"页面**

1. 打开浏览器访问 http://localhost:5173/settings
2. 点击"通用"标签页
3. 验证：
   - 页面底部显示"格式转换" Card
   - Card 包含"启用格式转换"开关
   - 开关默认状态为开启
   - 说明文字显示正确

**Step 3: 测试"日志管理"页面**

1. 点击"日志"标签页
2. 验证：
   - "性能优化" Card 不再包含"启用格式转换"设置
   - 其他设置项（关闭性能追踪、关闭 Token 统计等）仍然存在

**Step 4: 测试功能正常工作**

1. 在"通用"标签页中：
   - 切换"启用格式转换"开关
   - 点击"保存"按钮
   - 验证显示"通用设置保存成功"提示
2. 刷新页面
3. 验证设置已保存（开关状态保持）

**Step 5: 测试响应式布局**

1. 调整浏览器窗口大小（桌面 → 平板 → 手机）
2. 验证：
   - 所有断点下布局正常
   - 文字大小适配正确
   - 间距合理

**Step 6: 记录测试结果**

如果所有测试通过，继续下一步。如果有问题，修复后重新测试。

---

## Task 4: 最终提交

**Step 1: 检查 git 状态**

Run: `git status`
Expected: 显示两个已提交的文件

**Step 2: 查看提交历史**

Run: `git log --oneline -3`
Expected: 显示两个新提交：
- refactor(settings): 从日志管理页面移除启用格式转换设置
- feat(settings): 在通用设置页面添加格式转换设置

**Step 3: 推送到远程（可选）**

如果需要推送到远程仓库：
```bash
git push origin dev
```

---

## 验收清单

完成后检查以下项目：

- [ ] "日志管理"页面不再显示"启用格式转换"设置
- [ ] "通用设置"页面显示独立的"格式转换" Card
- [ ] 设置项的功能保持不变（可以正常切换和保存）
- [ ] 保存和重置功能正常工作
- [ ] 响应式布局在所有断点下正常显示
- [ ] TypeScript 类型检查通过
- [ ] 代码已提交到 git

---

## 回滚方案

如果需要回滚更改：

```bash
# 回滚到修改前的状态
git reset --hard HEAD~2

# 或者使用 revert
git revert HEAD~1
git revert HEAD
```

---

## 注意事项

1. 这是一个纯 UI 重构，不涉及后端修改
2. 不需要修改 API 接口或数据结构
3. 不需要数据库迁移
4. 修改后立即生效，无需重启服务
5. 建议在非高峰时段进行，避免影响用户使用
