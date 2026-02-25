# 移动端筛选面板折叠优化方案

## 问题描述
在移动端"模型提供商关联"页面，筛选框和操作按钮占据了约2/3的页面高度，导致实际内容区域过小，用户体验不佳。

## 解决方案
将筛选区域改为可折叠的面板，默认在移动端收起，点击展开/收起。

## 实现计划

### 1. 添加筛选面板折叠状态管理
```typescript
// 在组件顶部添加状态
const [filterPanelOpen, setFilterPanelOpen] = useState(false);
```

### 2. 创建筛选面板切换按钮
在筛选区域上方添加一个切换按钮，显示：
- 筛选图标
- 当前激活的筛选条件数量
- 展开/收起状态指示

```tsx
<div className="flex items-center justify-between mb-2">
  <Button
    variant="outline"
    size="sm"
    onClick={() => setFilterPanelOpen(!filterPanelOpen)}
    className="w-full sm:w-auto justify-between"
  >
    <span className="flex items-center gap-2">
      <Filter className="h-4 w-4" />
      筛选
      {activeFilterCount > 0 && (
        <span className="bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded-full">
          {activeFilterCount}
        </span>
      )}
    </span>
    {filterPanelOpen ? (
      <ChevronUp className="h-4 w-4" />
    ) : (
      <ChevronDown className="h-4 w-4" />
    )}
  </Button>
</div>
```

### 3. 计算激活的筛选条件数量
```typescript
const activeFilterCount = [
  selectedProviderType !== 'all',
  selectedProviderFilter !== 'all',
  selectedStatusFilter !== 'all',
  searchKeyword.trim() !== ''
].filter(Boolean).length;
```

### 4. 将筛选区域包裹在可折叠容器中
使用条件渲染和过渡动画：

```tsx
<div className="flex flex-col gap-2 flex-shrink-0">
  {/* 筛选面板切换按钮 */}
  <div className="flex items-center justify-between">
    <Button
      variant="outline"
      size="sm"
      onClick={() => setFilterPanelOpen(!filterPanelOpen)}
      className="w-full sm:w-auto justify-between"
    >
      <span className="flex items-center gap-2">
        <Filter className="h-4 w-4" />
        筛选
        {activeFilterCount > 0 && (
          <span className="bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded-full">
            {activeFilterCount}
          </span>
        )}
      </span>
      {filterPanelOpen ? (
        <ChevronUp className="h-4 w-4" />
      ) : (
        <ChevronDown className="h-4 w-4" />
      )}
    </Button>
  </div>

  {/* 筛选面板内容 */}
  <div
    className={cn(
      "overflow-hidden transition-all duration-300 ease-in-out",
      filterPanelOpen ? "max-h-[500px] opacity-100" : "max-h-0 opacity-0"
    )}
  >
    <div className="flex flex-col gap-2">
      {/* 第一行：模型选择 + 提供商类型筛选 + 具体提供商筛选 */}
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4 lg:gap-4">
        {/* ... 现有的筛选器 ... */}
      </div>

      {/* 第二行：搜索框 + 操作按钮 */}
      <div className="flex flex-col sm:flex-row gap-2">
        {/* ... 现有的搜索框和操作按钮 ... */}
      </div>
    </div>
  </div>
</div>
```

### 5. 响应式默认状态
- 移动端（< sm 断点）：默认收起
- 桌面端（>= sm 断点）：默认展开

```typescript
const [filterPanelOpen, setFilterPanelOpen] = useState(() => {
  // 桌面端默认展开，移动端默认收起
  return window.innerWidth >= 640;
});
```

### 6. 添加激活筛选条件的视觉提示
当有筛选条件激活时，在切换按钮上显示徽章，提示用户当前有筛选条件。

### 7. 优化移动端操作按钮布局
在移动端，将操作按钮改为更紧凑的布局：
- 使用图标按钮
- 减少按钮间距
- 使用更小的按钮尺寸

## 修改文件
- `webui/src/routes/model-providers.tsx`

## 需要导入的图标
```typescript
import { Filter, ChevronUp, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";
```

## 预期效果
1. 移动端默认只显示筛选切换按钮，节省约60%的垂直空间
2. 用户点击按钮可展开筛选面板
3. 有激活筛选条件时，按钮上显示计数徽章
4. 桌面端保持原有展开状态，不影响桌面用户体验
5. 平滑的展开/收起动画效果

## 测试要点
1. 移动端默认状态是否收起
2. 点击切换按钮是否能正确展开/收起
3. 激活筛选条件时徽章是否正确显示
4. 桌面端是否保持展开状态
5. 动画效果是否流畅
6. 筛选功能是否正常工作
