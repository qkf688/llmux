# AGENTS.md — LLMux 项目协作约定

> 本文件是 AI 协作与人类协作者的项目入口。修改代码或目录结构前，先读本文档与索引中的架构文档。

## 0. 全局最高优先级原则（先读）

> 以下两条为项目当前阶段的最高约束，凌驾于本文件其余章节的默认倾向之上；与后文冲突时以本节为准。

1. **处于开发阶段，不背历史包袱。** 无需强行兼容当前数据库结构/存量数据；发现表结构、字段、迁移不合理时直接改到位（含破坏性 schema 调整），**禁止**为兼容旧数据新增适配层、兼容分支、冗余字段或过渡代码。改动后同步 `models` schema 与 AutoMigrate 即可。
2. **质量优先，不惜工本，着眼长远。** 不因「任务量大」而妥协：宁可多改文件、多做重构，也要选架构最干净、最可维护的方案。**禁止**为省事就近堆代码、打补丁式绕过；该拆的拆、该抽象的抽象、该重构的连带重构，以长期可维护性为最高目标。

## 1. 文档索引

- **架构现状**（「系统长什么样」）：[docs/architecture/README.md](docs/architecture/README.md)（总览/依赖方向/模块索引）、[modules/](docs/architecture/modules/)（各模块职责·边界·接口契约）、[interactions.md](docs/architecture/interactions.md)（调用/Hook/注册表）、[conventions.md](docs/architecture/conventions.md)（命名规范**唯一来源**）
- **操作细则**（按需加载）：[docs/guides/extension.md](docs/guides/extension.md)（新增供应商/协议 style/策略/设置项/API 域/页面的最小改动集 + 复用策略 + 模块组织）、[docs/guides/framework-conventions.md](docs/guides/framework-conventions.md)（Go/Gin 关闭序与 bgtask、React dialog 尺寸与 DialogBody、协议代理约定）
- **用户向**：[docs/virtual-models-guide.md](docs/virtual-models-guide.md)

> 本文件是操作规范（「该怎么改」），只留常驻必需；细则在 `docs/guides/`，现状在 `docs/architecture/`。三者互补，不重复。

### ⚠️ 文档同步义务（重要）

下列改动**必须同步更新对应文档**，否则视为未完成：

| 改动类型 | 需更新的文档 |
|---|---|
| 新增/删除/重命名目录、文件 | 涉及的 `modules/{x}.md`「内部结构」节（**无**全局目录树文件；目录结构用 `list_dir` / `glob` 实时看） |
| 新增/删除/重命名业务模块 | `architecture/README.md` 模块索引 + 新建/删除 `modules/{x}.md` + 本文件第 3 节任务导航（如影响） |
| 修改核心接口契约 / 模块职责边界 | `modules/{x}.md` 的「关键接口契约」/「职责与边界」节 |
| 修改顶层分层或依赖方向 | `architecture/README.md`「全局架构图」+ 本文件第 4 节 |
| 修改扩展最小改动集 / 复用规则 / 模块组织 | `guides/extension.md`（架构文档不写「该怎么改」） |
| 修改框架约定（Gin 关闭序、dialog 规范、协议代理） | `guides/framework-conventions.md` |
| 修改技术栈、构建命令、路径别名 | `architecture/README.md`「项目概述」+ 本文件第 2 节 |
| 修改命名规范 | `conventions.md`（**唯一来源**，本文件不再重复命名表） |
| 修改架构纪律 | 本文件第 4 节 |
| 修改管理端路由/API 路径约定 | `conventions.md` API 节 + 相关 `modules/*.md` |

> 原则：**文档与代码同源**。每次 PR/commit 前自检——本次改动是否触发表中任一行？若是，文档与代码同提交。

## 2. 项目速览

| 属性 | 值 |
|---|---|
| 技术栈 | Go 1.25 + Gin + GORM/SQLite；React 19 + TypeScript 5.8 + Vite 7 + Tailwind CSS 4；TanStack Query + Zustand；前端包管理 pnpm |
| 形态 | 全栈（Go API 网关 + React 管理后台；`webui/dist` 由 Go `//go:embed`） |
| 架构 | 技术分层 + 业务子包拆分；扩展点用注册表（providers / chat style / transform / virtualmodel selector） |
| 设计原则 | SRP / OCP / DRY / DIP（落地细节见架构总览第 2 节） |
| 路径别名 | 前端 `@` → `webui/src/`（`vite.config.ts` + `tsconfig.json` paths）；Go 无路径别名，用 module path `github.com/qkf688/llmux/...` |
| 默认端口 | 后端 `7070`；前端 dev `5173`（代理 `/api` → `localhost:7070`） |
| 数据库 | SQLite `./db/llmux.db`；启动时 GORM AutoMigrate |

