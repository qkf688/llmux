# AGENTS.md — LLMux 项目协作约定

> 本文件是 AI 协作与人类协作者的项目入口。修改代码或目录结构前，先读本文档与索引中的架构文档。

## 1. 文档索引

| 文档 | 路径 | 用途 |
|---|---|---|
| 架构总览 | [docs/architecture/README.md](docs/architecture/README.md) | 项目概述、设计原则落地、依赖方向、模块索引（现状描述） |
| 模块详情 | [docs/architecture/modules/](docs/architecture/modules/) | 各业务模块的职责、边界、接口契约（现状描述） |
| 模块间交互 | [docs/architecture/interactions.md](docs/architecture/interactions.md) | 同步调用 / Hook / 注册表 / 共享模型（现状描述） |
| 命名规范 | [docs/architecture/conventions.md](docs/architecture/conventions.md) | 文件 / 目录 / 变量命名约定（现状描述） |
| 目录树 | [docs/architecture/dir-tree.txt](docs/architecture/dir-tree.txt) | 项目目录结构（脚本自动生成） |
| 虚拟模型用户指南 | [docs/virtual-models-guide.md](docs/virtual-models-guide.md) | 虚拟模型使用说明（用户向，非架构） |

> AGENTS.md 是操作规范（「该怎么改」）；架构文档是现状描述（「系统长什么样」）。两者互补，不重复。

### ⚠️ 文档同步义务（重要）

下列改动**必须同步更新对应文档**，否则视为未完成：

| 改动类型 | 需更新的文档 |
|---|---|
| 新增/删除/重命名目录、文件 | 架构文档（重跑目录树脚本更新 `dir-tree.txt`）+ 涉及的 `modules/{x}.md` 内部结构 |
| 新增/删除/重命名业务模块 | `docs/architecture/README.md` 模块索引 + 新建/删除 `modules/{x}.md` + 本文件第 2.1 节任务导航（如影响） |
| 修改核心接口契约 | `modules/{x}.md` 的「关键接口契约」节 |
| 修改模块职责边界 | `modules/{x}.md` 的「职责与边界」节 |
| 修改顶层分层或依赖方向 | `README.md` 的「全局架构图」节 + 本文件第 4 节 |
| 修改扩展最小改动集 | `README.md`「扩展最小改动集」+ 本文件第 5 节 |
| 修改技术栈、构建命令、路径别名 | `README.md`「项目概述」+ 本文件第 2 节 |
| 修改命名规范 | `conventions.md` + 本文件第 3 节（各自维护） |
| 修改架构纪律、扩展方式、复用规则 | 本文件第 4 / 5 / 6 节 |
| 修改管理端路由/API 路径约定 | `conventions.md` API 节 + 相关 `modules/*.md` |

> 原则：**文档与代码同源**。每次 PR/commit 前自检——本次改动是否触发表中任一行？若是，文档与代码同提交。

## 2. 项目速览

| 属性 | 值 |
|---|---|
| 技术栈 | Go 1.25 + Gin + GORM/SQLite；React 19 + TypeScript 5.8 + Vite 7 + Tailwind CSS 4；TanStack Query + Zustand；前端包管理 pnpm |
| 形态 | 全栈（Go API 网关 + React 管理后台；`webui/dist` 由 Go `//go:embed`） |
| 架构 | 技术分层 + 业务子包拆分；扩展点用注册表（providers / chat style / transform / virtualmodel selector） |
| 设计原则 | SRP / OCP / DRY / DIP（落地细节见架构总览第 2 节） |
| 路径别名 | 前端 `@` → `webui/src/`（`vite.config.ts` + `tsconfig*.json`）；Go 无路径别名，用 module path `github.com/atopos31/llmio/...` |
| 默认端口 | 后端 `7070`；前端 dev `5173`（代理 `/api` → `localhost:7070`） |
| 数据库 | SQLite `./db/llmio.db`；启动时 GORM AutoMigrate |

### 常用命令

