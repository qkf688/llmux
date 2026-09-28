# providers 模块

> 本文件描述供应商适配与管理端供应商域的职责与边界。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 上游供应商配置、HTTP 适配、元数据模板 |
| 目录位置 | `providers/`；`handler/providerapi/`；`repository/provider.go`；实体 `models.Provider` |
| 主要职责 | 定义供应商接口；按 type 构造实现；构建上游请求；拉取上游模型列表；管理端 CRUD/黑名单/模板 |

## 2. 职责与边界

- **负责什么**：`Provider` 工厂与实现（OpenAI / OpenAIRes / Anthropic）；客户端缓存；Metadata（模板、TestBody/StructuredBody/ThinkingBody/VisionBody 诊断测试体、HealthCheckBody、ProbeBody=#24-B 最小 completion 探针体）；管理端供应商 CRUD、黑名单、上游模型列表展示、组织级模型目录聚合
- **不负责什么**：模型-供应商关联行（`associations`）；协议互转（`protocol-transform`）；chat 选路与重试；虚拟模型策略
- **对外暴露**：`providers.Provider`、`Register`/`New`、`RegisterMetadata`/`MetadataOf`/`AllMetadata`、`OpenAICompat`、`BetaFeatureCapable`、`CredentialCapable`；`handler/providerapi` REST（含 `/api/providers/model-catalog`）；`ProviderRepo`
- **依赖谁**：`models`、`consts`、少量 `common`；handler 侧 `repository`/`httpresp`/`service/modelsync`（model-catalog 聚合 + 上游模型列表选路，两者逻辑都不在 handler）

## 3. 内部结构

