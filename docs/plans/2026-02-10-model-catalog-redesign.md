# 模型目录页面优化设计

## 问题分析

当模型数量较多时（50+），当前模型目录页面存在以下问题：

1. **分组逻辑粗糙** — 按第一个 `-` 前的前缀分组，导致 `gpt-4o` 和 `gpt-3.5-turbo` 混在同一个 "gpt" 组，`o3`/`o4-mini` 等无 `-` 模型归入"其他"
2. **信息密度低** — 每个模型占 ~52px 高度（名称一行 + provider 标签一行 + padding），100 个模型需要大量滚动
3. **筛选维度单一** — 只有文本搜索，无法按提供商维度快速过滤
4. **视觉层次扁平** — 所有模型外观一致，没有重点突出

## 设计方案：智能分组 + 紧凑列表 + 多维筛选

### 1. 智能分组逻辑

替换当前 `indexOf("-")` 的粗暴分组，改为**规则匹配 + 前缀回退**：

```typescript
const MODEL_FAMILIES: [string, RegExp][] = [
  ["GPT-4o",     /^gpt-4o/i],
  ["GPT-4.1",    /^gpt-4\.1/i],
  ["GPT-4",      /^gpt-4(?!o|\.1)/i],
  ["GPT-3.5",    /^gpt-3\.5/i],
  ["o 系列",     /^o[1-9]/i],
  ["Claude",     /^claude/i],
  ["Gemini",     /^gemini/i],
  ["DeepSeek",   /^deepseek/i],
  ["Qwen",       /^qwen/i],
  ["GLM",        /^glm/i],
  ["Llama",      /^llama/i],
  ["Mistral",    /^mistral/i],
  ["Yi",         /^yi-/i],
  ["Moonshot",   /^moonshot/i],
  ["ERNIE",      /^ernie/i],
];

function getModelFamily(modelName: string): string {
  for (const [family, regex] of MODEL_FAMILIES) {
    if (regex.test(modelName)) return family;
  }
  // 回退：取第一个 `-` 前的前缀并首字母大写
  const dashIndex = modelName.indexOf("-");
  if (dashIndex > 0) {
    const prefix = modelName.substring(0, dashIndex);
    return prefix.charAt(0).toUpperCase() + prefix.slice(1);
  }
  return "其他";
}
```

**效果：**
- `o3-mini` → "o 系列"（而非"其他"）
- `gpt-4o-mini` → "GPT-4o"（而非和 gpt-3.5 混在一起）
- `claude-sonnet-4-5-20250514` → "Claude"
- 未知模型仍按首段前缀分组

### 2. 紧凑列表视图

将每行高度从 ~52px 压缩到 ~36px：

**改动要点：**
- Provider 标签从独立行移到模型名同一行右侧
- 操作按钮（复制/添加）默认隐藏，hover 时显示
- 减少 padding（py-3 → py-1.5）
- 移除桌面/移动端双套布局，用 responsive 统一处理

**布局示意：**
```
│ gpt-4o              [OpenAI] [Azure]        [复制] [+添加] │
│ gpt-4o-mini         [OpenAI] [Azure]        [复制] [+添加] │
│ gpt-4o-2024-08-06   [OpenAI]                [复制] [+添加] │
```

**移动端：**
- Provider 标签换行显示
- 操作按钮始终可见（触屏没有 hover）

### 3. 提供商筛选 Chip 栏

在搜索框下方添加一行提供商筛选 chip：

```
提供商: [全部] [OpenAI] [Anthropic] [Google] [DeepSeek] [Azure]
```

- 从 providers 数据动态生成
- 单选切换，默认"全部"
- 选中后只显示该提供商下的模型
- 每个 chip 显示该提供商的模型数量：`OpenAI (45)`

### 4. 辅助改进

- **全部折叠/展开按钮** — 放在统计信息旁边
- **已添加状态持久化** — 页面加载时查询已有模型列表，自动标记已添加的模型
- **分类内模型计数** — 筛选后动态更新每个分类的数量
- **排序选项** — 按名称（默认）/ 按提供商数量（多的优先）

### 5. 批量添加

- 每个分类标题旁增加「全部添加」按钮
- 勾选框 + 底部浮动操作栏（选中 N 个模型后出现）

## 不做的事情

- **网格/卡片视图** — 对于工具型列表页面，紧凑列表更高效
- **虚拟滚动** — 模型数量通常在几百以内，原生 DOM 足够
- **树形三级结构** — 过度设计，两级（系列→模型）已足够

## 实施优先级

1. **P0 — 智能分组逻辑** — 改动最小，效果最明显
2. **P0 — 紧凑列表** — 信息密度翻倍
3. **P1 — 提供商筛选** — 多维度过滤
4. **P1 — 全部折叠/展开**
5. **P2 — 批量添加**
6. **P2 — 已添加状态持久化**

## 技术实现要点

- 所有改动集中在 `webui/src/routes/model-catalog.tsx` 一个文件
- 智能分组函数可抽到 `webui/src/lib/model-families.ts`
- 无需新增依赖
- 无后端改动