### 常用命令

```bash
# 后端（makefile 目标仅 run/fmt/tidy/mkdb/webui/add，无 test/lint）
make run              # go fmt + go mod tidy + mkdir db + go run .（默认 :7070）
make fmt              # go fmt ./...
go test ./...                        # 全部 Go 测试
go test ./service/virtualmodel/...   # 单包测试
go test -run TestName ./path/to/pkg  # 单测

# 前端（在 webui/ 下）
pnpm dev              # Vite HMR :5173
pnpm run build        # tsc -b && vite build（改前端后进二进制必须 rebuild）
pnpm lint             # eslint .
pnpm test             # vitest run（另有 test:watch / preview）
make webui            # 生产构建供 Go embed：cd webui && pnpm install && pnpm run build

# Windows：.\run.bat（构建+启动，可 --no-build）/ .\dev.bat|.\dev.ps1（前后端分离）/ .\build.bat
```

> 环境变量：`JWT_SECRET`（管理后台 JWT 签名密钥，必填）、`ADMIN_PASSWORD`（首启动 admin 密码，未设则随机生成并打印日志）、`GIN_MODE`、`TZ`。支持 `.env` 文件加载（启动时自动读取项目根 `.env`，不存在则跳过；`.env` 已在 `.gitignore`，模板见 `.env.example`）。

## 3. 任务导航

> **新会话 AI 接到任务时，先查这张表**——直接定位到该看的文件，不用瞎探索。
> 「新增」类任务的分步改动集在 [guides/extension.md](docs/guides/extension.md)。

| 任务 | 先看这些文件 | 参考 |
|------|-------------|------|
| 改代理主路径 / 选路 / 重试 / 流式回写 | `handler/v1/`、`service/chat/`（`before.go` / `process.go` / `chat_balance*.go` / `chat_attempt.go`）、`service/chatcore/`、`balancer/`、`service/chat_facade.go` | [modules/chat-gateway.md](docs/architecture/modules/chat-gateway.md) |
| 改供应商内部选路（端点匹配 / 分组 / 凭据轮询+冷却） | `service/channel/`（`selector.go` `assemble.go` 及其余）、`consts/protocol.go`、`repository/`（endpoint / key_group / credential） | [modules/channel-routing.md](docs/architecture/modules/channel-routing.md) |
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

## 4. 架构纪律（硬性）

### 4.1 依赖方向

- **必须**遵守：`webui`/外部客户端 → `middleware` + `handler` → `service` → `providers` + `repository` → `models` + SQLite；`common` / `consts` / `balancer` / `httpresp` 只被上层依赖。
- **禁止** `repository` → `service` / `handler`；**禁止** `models` → `service` / `handler` / `repository` 业务逻辑。
- **禁止** `providers` → `handler` / `service` 业务编排；**禁止** `middleware` → `handler` 业务包。
- **禁止** `common` / `httpresp` / `consts` / `balancer` / `models/unified` → 业务模块。
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
- 新增数据访问：**必须**走 `repository` 接口；**禁止**在新代码中新增 `models.DB` / `gorm.G` 直连（存量直连不作为新代码模板）。

## 5. 编码规范（linter 覆盖不到的部分）

> 命名大小写、import 顺序、`any` 禁用等交给 `go fmt` / `eslint` / `tsc`，本节不重复。命名现状见 `conventions.md`。