```
providers/
├── provider.go          # 接口（BuildReq 契约：实现就地改传入 header，调用方不得复用——4b）、Factory 注册表
├── auth.go              # 认证方式单源：applyAuthHeader 唯一凭据头执法点 + AuthType 枚举 + DeriveConfigAuthType 展示派生（4b / Stage 20）
├── meta.go / meta_bodies.go
├── openai_base.go       # OpenAI 兼容公共基类
├── openai.go / openai_res.go / anthropic.go
└── cache.go             # HTTP 客户端缓存
handler/providerapi/     # CRUD、templates、upstream models、blacklist、model-catalog
  model_catalog.go       # GET /api/providers/model-catalog（聚合目录薄层，逻辑在 service/modelsync）
  models_upstream.go     # GET /api/providers/models/:id（HTTP 薄层，选路+拉取在 service/modelsync.ListUpstreamModels）
  structured.go         # S6 结构化读写：loadProviderDetail（详情含掩码项）/ syncProviderChildren（四表事务同步）
  inline_keys.go        # GET /api/providers/:id/groups/:groupId/inline-keys/raw（受控明文拉取）
repository/provider.go
service/modelsync/config.go  # ProviderModelCatalog 类型 + GetProviderModelCatalogs 批量聚合
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Provider` | `BuildReq` / `Models` / `GetProxy` | `providers/provider.go` | OpenAI、OpenAIRes、Anthropic |
| 认证方式单源（4b，审查后按协议拆键；Stage 20 派生化） | `applyAuthHeader`（`providers/auth.go`）是**唯一**凭据头执法点：`bearer` 只发 `Authorization: Bearer <key>` 并删任何配进来的 `x-api-key`；`x-api-key` 反之；空值/未知值回退类型默认（anthropic=x-api-key、openai 系=bearer——**存量配置零迁移**）。写凭据头的一切位点（`BuildReq` 与 `Models` 旁路，含 GET /models）必须走该函数，**禁止**再手写 `Set("Authorization")`。**config 键按协议拆分**：anthropic 读 `auth_type`（存量键语义不变），openai 系读 `openai_auth_type`——一份 config 会喂给按端点协议派生的多家 adapter，共键会让存量混合行（anthropic 主类型+openai 协议，旧 UI 恒写 `auth_type:x-api-key`）在无人保存时静默翻转 openai 端点凭据头（审查 P0）。顶层 `Provider.AuthType` = **config 派生的只读展示值**（Stage 20，2026-09-24 消灭双写：`gorm:"-"` 不落库、DTO 顶层 `auth_type` 写入口已删，派生单源 `providers.DeriveConfigAuthType`——anthropic 读 `auth_type`、其余读 `openai_auth_type`，值原样返回不做过滤（垃圾值暴露配置错误优于静默回默认），读侧详情/列表/导出出库后填充；此前是 anthropic 专属列副本，PUT 顶层 `auth_type` 可造出「列显 bearer、实际发 x-api-key」的漂移，旧 `resolveAuthType` 已随写侧删除）。**诊断路径方言贯通（Stage 15，2026-09-20）**：openai ReAct 诊断走 openai-go SDK 直发、不经 `BuildReq`，其凭据头经 `OpenAICompatAuthHeaders()` 搬运——同一 `applyAuthHeader` 单源按 `openai_auth_type` 写好头，诊断侧只搬运不 switch 方言，与真实转发永不分叉（修复前 SDK 固定 bearer，x-api-key 方言中转诊断必假阴性）。契约测试 `providers/auth_headers_test.go`（BuildReq 注入面 + httptest 上游侧断言「键缺失≠空值头」且至多一个凭据头 + 拆键互不读取回归钉 + `OpenAICompatAuthHeaders` 方言口径与 BuildReq 同源钉）、`providers/derive_auth_type_test.go`（派生口径表驱动）、`handler/providerapi/auth_type_derived_test.go`（漂移输入不生效 + openai 方言派生 + 未设置=null + 不落库，打响应体） | `providers/auth.go` | `openaiBase`、`Anthropic` |
| `OpenAICompat` | 标记 OpenAI 兼容协议族并暴露 baseURL/apiKey/**按方言写好的凭据头**（`OpenAICompatAuthHeaders()`，Stage 15：诊断路径不自行判断方言） | `providers/provider.go` | OpenAI、OpenAIRes |
| `BetaFeatureCapable` | `HasBetaFeature(name)`：查询本供应商已启用的 beta 特性，供上层判断协议约束是否被 beta 豁免 | `providers/provider.go` | Anthropic（判据为配置的 `Beta` 字段） |
| `CredentialCapable` | `CredentialPlaintext()`：读取当前装配的凭据明文，供诊断前置守卫判定「明文为空 = 配置错误，本地失败、零上游请求」（2026-09-20 补齐 anthropic ReAct 路径）；明文**禁止**进日志与响应，调用方限 `testapi` ReAct runner 的空密钥守卫。openai 系经 `OpenAICompatAPIKey()` 读取，不重复实现本接口 | `providers/provider.go` | Anthropic |
| `Factory` + `Register`/`New` | 按 type 构造实现 | `providers/provider.go` | 各实现 `init` 注册 |
| `Metadata` | 配置模板与探测 body | `providers/meta.go` | `RegisterMetadata` |
| `ProviderRepo` | 供应商持久化 | `repository/provider.go` | GORM 实现 |
| `models.Provider` | 供应商实体（`AuthType` 为 **config 派生只读展示值**，`gorm:"-"` 不落库，Stage 20 消灭双写——见上「认证方式单源」行） | `models/model.go` | GORM |
| 内联凭据视图回填 | 详情 `GroupDetail.InlineKeys` 为 `[]credview.View`（凭据视图单源，与 `/api/credentials` 列表同一形状）——掩码 + 状态/冷却/连败；明文永不出常规响应，保存时按 `id` 引用（key 空 = 保留、key 非空 = 替换，id=0 = 新增） | `handler/providerapi/types.go` + `structured.go` | `loadProviderDetail`（`credview.ViewsOf`）/ `syncGroups` |
| 内联 key 状态与人工恢复 | 编辑弹窗的内联 key 行显示状态徽标（`CredentialStatusBadge`），非健康行带「启用」、健康行带「停用」，组头另有「全部启用 N 条」；动作走凭据域 `PATCH /api/credentials/status`（`batchUpdateCredentialStatus`，单条也是它），成功后按**最新表单行**回写状态（重拉详情会覆盖未保存的表单编辑）。**保存表单不会清判停状态**（`syncGroups` 只写 key/归属，不碰 status/fail_count）——所以「换 key」后仍需点一次「启用」或等组内惰性探活成功。状态字段不参与提交 | `webui/src/routes/providers/components/dialogs/inline-keys-field.tsx` + `webui/src/components/credentials/credential-status-badge.tsx` | `handler/credentials`（状态读写）/ `handler/providerapi`（视图） |
| `GET /api/providers/:id/groups/:groupId/inline-keys/raw` | 受控明文拉取：解密返回 `[{id,key}]`（按凭据 id 归位，前端不依赖数组顺序）；池来源分组 400、供应商/分组不存在或越组 404（不暴露存在性）、cipher 未配置或**任一条解密失败 → 整组 500**（明文替换是覆盖动作，部分成功会静默丢条目）；响应 `Cache-Control: no-store`（走 `httpresp.SuccessNoStore`，明文不得进浏览器/代理缓存） | `handler/providerapi/inline_keys.go` | 同文件 |
| 内联 key 单条复制 | 明文行（`masked=false`）带复制按钮，走 `lib/clipboard.ts` 的 `copySecretWithToast`（机密专用：禁 `window.prompt` 兜底，prompt 的默认值参数会把明文整条弹上屏）；**掩码行刻意无复制入口**——供应商侧只有整组 raw 端点、没有单条明文出口（单条明文在凭据域 `GET /api/credentials/:id/raw`，弹窗不为此多引一处明文通道），要复制必须先点「显示」。成功提示只回显行号：前端不重建掩码（掩码单源在后端 `KeyMasked`），更不能把明文写进任何用户可见文案 | `webui/src/routes/providers/components/dialogs/inline-keys-field.tsx` | `lib/clipboard.ts` |
| `GET /api/providers/models/:id` | 拉真实上游模型列表（供应商「上游模型」弹窗）：**无 query 参数**（`source` 分支已删——本端点唯一语义就是打上游，本地自定义模型走 `model-catalog` 的 `Custom`）；选路与落库同步同源（协议端点 + 分组凭据），多分组返回并集去重；供应商不存在 → `code=404 "Provider not found"`，选路/上游失败 → `code=404 "Failed to get models: <原因>"`（HTTP 恒 200，前端 `apiRequest` 按 `code!=200` 抛 message 并 toast） | `handler/providerapi/models_upstream.go`（HTTP 薄层） | `service/modelsync.ListUpstreamModels` |
| `GroupInput`（结构化分组写入体，2026-09-28） | `{id, name, weight, source, inline_keys, pool_id}`：`id>0` = 就地更新既有分组（未知/重复 id → 400，先全校验再落写），`id=0`/缺失 = 新建；**无 `models` 字段**——`KeyGroup.Models` 是模型同步专属数据，表单保存不得触碰（删光重建会把同步快照冲成空 = 不限、组织级目录缩水连累自动关联清理），未提交的既有组删除。前端分组行携带 `groupId` 回传作就地更新键 | `handler/providerapi/types.go` + `structured.go`（`syncGroups`） | 同文件 |
| `GET /api/providers/model-catalog` | 组织级模型目录聚合：每供应商 `{ProviderID, Upstream(分组白名单并集), Custom(=config.custom_models)}`，PascalCase 无 json tag，空值显式空数组（非 null）；前端三页目录与 all-models 弹窗的**唯一目录数据源**（替代 `Config.upstream_models` 客户端解析，该键为死键）；路由固定段 `model-catalog` 声明在 `/:id` 之前防参数段捕获 | `handler/providerapi/routes.go` + `model_catalog.go`（HTTP 薄层） | `service/modelsync.GetProviderModelCatalogs`（聚合逻辑，SRP） |

## 5. 特殊约定

- **工厂层 OCP**：实现接口 + `init` 中 `Register` 与 `RegisterMetadata`，不改 `New` 分发 switch
- **外围未完全 OCP（现状）**：仍可能出现 type 特判（如 auth 解析、structured 测试、`AllMetadata` preferred 顺序、chat 出站预处理仅部分 style）；新 type 需 grep 外围分支
- OpenAI 兼容供应商优先嵌入 `openaiBase` 并设置 `endpointPath`；模板/TestBody 等同构常量可能重复出现在 `meta_bodies.go`
- 黑名单影响虚拟模型候选池过滤（消费方在 virtualmodel/chat，不在本模块写选路）
- **`Models()` 判定上游是否成功必须走 `common.IsPassingStatus`**（Stage 5，2026-09-17）：本模块两处 `Models()`（`openaiBase` / `Anthropic`）曾各自写严格 `== 200`，与诊断侧 2xx 口径分叉——回 201 的兼容网关拉不到模型列表只是显性症状，同口径在健康检查处会**自动停用关联**。providers 层禁止 import service 子包（依赖方向），故判定落在 `common`，机械执法见 `gate_upstream_status_test.go`、约定见 `framework-conventions.md §6`
- **`Models()` 成功路径读体上限（Stage 27，2026-09-27）**：两处 `Models()` 解码前经 `common.ReadModelsListBody`（上限 `common.ModelsListReadLimitBytes` 10 MiB，量级依据：每模型 ~100-200B、聚合商数百模型 ≈ 100 KiB，10 MiB 为数百倍余量；刻意区别于诊断探针的 1 MiB——清单体量天然大一个数量级）——此前无上限整读，上游异常/被篡改时可被灌进任意体量；达上限视为截断按解码失败处置（错误文本含 `read cap` 与状态码）

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写；2026-09-17 记录上游状态码通过口径收口（Stage 5）。*
