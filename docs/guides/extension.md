# 扩展指南与复用策略

> 按需加载文档。操作规范总入口见 [AGENTS.md](../../AGENTS.md)；架构现状见 [docs/architecture/](../architecture/)。
> 本文回答「新增一个 X 要改哪些地方」与「什么时候该下沉到共享层」。硬性纪律（禁止/必须）在 AGENTS.md 第 4 节。

## 1. 扩展指南（OCP）

### 1.1 新增上游供应商 type

1. 在 `providers/` 新增实现文件；OpenAI 兼容优先嵌入 `openaiBase`。
2. 在 `init()` 中调用 `Register(type, Factory)` 与 `RegisterMetadata(Metadata)`。
3. **不要**修改 `New` 的 switch 分发。
4. grep 全仓 type 字符串 / 分支（`providerapi`、`testapi`、chat 出站预处理、`AllMetadata` 顺序等），补齐或消除特判。
5. 管理端如需模板展示：确认 Metadata 字段完整；前端供应商表单如有 type 枚举则同步。
6. 补充/更新 `providers` 测试；`go test ./providers/...`。

### 1.2 新增客户端协议 style

1. 在 `consts` 增加 `Style*` 常量。
2. `service/chat`：实现并 `RegisterBeforer` / `RegisterProcesser`。
3. `service/transform`：`RegisterAdapter`；若涉及 SSE，注册 `RegisterRealtimeRoute`。
4. `handler/register_v1`（及 v1 handler）挂路由。
5. 统一模型字段不足时扩展 `models/unified/`（一处定义，转换器共用）。
6. 补 golden/单元测试；`go test ./service/transform/... ./service/chat/...`。

### 1.3 新增虚拟模型策略

1. 新增 `service/virtualmodel/*_selector.go`，`init` 中 `RegisterSelector`。
2. 若需新算法：实现于 Service 的 `selectBy*`（或把算法从 Service 下沉到 selector，避免薄包装谎言）。
3. 同步前端策略枚举/校验（`webui` virtual-models 表单 schema）。
4. 重复注册必须 panic；补 `TestRegisterSelector_*`。

### 1.4 新增系统设置项

1. 在 `models/setting_schema.go` 声明 schema（类型、默认、范围、枚举）+ `SettingKey*`。
2. 更新 `handler/settings` DTO 与 get/update 路径（含交叉字段 normalize，若有）。
3. 更新前端 `lib/api` settings 类型 + `routes/settings` 表单。
4. 业务读取**必须**经 schema/getter，**禁止**手写默认值分叉；注意现有多热读路径，新增读取优先复用 `models.GetSetting*` 或 `service/settings`。

### 1.5 新增管理 API 域

1. 新建 `handler/<domain>/`，提供 `Register(rg gin.IRoutes)`（复杂域可拆 `RegisterHead`/`RegisterCRUD`）。
2. 在 `handler/register.go` 的 `RegisterAll` 中按现序风格挂接；Group/Use **只**在 `RegisterAll`。
3. 业务逻辑放 `service/` 子包或既有 service；持久化放 `repository/`。
4. 响应走 `httpresp`；绑定走 `httpx`（若适用）。
5. 前端：`lib/api/modules` + 页面 + `route-config.ts`。

### 1.6 新增管理后台页面

1. 创建 `webui/src/routes/<page>/`（index + hooks + sections/dialogs）。
2. 在 `route-config.ts` 追加 path/layout/nav。
3. API 写入 `lib/api/modules/**`，页面经 hooks 调用。
4. `pnpm lint` + `pnpm run build`；若需进二进制再 `make webui`。

### 1.7 配置驱动优先

- 运行时行为变更优先：`models.Setting`、Provider.Config/Proxy/Blacklisted、VirtualModel.Strategy、关联 Weight/Priority/Status、环境变量 `JWT_SECRET`/`ADMIN_PASSWORD`/`GIN_MODE`/`TZ`。
- **禁止**为可配置行为新增硬编码分支（除非架构文档标明的未闭环例外，并在 PR 说明）。

## 2. 复用策略（DRY）

### 2.1 共享层位置

```
common/           # 纯工具（maputil、dataurl、bgtask…）；禁止依赖 Gin/业务包
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

### 2.2 复用规则

- 跨 2 个以上业务域使用的逻辑 → 提取到上表共享位置，**禁止**复制粘贴到多个 handler/service。
- 共享层**只能**被上层引用，**禁止**依赖 `handler` / 具体业务 `service` 子包。
- 协议转换**必须**经统一模型中枢，**禁止**每对协议手写双向转换。
- 自动关联业务**必须**落在 `service/autoassoc`；HTTP / provider CRUD / modelsync ActionHooks 只调该服务。**禁止**在 handler 或 model_sync 门面再复制一套关联/清理循环。
- 前端：分页/表格/日志类 UI 优先复用已有组件与 hooks；**禁止**在 routes 间复制同构大段 JSX/类型。
- service 根包门面只做 re-export：**禁止**在门面文件堆积业务分支。

## 3. 模块组织约定

### Go

- 业务路由**只**经 `handler.RegisterAll` 挂接；子包 `Register*` 只挂叶子路由，**禁止**在 `main.go` 写业务路由明细。
- `service/` 根包门面（`*_facade.go` 等）是**兼容 re-export / 装配**，不是第二套业务实现；新逻辑写在子包（`service/chat` 等）。
- 数据访问**必须**经 `repository.*Repo`；`repository.SetDefault` 在 `main` 绑定，包内经 `repos()`（`repos.go`）取默认聚合根。聊天主路径、chatstats / metrics、healthcheck / adjustment / autoassoc / modelsync / modelapi 已完成迁移；存量直连（settings、v1、database、importexport、virtualmodel）**不作为新代码模板**，改到哪儿顺手迁到哪儿。
- 跨 service 副作用用 Hook 接口（`AdjustmentHooks`、`ActionHooks`），契约由**调用方包**定义，实现侧包级注入，**禁止**为副作用制造循环 import。
- handler 优先依赖 `service` 门面与 `repository`，避免深挖 `service/chat` 内部未导出细节（除非同域扩展）。

### 前端

- 页面目录：`webui/src/routes/<page>/`，含 `index.tsx`、`hooks/`、`components/sections|dialogs/`、`schemas/` / `types/` / `utils/`。
- 路由与侧栏的**单一数据源**：`webui/src/routes/route-config.ts`；新增页面 = 新目录 + config 追加一项，`App`/`layout` 自动派生。
- API：`webui/src/lib/api/core/client.ts` + `lib/api/modules/**`；页面 hooks 调 API 模块，不散落裸 `fetch`。
- 导入：优先路径别名 `@/`，避免跨层相对路径（如 `../../../lib`）；仅导入类型时用 `import type`；顺序为第三方 → `@/` → 相对路径。
- 同一概念只定义一次：DTO/表单 schema 放页面 `schemas/` 或共享类型目录，避免 routes 与 stores 重复手写同构类型。