```bash
# 后端
make run              # go fmt + go mod tidy + mkdir db + go run .（默认 :7070）
go run .              # 直接启动
make fmt              # go fmt ./...
make tidy             # go mod tidy
go test ./...         # 全部 Go 测试
go test ./service/virtualmodel/...   # 单包测试
go test -run TestName ./path/to/pkg  # 单测

# 前端（在 webui/ 下）
pnpm install
pnpm dev              # Vite HMR :5173
pnpm run build        # tsc -b && vite build（改前端后进二进制必须 rebuild）
pnpm lint             # eslint .
pnpm test             # vitest run
pnpm test:watch
pnpm preview

# 前端生产构建（供 Go embed）
make webui            # cd webui && pnpm install && pnpm run build

# Windows
.\run.bat             # 构建 webui + 启动（可 --no-build）
.\dev.bat / .\dev.ps1 # 前后端分离开发
.\build.bat           # webui build + go build -o llmux.exe
```

> makefile **无** `test`/`lint` 目标；Go 用 `go test ./...`，前端用 `pnpm lint` / `pnpm test`。  
> 环境变量：`TOKEN`（API 鉴权，未设则跳过）、`GIN_MODE`、`TZ`。

## 2.1 任务导航

> **新会话 AI 接到任务时，先查这张表**——直接定位到该看的文件，不用瞎探索。  
> 架构文档按「模块」组织（`modules/{x}.md`），本表按「任务」组织，把两种视角连起来。

| 任务 | 先看这些文件 | 参考 |
|------|-------------|------|
| 改代理主路径 / 选路 / 重试 / 流式回写 | `handler/v1/`、`service/chat/`（`before.go` / `process.go` / `chat_balance*.go` / `chat_attempt.go`）、`service/chatcore/`、`balancer/`、`service/chat_facade.go` | [modules/chat-gateway.md](docs/architecture/modules/chat-gateway.md) |
| 新增 / 修改客户端协议 style（OpenAI / Anthropic / Responses） | `service/chat/registry.go` + `before.go` / `process.go`、`service/transform/`（adapter + streaming）、`handler/register_v1.go`、`consts/consts.go`、`models/unified/` | [modules/protocol-transform.md](docs/architecture/modules/protocol-transform.md)、[modules/chat-gateway.md](docs/architecture/modules/chat-gateway.md) |
| 新增 / 修改上游供应商 type | `providers/`（实现 + `init` 中 `Register`/`RegisterMetadata`）、`handler/providerapi/`、grep 外围 type 分支（providerapi / testapi / chat 预处理） | [modules/providers.md](docs/architecture/modules/providers.md) |
| 改虚拟模型策略 / 映射 / 第一层 LB | `service/virtualmodel/`（`selector.go`、`*_selector.go`、`service.go`）、`handler/virtualmodels/`、`repository/virtual_model.go`、前端 `webui/src/routes/virtual-models/` | [modules/virtual-models.md](docs/architecture/modules/virtual-models.md)、[docs/virtual-models-guide.md](docs/virtual-models-guide.md) |
| 改模型-供应商关联 / 自动关联 / 能力标志 | `service/autoassoc/`、`service/autoassoc_facade.go`、`handler/autoassoc/`（HTTP 薄层）、`handler/associations/`、`repository/model_with_provider.go`、`service/model_template.go` | [modules/associations.md](docs/architecture/modules/associations.md) |
| 改真实模型目录 / 模板 | `handler/modelapi/`、`repository/model.go`、`repository/model_template_item.go`、`service/model_template.go` | [modules/models-catalog.md](docs/architecture/modules/models-catalog.md) |
| 改模型同步 / 同步后动作 | `service/modelsync/`、`service/model_sync.go`（ActionHooks → autoassoc）、`handler/modelsync/`、`repository/model_sync_log.go` | [modules/model-sync.md](docs/architecture/modules/model-sync.md) |
| 改健康检查 / 关联启停 / 权重衰减 Hook | `service/healthcheck/`、`service/healthcheck.go`、`handler/healthcheck/`、`service/adjustment/`（AdjustmentHooks 注入 + 权重/优先级算法）、`repository/health_check_log.go` | [modules/health-check.md](docs/architecture/modules/health-check.md) |
| 改请求日志 / 仪表盘指标 / 保留策略 | `handler/logs/`、`handler/metrics/`、`service/chat` 的 record/落库编排、`service/chatstats/`、`repository/chat_log.go`、`models/retention.go` | [modules/logs-metrics.md](docs/architecture/modules/logs-metrics.md) |
| 改系统设置项 | `models/setting_schema.go`、`handler/settings/`、`service/settings/`、`repository/setting.go`、前端 `webui/src/routes/settings/` + `lib/api` settings 模块 | [modules/settings.md](docs/architecture/modules/settings.md) |
| 新增管理 API 域 | `handler/<domain>/routes.go` + 实现、`handler/register.go` 的 `RegisterAll` 一行挂接、可选 `service/` + `repository/`、前端 `lib/api` + 页面 | [modules/system-ops.md](docs/architecture/modules/system-ops.md) |
| 新增 / 改管理后台页面 | `webui/src/routes/<page>/`、`webui/src/routes/route-config.ts`、`webui/src/lib/api/modules/**`、必要时 `stores/` | [modules/webui.md](docs/architecture/modules/webui.md) |
| 改鉴权 / 启动装配 / 导入导出 / 连通测试 | `main.go`、`middleware/auth.go`、`handler/register.go`、`handler/importexport/`、`handler/database/`、`handler/testapi/`、`httpresp/`、`handler/httpx/` | [modules/system-ops.md](docs/architecture/modules/system-ops.md) |
| 改构建 / 部署 / embed | `makefile`、`Dockerfile`、`docker-compose.yml`、`run.bat`/`build.bat`、`main.go` embed 声明 | — |

