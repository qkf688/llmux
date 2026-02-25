# 移动"启用格式转换"设置到通用页面

## 概述

将"启用格式转换"设置项从"日志管理"页面移动到"通用设置"页面，使其位置更符合功能语义。

## 背景

当前"启用格式转换"设置位于"日志管理"页面的"性能优化" Card 中，但该功能实际上是系统核心路由功能的一部分，与日志记录关系不大。将其移动到"通用设置"页面更符合用户预期。

## 设计方案

### 1. 从日志管理页面移除

**文件**: `webui/src/routes/settings/logs-settings.tsx`

**操作**: 删除第 351-367 行的"启用格式转换"设置项

**保留内容**: "性能优化" Card 的其他设置项：
- 关闭性能追踪
- 关闭 Token 统计
- 启用请求追踪
- 移除不必要的响应头

### 2. 添加到通用设置页面

**文件**: `webui/src/routes/settings/routing-settings.tsx`

**位置**: 在现有 Card 列表末尾（"模型关联自动化" Card 之后）

**新增 Card 结构**:
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

### 3. 影响范围

**修改文件**:
- `webui/src/routes/settings/logs-settings.tsx` - 删除代码
- `webui/src/routes/settings/routing-settings.tsx` - 添加代码

**不影响**:
- 数据结构（`Settings` 类型定义）
- API 接口
- 其他组件
- 后端逻辑

## 实施步骤

1. 从 `logs-settings.tsx` 删除"启用格式转换"设置项（行 351-367）
2. 在 `routing-settings.tsx` 末尾添加新的"格式转换" Card
3. 测试验证：
   - 确认"通用设置"页面显示新 Card
   - 确认"日志管理"页面不再显示该设置
   - 确认开关功能正常工作
   - 确认保存功能正常

## 验收标准

- [ ] "日志管理"页面不再显示"启用格式转换"设置
- [ ] "通用设置"页面显示独立的"格式转换" Card
- [ ] 设置项的功能保持不变
- [ ] 保存和重置功能正常工作
- [ ] 响应式布局正常显示

## 风险评估

**风险**: 无

**理由**:
- 仅移动 UI 组件位置
- 不修改数据结构和业务逻辑
- 不影响现有功能

## 时间估算

- 代码修改: 5 分钟
- 测试验证: 5 分钟
- 总计: 10 分钟