- 错误处理：返回带上下文的 `fmt.Errorf("...: %w", err)`（或等价包装），**禁止**静默吞错。日志用标准库 `log/slog` 结构化键值，**禁止**引入第二套日志门面。
- HTTP 响应统一走 `httpresp`（`Success` / `BadRequest` / `NotFound` / `InternalServerError` 等），**禁止**各 handler 自造不一致信封。
- 注册表重复注册：必须 **panic**（与现有 `Register*` 行为一致），测试用 `TestRegister*_DuplicatePanics` 类用例覆盖。
- **直返实体的 JSON 形状必须两侧同源。** 被 handler 直接序列化返回的 GORM 实体（`models.Model` / `ModelWithProvider` / `Provider` / `VirtualModel` / 日志类）对外**以 PascalCase 为主**（多数字段无 json tag，走 Go 字段名），少数存量字段带 snake_case tag（限于 `models.Model` 上的 `auto_associate` / `supports_thinking` / `thinking_levels`，前端按 snake_case 消费）——形状是混合的，**不要假设整体统一**。**注意**：`ModelWithProvider` 的同名能力字段 `SupportsThinking` / `ThinkingLevels` 反而是 **PascalCase**（`ThinkingLevels` 显式 `json:"ThinkingLevels"`），即同一 thinking 语义在两个实体上键名风格**相反**，跨实体新增/复制字段时**尤其**要逐一核对。新增/修改字段**必须**先确认前端 `lib/api` 的 interface 实际读哪个键名并保证两侧一致；**禁止**只改一侧。
- **三态指针字段禁止 `omitempty`。** `*bool` / `*[]string` 等三态字段的 nil 是「继承 / 未设置」的**有效语义**，必须序列化成 JSON `null`。`omitempty` 会把键整个省略，前端无法区分「继承」与「键不存在」，会把继承误判成 override 并在保存时回写错误值——`ModelWithProvider.ThinkingLevels` 曾因此把「继承」静默改写成「显式不约束」，使 `ClampReasoningEffort` 的白名单钳制失效。
- 契约测试**必须**打在响应体 JSON 上（`gjson` 断言 + `Exists()` 区分「键存在且为 null」与「键缺失」）。只断 repository 读回的 Go 字段值会绕过序列化层，测不出键名/omitempty 类断层。
- 注释：**不无故增删**，改代码时同步更新其上方注释；解释「为什么」与契约，不复述「是什么」。关键设计决策注明原则，如 `// 新 type 只 Register，不改 New 分发（OCP）`。
- 测试：Go 用 `Test<Feature>` / `TestXxx_Scenario`、优先表驱动、与实现同目录；前端 Vitest + Testing Library + jsdom。出现 bug **先写回归测试再修**。
- Git：commit message **禁止**任何 AI 署名（`Generated with ...`、`Co-Authored-By: ...`），信息聚焦「为什么」。

## 6. 提交前自检清单

> 每项须可机械验证（命令 / grep / 文件存在性）。

- [ ] `go test ./...` 通过；若改 Go 源文件已执行 `go fmt ./...`（或 `make fmt`）
- [ ] 若改前端：`cd webui && pnpm lint` 与 `pnpm run build`（`tsc -b && vite build`）均通过；需嵌入二进制则 `make webui`
- [ ] 新增/修改公共注册：无重复注册；相关 `TestRegister*_DuplicatePanics` 或等价测试仍通过
- [ ] 新供应商 type：已 `Register`+`RegisterMetadata`，且 `rg` 外围 type 分支已处理
- [ ] 新协议 style：chat 注册 + transform 适配 + `register_v1` + `consts.Style*` 四处（及流式路由若需要）均已改
- [ ] 新设置项：`setting_schema.go` + handler DTO + 前端类型/表单三处同步；无新魔法字符串键
- [ ] 新管理 API 域：只通过 `RegisterAll` 挂接，`main.go` 无新业务路由
- [ ] 新管理页面：`route-config.ts` 已追加；API 在 `lib/api/modules`
- [ ] 新增/修改 dialog：已声明 `size` 档位且内容包 `DialogBody`；`rg 'DialogContent (size|className)' webui/src` 的 className 无 `max-w-*`/`max-h-*`/`h-[...]`
- [ ] 依赖方向：无 `repository`→`service`、`providers`→`handler` 等违规 import
- [ ] 跨域副作用：healthcheck/modelsync 未直接 import 对方内部函数，而是走 Hook
- [ ] 自动关联规则变更：只改 `service/autoassoc`；handler / modelsync hooks 无重复业务逻辑
- [ ] 会写库的后台 goroutine 已走 `bgtask.Go` 登记，未用裸 `go f(context.Background())`
- [ ] HTTP 响应走 `httpresp`；日志为 `slog`；错误带上下文返回
- [ ] 直返实体新增/改 json tag：前端 `lib/api` interface 与测试 mock 已同步；三态指针字段无 `omitempty`；有响应体 JSON 断言覆盖（`rg 'omitempty' models/model.go` 命中的字段须确认非三态）
- [ ] `models.ModelWithProvider` 增删字段：`handler/associations/crud_response_shape_test.go` 的 `modelWithProviderResponseKeys` 表已同步加/删一行（该表是 `/api/model-providers` 响应键契约的单一数据源，覆盖 Create/Update/List 三路径的键存在性与 null 语义）
- [ ] 命名符合 `docs/architecture/conventions.md`（Go snake_case 文件名、JSON snake_case、前端 kebab-case 路由目录等）
- [ ] 文档同步义务：查第 1 节表格，触发行对应的 `docs/architecture/**` / `docs/guides/**` / 本文件已同步
- [ ] 无 API Key / Token / 供应商密钥进入仓库；密钥仅环境变量或本地未跟踪配置
- [ ] 提交信息：Conventional Commits（`feat:`/`fix:`/`refactor:` 等，可带范围如 `feat(webui):`）

---

*本文件由 Agents MD Generator skill 生成，最近更新：2026-08-21*