## 3. 编码规范

### 3.1 命名

> 与 `docs/architecture/conventions.md` 各自维护：本表侧重「该这么命名」的操作规范。

| 对象 | 约定 | 示例 |
|---|---|---|
| Go 顶层包目录 | 小写短名 | `handler/`、`service/`、`providers/`、`repository/` |
| handler 业务子包 | 小写，业务域缩写或复数 | `providerapi/`、`modelapi/`、`virtualmodels/` |
| service 业务子包 | 小写，常为单数领域名 | `chat/`、`virtualmodel/`、`healthcheck/`、`modelsync/` |
| Go 源文件 | snake_case | `chat_balance.go`、`model_with_provider.go` |
| Go 测试文件 | `*_test.go` 与实现同目录 | `provider_test.go` |
| 路由注册文件 | `register*.go` 或子包 `routes.go` | `handler/register_v1.go`、`providerapi/routes.go` |
| service 门面 | `*_facade.go` 或同名包装 | `chat_facade.go`、`virtual_model.go` |
| Go 导出类型/函数 | UpperCamelCase | `RegisterAll`、`BalanceChat` |
| Go 局部变量 | lowerCamelCase | `syncService`、`modelID` |
| Go 接口 | 名词或 `XxxRepo` / 能力名 | `Provider`、`Selector`、`ModelRepo` |
| JSON / API 字段 | snake_case | `provider_model`、`max_retry` |
| 协议 style 常量 | `Style` 前缀 | `StyleOpenAI`、`StyleAnthropic` |
| 设置键常量 | `SettingKey` 前缀 | `SettingKey*` |
| 虚拟模型策略字符串 | snake_case | `priority`、`round_robin`、`random` |
| 供应商 type 字符串 | 小写连字符 | `openai`、`openai-res`、`anthropic` |
| 前端路由目录 | kebab-case | `virtual-models/`、`model-providers/` |
| 前端组件/页面文件 | kebab-case | `models-desktop-table.tsx`、`use-models-page.ts` |
| 前端组件名 | PascalCase | `ModelsPage`、`ThemeProvider` |
| 前端 hooks | camelCase + `use` 前缀 | `useModelsPage` |
| 前端 store selector | 常见 `select` 前缀 | `selectIsAuthenticated` |

### 3.2 模块组织

**Go**

