# webui 模块

> 本文件描述管理后台前端 SPA。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 配置与运维可视化（不实现服务端业务规则） |
| 目录位置 | `webui/`（源码 `webui/src/`；构建 `webui/dist/` 由 Go embed） |
| 主要职责 | 供应商（含号池/凭据/协议端点/分组）、模型、关联、虚拟模型、日志、指标、设置、健康与同步日志、数据库管理界面 |

## 2. 职责与边界

- **负责什么**：登录鉴权态 UI；路由与布局；表单与列表；调用 `/api`；服务端状态（TanStack Query）与客户端状态（Zustand）；图表与主题
- **不负责什么**：任何后端业务规则、选路、协议转换、持久化
- **对外暴露**：浏览器 SPA 路径（见 `route-config.ts`）；构建产物供 `main` embed
- **依赖谁**：后端 `/api`（开发态 Vite 代理到 `7070`）

## 3. 内部结构

```
webui/src/
├── main.tsx / App.tsx
├── routes/                 # 页面 + route-config.ts + layout
│   ├── providers|models|virtual-models|model-providers|...
│   └── settings/
├── components/             # ui/、charts/、page-header.tsx、page-toolbar.tsx、table-card.tsx、stat-card.tsx、共享组件
├── lib/animations/         # 动效 token 单一来源（EASING / NUMBER_ANIMATION_MS）
├── lib/api/                # core client + modules（catalog/logs/system）
├── hooks/                  # 含 hooks/api 查询封装
├── stores/                 # Zustand（含 core/updater）
└── test/
```

页面文件夹常见形态：

```
routes/<page>/
├── index.tsx
├── hooks/use-*-page.ts
├── components/sections|dialogs/
├── components/shared/          # 页面级共享 UI（跨 dialog/section 复用）
├── schemas/ / types/ / utils/
```

各子目录职责：
- `routes/`：按业务页拆分；`route-config.ts` 为 path/layout/nav 单一数据源
- `lib/animations/`：动效 token 单一来源（缓动曲线/动画时长）；新增动效优先引用，禁止组件散落魔法缓动值
- `lib/api/`：HTTP 客户端与领域 API 函数
- `stores/`：客户端状态；`Updater` / `Setter` 统一更新模式
- `components/ui/`：Radix + Tailwind 基础组件
- `components/ui/dialog.tsx`：全站 dialog 尺寸配方单一数据源（`dialogContentVariants` 六档 `sm`/`md`/`lg`/`xl`/`menu`/`sheet`，宽度一律带 `sm:` 前缀；`lg`/`xl` 固定高度、其余只给 `max-h` 上限——按「内容量在弹窗生命周期内是否会变」分而非按档位一刀切）+ `DialogBody`（Header/Footer 之间唯一的滚动容器）。29 处 `DialogContent` 全部只声明 `size`，页面侧无尺寸类；`alert-dialog.tsx` 的 `AlertDialogContent` 同为确认框单一数据源（固定 `sm:max-w-lg` + `max-h-[85dvh]`，无 body 抽象、整框滚动）
- `components/page-header.tsx`：全站页头单一数据源（图标 + 标题/副标题 + 右侧动作，卡片外壳 `rounded-xl border bg-card shadow-sm`，标题统一 `h2 text-xl`）；各页页头**必须**复用，禁止再手写裸标题
- `components/page-toolbar.tsx`：页头之下、列表之上那条工具栏的卡片外壳单一数据源（`rounded-xl border bg-card px-2.5 py-2 shadow-sm`，内距比 page-header 小）；配套导出 `ToolbarFilter`（Label+Select 筛选单元）与 `ToolbarSearch`（带放大镜的搜索框）。各页筛选/操作横条**必须**复用，禁止再手写卡片配方或逐页复制筛选下拉 JSX；布局用 flex-wrap 而非 grid 列数（增减筛选项零改动）
- `components/table-card.tsx`：列表/表格卡片外壳单一数据源（同一套 `rounded-xl border bg-card shadow-sm` 配方 + 可选 `footer`，传入时自动补 `border-t` 把分页器收进卡内）；各页列表容器**必须**复用，禁止再手写卡片配方或把分页器裸放卡外
- `components/stat-card.tsx`：单指标统计卡单一数据源（`StatCard` = 语义色圆点 + 指标名 + 大号数值 + 可选单位/副说明，卡壳 `rounded-xl border bg-card px-3.5 py-3 shadow-sm`；`StatGrid` = 窄屏 2 列 / 宽屏 4 列网格）。各页统计条**必须**复用，禁止再手写 `rounded-lg border bg-card p-4` 之类的小卡配方；语义色只落在圆点上、数值一律前景色。**不覆盖** home 的分组多指标卡（`TopMetricCard`/`MetricItem`，竖排标题栏 + 多行指标），那是另一种形态，避免为它开一堆可选 props 退化成万能卡
- `components/shared/`：页面级共享展示组件（如 providers 的批量测试 UI）；不上提到 `components/ui/`（YAGNI，仅模块内复用）

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `appRoutes` / `route-config` | 路由与侧栏配置 | `webui/src/routes/route-config.ts` | `App.tsx` / `layout.tsx` 消费 |
| `fetchWithAuth` / `apiRequest` | 带鉴权信封请求 | `webui/src/lib/api/core/client.ts` | 各 api modules |
| API modules | 按域封装 REST | `webui/src/lib/api/modules/**` | 页面 hooks 调用 |
| `Updater` / `Setter` | store 统一更新接口 | `webui/src/stores/core/updater.ts` | 各 store / hooks / 组件 props |

## 5. 特殊约定

- 新增页面：新目录 + `route-config.ts` 追加一项；`App`/`layout` 自动派生
- API 信封：`{ code, message, data }`，`code !== 200` 抛错；401 跳转 `/login`
- 路径别名：`@` → `src/`
- 修改前端后需重新 `pnpm run build`（或 `make webui`）才能进入 embed 二进制
- **边界现状**：页面以 API 编排为主；仍可能复制服务端默认策略（如关联表单 `weight`/`priority` 硬编码后再被 settings 覆盖）——与「不实现业务规则」的目标尚有缺口
- **DRY 现状**：日志类分页 UI、部分 Provider 视图类型在 routes/stores 间存在重复定义

### 页面 ↔ 后端域映射

| 前端路由 | 后端域 |
|----------|--------|
| `/providers` | providers |
| `/models` | models-catalog |
| `/model-providers` | associations |
| `/virtual-models` | virtual-models |
| `/number-pools` | pools |
| `/logs`, `/logs/:id/chat-io` | logs-metrics |
| `/health-check-logs` | health-check |
| `/model-sync-logs` | model-sync |
| `/settings` | settings |
| `/database` | system-ops（database/importexport） |
| `/` home | logs-metrics metrics |

### thinking 能力开关（models / model-providers 页）

- `/models` 编辑弹窗：`支持 thinking` Checkbox ↔ `Model.SupportsThinking`（`supports_thinking` 字段，二态 bool，默认 false）
- `/model-providers` 关联弹窗：`支持 thinking` 三态 Select（继承/启用/禁用）↔ `ModelWithProvider.SupportsThinking`（`supports_thinking` 字段，`*bool`：不传=继承、true/false=override）；"继承"选项实时显示当前所选 model 的支持状态（弹窗持有 `models` prop 查询）

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
