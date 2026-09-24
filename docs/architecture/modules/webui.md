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
├── components/             # ui/、charts/、page-header.tsx、page-toolbar.tsx、table-card.tsx、stat-card.tsx
│   └── number-pools/       # 号池域跨路由共享（pool-detail-dialog / credential-key-cell / use-credential-key-reveal / credential-status / 删除确认弹窗）
├── lib/animations/         # 动效 token 单一来源（EASING / NUMBER_ANIMATION_MS）
├── lib/api/                # core client + modules（auth/catalog/logs/system；auth 走全路径 import 不进 index 桶）
├── lib/clipboard.ts        # 剪贴板写入跨域单源（三级降级 + 普通/机密两个提示入口）
├── lib/concurrent-batch.ts # 批量并发调度跨域单源（按项回 成功/失败/已取消 三态；providers 两套 + model-providers 一处批量共用）
├── lib/header-rows.ts      # 出站自定义头「行 ↔ Record」折叠与预设互斥单源（api-keys 与关联两弹窗共用，4a；语法权威在服务端 common/outboundheaders.go）
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
- `lib/api/`：HTTP 客户端与领域 API 函数。测试类 API（`testModelProvider*` / `testProviderModel`）接受**可选** `signal`，调用方从 `AbortController` 透传——取消要真正生效必须一路传到 `fetch`，只在前端把 loading 置否不会停止上游会话计费
- `lib/concurrent-batch.ts`：批量并发调度**跨域单一来源**（providers 页两套批量 + model-providers 页批量）。`runItem` 回 `"success" | "failed" | "cancelled"` 三态并被单独计数。两条硬约束：① **调用方必须把 signal 透传到真正的请求上**——只判 `signal.aborted` 就返回会让「取消」停在调度层（不再派新项，但在飞的那项照跑完并计费）；② 中止异常用 `lib/errors.isAbortError` 判为 `"cancelled"`，与 `"failed"` 分开计——取消不证明供应商不可用，混进失败会污染「选择失败项」这类下游动作
- `lib/clipboard.ts`：剪贴板写入**跨域单一来源**（providers 模型名、号池凭据 key、供应商内联 key 三处共用）。降级链 = 安全上下文走 Clipboard API → 非安全上下文（`http://<ip>:7070`）直接走同步 `execCommand`（保住点击手势有效期）→ 再失败弹手动复制 prompt，逐级回传 `method`；`copyTextDetailed` 是唯一的结构化入口（不依赖 toast，非 UI 场景复用它）。两个带提示的入口按内容敏感度二选一：普通文本用 `copyTextWithToast`（保留 prompt 兜底），**明文 key 等机密必须用 `copySecretWithToast`**（禁 prompt——`window.prompt` 的默认值参数会把机密整条弹上屏：模态、可被肩窥/录屏捕获、且不受前端任何生命周期守卫管辖）。成功提示只传掩码或行号：机密不得写进任何用户可见文案（toast / prompt / `title` / `aria-label`）。任何新的复制入口都必须走此处，禁止在页面里直接调 `navigator.clipboard`
- `stores/`：客户端状态；`Updater` / `Setter` 统一更新模式。`stores/auth/login-expiry-notice.ts` 是「登录已过期」一次性提示的**载体单一来源**（sessionStorage 单键 `loginExpiryNotice`，值 = 写入时间戳，兼任「失效处置已触发」的去重标记与「过期提示待展示」标记：`markLoginExpiredOnce` 在 10s 去重窗口内仅首次返回 true、`hasLoginExpiryNotice` 非破坏性读取且 60s 后视为陈旧、`consumeLoginExpiryNotice` 读后即焚；sessionStorage 写失败退内存兜底）。展示文案渲染侧取常量 `LOGIN_EXPIRY_NOTICE_TEXT`（不取存储值，结构性保证不携带内部错误文本），其余处禁止直读写该键
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
| 统一拆包通道（`apiRequest` / `apiRequestAnonymous` / `apiRequestForm` / `apiRequestFile`） | `/api` 请求的唯一出口：取信封 `data`、把失败翻成可展示 Error、401 处置 | `webui/src/lib/api/core/client.ts` | 各 api modules |
| API modules | 按域封装 REST | `webui/src/lib/api/modules/**` | 页面 hooks 调用 |
| `Updater` / `Setter` | store 统一更新接口 | `webui/src/stores/core/updater.ts` | 各 store / hooks / 组件 props |

## 5. 特殊约定

- 新增页面：新目录 + `route-config.ts` 追加一项；`App`/`layout` 自动派生
- **API 信封与失败文案（Stage 8 起，单一通道）**：响应信封为 `{ code, message, data }`——`data`/`error` 带 `omitempty`，**失败信封不含 `data` 键**、`error` 键仅 500 有（前端按可选读，勿假设键恒在）。一个请求「成不成功」看两个信号——真实 HTTP 状态码与信封 `code`；两者都由 `client.ts` 读取，**页面层不得自己解响应**（此前 `createApiKey`/`importConfig`/两处导出各写一份手写解包，已全部收编）。
  - `client.ts` 是 `/api` **信封型**请求的唯一出口（例外：`/api/test/react/:id` 是 SSE，由 `fetchEventSource` 消费，不经信封）。
  - 失败一律抛 `Error(可展示文案)`：优先信封 `message`，其次 `error`，再退中文兜底（真实非 2xx 为 `请求失败（错误码 <status>）`，2xx 但信封 `code≠200` 为 `请求失败（响应码 <code>）`）；body 解析不出对象（非 JSON / 字面量 `null`）时给固定人话，不透传 `SyntaxError`/`TypeError` 原文。500 额外把响应头 `X-Request-Id` 的报障编号补进文案（信封已内嵌同号则不重复），其余失败不带编号。
  - **401 处置**：`apiRequest` 收到 401 视为登录过期 → 清会话 + 跳 `/login` + 抛 `Unauthorized`。凭据本身无效的接口用 `apiRequestAnonymous`（当前仅登录：密码错的 401 要留在页面上显示，不能把刚写的提示刷掉）；判断「密码/旧密码填错」类**不给后端 401**（改密码已改判 400，见 `handler/auth/password.go`），故不需要前端接口名单。**「跳 /login」单一 owner（Stage 22）**：`redirectToLogin()`（清会话 + 整页跳转，不写提示）是唯一跳转 owner，`main.tsx` 启动守卫等处禁止再手写 `window.location.href = '/login'`。401 失效出口 `handleUnauthorized` = 标记过期（`stores/auth/login-expiry-notice.ts`，兼作去重）→ `redirectToLogin` → 抛 `Unauthorized`：同一去重窗口内并发多请求撞 401 只有首个调用方触发整套处置（已设标记不再跳/不再写）；登录页挂载展示一次性提示（非破坏性读取 + effect 清除），文案取静态常量 `LOGIN_EXPIRY_NOTICE_TEXT`。**未登录直入不写提示**——`requireAuthTokenOrRedirect`（发起时无凭据）与 `main.tsx` 启动守卫直接 `redirectToLogin`（未登录 ≠ 登录过期，首访用户不该看到过期文案）。
  - 文件下载走 `apiRequestFile`：成功路径必须设 `Content-Disposition`（判据），否则被当成失败信封；`apiRequestForm` 用于 FormData（不注入 `Content-Type`，保住 multipart 边界）。
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
| `/api-keys` | api-keys |
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