- 业务路由**只**经 `handler.RegisterAll` 挂接；子包 `Register*` 只挂叶子路由，**禁止**在 `main.go` 写业务路由明细。
- `service/` 根包门面（`*_facade.go` 等）是**兼容 re-export / 装配**，不是第二套业务实现；新逻辑写在子包（`service/chat` 等）。
- 管理端 CRUD 优先走 `repository.*Repo`；`repository.SetDefault` 在 `main` 绑定。chat / healthcheck / modelsync / virtualmodel 现状仍可直连 `models.DB`——**新增代码优先仓储，不扩大直连面**。
- 跨 service 副作用用 Hook 接口（`AdjustmentHooks`、`ActionHooks`），契约由**调用方包**定义，实现侧包级注入，**禁止**为副作用制造循环 import。

**前端**

- 页面目录：`webui/src/routes/<page>/`，含 `index.tsx`、`hooks/`、`components/sections|dialogs/`、`schemas/` / `types/` / `utils/`。
- 路由与侧栏的**单一数据源**：`webui/src/routes/route-config.ts`；新增页面 = 新目录 + config 追加一项，`App`/`layout` 自动派生。
- API：`webui/src/lib/api/core/client.ts` + `lib/api/modules/**`；页面 hooks 调 API 模块，不散落裸 `fetch`。

### 3.3 导入

**Go**

- 使用 module path：`github.com/atopos31/llmio/...`。
- 分层依赖见第 4 节；下层包**禁止** import 上层业务包。
- handler 优先依赖 `service` 门面与 `repository`，避免深挖 `service/chat` 内部未导出细节（除非同域扩展）。

**前端**

- 优先路径别名 `@/`，**禁止**跨层相对路径（如 `../../../lib`）。
- 仅导入类型时用 `import type { ... }`。
- 导入顺序：第三方 → `@/` 别名 → 相对路径（同模块内）。

### 3.4 类型与错误

**Go**

- 错误处理：`if err != nil`；返回带上下文的 `fmt.Errorf("...: %w", err)`（或等价包装），**禁止**静默吞错。
- HTTP 响应统一走 `httpresp`（`Success` / `BadRequest` / `NotFound` / `InternalServerError` 等），**禁止**各 handler 自造不一致信封。
- 日志用标准库 `log/slog` 结构化键值，**禁止**再引入第二套日志门面（除非全仓迁移）。
- 注册表重复注册：必须 **panic**（与现有 `Register*` 行为一致），测试用 `TestRegister*_DuplicatePanics` 类用例覆盖。

**前端**

- TypeScript 严格模式；**禁止**无注释的 `any`；确需放宽用 `unknown` 并在使用处收窄。
- API 信封约定：`{ code, message, data }`，`code !== 200` 抛错；401 跳转 `/login`。
- 同一概念只定义一次：DTO/表单 schema 放页面 `schemas/` 或共享类型目录，避免 routes 与 stores 重复手写同构类型。

### 3.5 注释与测试

- **不无故增删注释**。改代码时同步更新其上方注释。
- 注释解释「为什么」与契约，不复述代码「是什么」。关键设计决策注明原则，如 `// 新 type 只 Register，不改 New 分发（OCP）`。
- Go 测试：`Test<Feature>` / `TestXxx_Scenario`；优先表驱动；与实现同目录。
- 前端测试：Vitest + Testing Library + jsdom（`pnpm test`）。
- 出现 bug：**先写回归测试再修**（Go 侧至少一测；前端至少相关 build/lint 冒烟）。

## 4. 架构纪律（硬性）

> 语气：禁止 / 必须。与架构总览「设计原则落地」「禁止的依赖」一致。

### 4.1 依赖方向

- **必须**遵守：`webui`/外部客户端 → `middleware` + `handler` → `service` → `providers` + `repository` → `models` + SQLite；`common` / `consts` / `balancer` / `httpresp` 只被上层依赖。
- **禁止** `repository` → `service` / `handler`。
- **禁止** `models` → `service` / `handler` / `repository` 业务逻辑。
- **禁止** `providers` → `handler` / `service` 业务编排。
- **禁止** `common` / `httpresp` / `consts` / `balancer` / `models/unified` → 业务模块。
- **禁止** `middleware` → `handler` 业务包。
- **禁止** `main` 直接注册业务路由明细；**必须**只调 `handler.RegisterAll`（及启动装配：DB、`repository.SetDefault`、后台任务）。

### 4.2 OCP / 扩展

- **禁止**为新增供应商 type 修改 `providers.New` 分发 switch；**必须** `Register` + `RegisterMetadata`（通常在 `init`）。新 type 落地后**必须** grep 外围 type 分支并消除或显式收录。
- **禁止**假设「注册表 = 只改一个文件」。新协议 style **必须**按最小改动集多点注册（chat Beforer/Processer + transform adapter/流路由 + `register_v1` + `consts.Style*`）。
- 新虚拟模型策略：**必须** `RegisterSelector`；若算法不在现有 `selectBy*`，**必须**改 Service 并同步前端策略白名单。
- 新设置项：**必须**写入 `models/setting_schema.go` + handler DTO + 前端 settings 类型/表单；键名**必须**用 `SettingKey*`，**禁止**魔法字符串。
- 新管理 API 域：**必须**新 `handler/<domain>/` + 修改 `RegisterAll` 挂接；**禁止**写进 `main.go`。
- 新管理页面：**必须** `routes/<page>/` + `route-config.ts` 追加；**禁止**只在 `App.tsx` 硬编码旁路路由而不进 config。

### 4.3 SRP / 边界

- HTTP 接入在 `handler/*`；上游适配在 `providers/`；协议字段转换在 `service/transform/`（及 anthropic/responses）；**禁止**在 handler 写上游 HTTP 细节或在 providers 写选路重试。
- 虚拟模型**只**解析虚拟→真实（第一层 LB）；**禁止**在 virtualmodel 包发起上游调用或做真实→供应商选路。
- healthcheck **禁止**直接 import `service/adjustment` 或 chat 内部调整函数；权重/优先级调整**必须**经 `AdjustmentHooks`（实现侧在 `service/adjustment` 包级注入）。
- modelsync 同步后副作用**必须**经 `ActionHooks`；**禁止** modelsync 硬依赖 `handler/autoassoc` 包。
- `system-ops` 是横切合集文档桶：**禁止**把新业务规则继续堆进 common / main / 无主 handler。
- webui **禁止**实现服务端选路/协议转换/持久化规则；页面只做 API 编排与展示（已知缺口：默认 weight/priority 等前端硬编码——新增时不要扩大此类复制）。

### 4.4 DRY / DIP

- 协议中枢**必须**走 `models/unified`（及 `unified.go` 门面已有类型）；**禁止**新开 N×M 协议直转而不经统一模型。
- OpenAI 兼容供应商**优先**嵌入 `openaiBase` 并配置 `endpointPath`，**禁止**复制一整套 BuildReq。
- 管理端 JSON 绑定优先 `handler/httpx`；响应**必须** `httpresp`。
- 跨 2 个以上域复用的纯逻辑**必须**下沉到 `common/` 或既有 shared 子包，且 shared **禁止**反向依赖业务包。
- 新增数据访问：**优先** `repository` 接口；**禁止**在新代码中无必要地扩大 `models.DB` / `gorm.G` 直连（存量直连不作为新代码模板）。

## 5. 扩展指南（OCP）

### 5.1 新增上游供应商 type

1. 在 `providers/` 新增实现文件；OpenAI 兼容优先嵌入 `openaiBase`。
2. 在 `init()` 中调用 `Register(type, Factory)` 与 `RegisterMetadata(Metadata)`。
3. **不要**修改 `New` 的 switch 分发。
4. grep 全仓 type 字符串 / 分支（`providerapi`、`testapi`、chat 出站预处理、`AllMetadata` 顺序等），补齐或消除特判。
5. 管理端如需模板展示：确认 Metadata 字段完整；前端供应商表单如有 type 枚举则同步。
6. 补充/更新 `providers` 测试；`go test ./providers/...`。

### 5.2 新增客户端协议 style

1. 在 `consts` 增加 `Style*` 常量。
2. `service/chat`：实现并 `RegisterBeforer` / `RegisterProcesser`。
3. `service/transform`：`RegisterAdapter`；若涉及 SSE，注册 `RegisterRealtimeRoute`。
4. `handler/register_v1`（及 v1 handler）挂路由。
5. 统一模型字段不足时扩展 `models/unified/`（一处定义，转换器共用）。
6. 补 golden/单元测试；`go test ./service/transform/... ./service/chat/...`。

### 5.3 新增虚拟模型策略

1. 新增 `service/virtualmodel/*_selector.go`，`init` 中 `RegisterSelector`。
2. 若需新算法：实现于 Service 的 `selectBy*`（或把算法从 Service 下沉到 selector，避免薄包装谎言）。
3. 同步前端策略枚举/校验（`webui` virtual-models 表单 schema）。
4. 重复注册必须 panic；补 `TestRegisterSelector_*`。

### 5.4 新增系统设置项

1. 在 `models/setting_schema.go` 声明 schema（类型、默认、范围、枚举）+ `SettingKey*`。
2. 更新 `handler/settings` DTO 与 get/update 路径（含交叉字段 normalize，若有）。
3. 更新前端 `lib/api` settings 类型 + `routes/settings` 表单。
4. 业务读取**必须**经 schema/getter，**禁止**手写默认值分叉；注意现有多热读路径，新增读取优先复用 `models.GetSetting*` 或 `service/settings`。

### 5.5 新增管理 API 域

1. 新建 `handler/<domain>/`，提供 `Register(rg gin.IRoutes)`（复杂域可拆 `RegisterHead`/`RegisterCRUD`）。
2. 在 `handler/register.go` 的 `RegisterAll` 中按现序风格挂接；Group/Use **只**在 `RegisterAll`。
3. 业务逻辑放 `service/` 子包或既有 service；持久化放 `repository/`。
4. 响应走 `httpresp`；绑定走 `httpx`（若适用）。
5. 前端：`lib/api/modules` + 页面 + `route-config.ts`。

### 5.6 新增管理后台页面

1. 创建 `webui/src/routes/<page>/`（index + hooks + sections/dialogs）。
2. 在 `route-config.ts` 追加 path/layout/nav。
3. API 写入 `lib/api/modules/**`，页面经 hooks 调用。
4. `pnpm lint` + `pnpm run build`；若需进二进制再 `make webui`。

### 5.7 配置驱动优先

- 运行时行为变更优先：`models.Setting`、Provider.Config/Proxy/Blacklisted、VirtualModel.Strategy、关联 Weight/Priority/Status、环境变量 `TOKEN`/`GIN_MODE`/`TZ`。
- **禁止**为可配置行为新增硬编码分支（除非架构文档标明的未闭环例外，并在 PR 说明）。

## 6. 复用策略（DRY）

### 共享层位置

```
common/           # 纯工具（maputil、sse、dataurl…）；禁止依赖 Gin/业务包
consts/           # 全局常量（Style*、SettingKey* 等）
balancer/         # 无状态加权随机算法
httpresp/         # 管理端统一响应信封
handler/httpx/    # 绑定等 HTTP 辅助
models/unified/   # 协议统一模型（+ models/unified.go 门面）
models/setting_*  # 设置 schema / reader / convert
providers/openai_base.go  # OpenAI 兼容基类
webui/src/lib/    # api client、formatters、utils
webui/src/components/ui/  # 基础 UI
```

### 复用规则

- 跨 2 个以上业务域使用的逻辑 → 提取到上表共享位置，**禁止**复制粘贴到多个 handler/service。
- 共享层**只能**被上层引用，**禁止**依赖 `handler` / 具体业务 `service` 子包。
- 协议转换**必须**经统一模型中枢，**禁止**每对协议手写双向转换。
- 自动关联业务**必须**落在 `service/autoassoc`；HTTP / provider CRUD / modelsync ActionHooks 只调该服务。**禁止**在 handler 或 model_sync 门面再复制一套关联/清理循环。
- 前端：分页/表格/日志类 UI 优先复用已有组件与 hooks；**禁止**在 routes 间复制同构大段 JSX/类型。
- service 根包门面只做 re-export：**禁止**在门面文件堆积业务分支。

## 7. 框架专属约定

### 7.1 Go / Gin

- 路由：`RegisterAll` 建 `/v1` 与 `/api`；`/api` 挂 `middleware.Auth(token)`。
- 子包注册函数只接收 `gin.IRoutes`（或项目既有签名），**不**在子包内 `Group` 出与全局不一致的鉴权边界。
- 后台任务在 `main` 启动：`HealthChecker.Start`、`ModelSyncService.StartAutoSync`；**禁止**在 handler 请求路径里偷偷起无生命周期管理的全局 goroutine（除非既有模式且有 ctx 取消）。
- 前端产物：`//go:embed webui/dist`；改前端后未 rebuild 则二进制仍是旧 UI。
- 并发：虚拟模型轮询等共享计数**必须**用 `sync.Mutex`（或等价）保护，与现实现一致。

### 7.2 React / Vite 管理端

- 服务端状态：TanStack Query；客户端状态：Zustand（`stores/core/updater` 统一更新模式）。
- 表单：react-hook-form + zod；UI：Radix + Tailwind + CVA。
- 路径别名：`@/` → `src/`。
- 开发代理：`/api` → `http://localhost:7070`。
- 页面**不得**绕过 `lib/api` 核心客户端自造鉴权 header 逻辑（除非扩展 client 本身）。

### 7.3 协议与代理

- 对外：OpenAI/Anthropic 兼容 `/v1/*`；对内管理：`/api/*`。
- 认证：OpenAI 风格 `Authorization: Bearer <TOKEN>`；Anthropic 风格 `x-api-key: <TOKEN>`。
- 流式：SSE；转换与 process 路径分流行/非流，**禁止**混用缓冲假设。
- 两层 LB：虚拟→真实（virtualmodel）+ 真实→供应商（chatcore/balancer）；改一层时**禁止**破坏另一层的候选过滤（如黑名单在候选池）。

## 8. 提交前自检清单

> 每项须可机械验证（命令 / grep / 文件存在性）。

- [ ] `go test ./...` 通过
- [ ] 若改动 Go 源文件：`go fmt ./...` 已执行（或 `make fmt`）
- [ ] 若改动前端：`cd webui && pnpm lint` 通过
- [ ] 若改动前端：`cd webui && pnpm run build` 通过（`tsc -b && vite build`）
- [ ] 若需嵌入二进制验证：`make webui` 后服务能拉起管理端
- [ ] 新增/修改公共注册：无重复注册；相关 `TestRegister*_DuplicatePanics` 或等价测试仍通过
- [ ] 新供应商 type：已 `Register`+`RegisterMetadata`，且 `rg`/`grep` 外围 type 分支已处理
- [ ] 新协议 style：chat 注册 + transform 适配 + `register_v1` + `consts.Style*` 四处（及流式路由若需要）均已改
- [ ] 新设置项：`setting_schema.go` + handler DTO + 前端类型/表单三处同步；无新魔法字符串键
- [ ] 新管理 API 域：只通过 `RegisterAll` 挂接，`main.go` 无新业务路由
- [ ] 新管理页面：`route-config.ts` 已追加；API 在 `lib/api/modules`
- [ ] 依赖方向：无 `repository`→`service`、`providers`→`handler` 等违规 import（可用 go list / 代码审查）
- [ ] 跨域副作用：healthcheck/modelsync 未直接 import 对方内部函数，而是走 Hook
- [ ] 自动关联规则变更：只改 `service/autoassoc`；handler / modelsync hooks 无重复业务逻辑
- [ ] HTTP 错误/成功响应走 `httpresp`；未引入第二套响应信封
- [ ] 日志为 `slog`；错误带上下文返回
- [ ] 命名符合第 3.1 节（Go snake_case 文件名、JSON snake_case、前端 kebab-case 路由目录等）
- [ ] 文档同步义务：目录/模块/契约/纪律变更已更新 `docs/architecture/**` 与本文件相应节
- [ ] 无 API Key / Token / 供应商密钥进入仓库；密钥仅环境变量或本地未跟踪配置
- [ ] 提交信息：Conventional Commits（`feat:`/`fix:`/`refactor:` 等，可带范围如 `feat(webui):`）

---

*本文件由 Agents MD Generator skill 生成，文件名：AGENTS.md，生成日期：2026-07-23*
