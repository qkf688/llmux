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

- **负责什么**：`Provider` 工厂与实现（OpenAI / OpenAIRes / Anthropic）；客户端缓存；Metadata（模板、TestBody、HealthCheckBody）；管理端供应商 CRUD、黑名单、上游模型列表展示、组织级模型目录聚合
- **不负责什么**：模型-供应商关联行（`associations`）；协议互转（`protocol-transform`）；chat 选路与重试；虚拟模型策略
- **对外暴露**：`providers.Provider`、`Register`/`New`、`RegisterMetadata`/`MetadataOf`/`AllMetadata`、`OpenAICompat`、`BetaFeatureCapable`；`handler/providerapi` REST（含 `/api/providers/model-catalog`）；`ProviderRepo`
- **依赖谁**：`models`、`consts`、少量 `common`；handler 侧 `repository`/`httpresp`/`service/modelsync`（仅 model-catalog 聚合）；拉上游模型时可能经 `service` 辅助

## 3. 内部结构

```
providers/
├── provider.go          # 接口、Factory 注册表
├── meta.go / meta_bodies.go
├── openai_base.go       # OpenAI 兼容公共基类
├── openai.go / openai_res.go / anthropic.go
└── cache.go             # HTTP 客户端缓存
handler/providerapi/     # CRUD、templates、upstream models、blacklist、model-catalog
  model_catalog.go       # GET /api/providers/model-catalog（聚合目录薄层，逻辑在 service/modelsync）
  structured.go         # S6 结构化读写：loadProviderDetail（详情含掩码项）/ syncProviderChildren（四表事务同步）
  inline_keys.go        # GET /api/providers/:id/groups/:groupId/inline-keys/raw（受控明文拉取）
repository/provider.go
service/modelsync/config.go  # ProviderModelCatalog 类型 + GetProviderModelCatalogs 批量聚合
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Provider` | `BuildReq` / `Models` / `GetProxy` | `providers/provider.go` | OpenAI、OpenAIRes、Anthropic |
| `OpenAICompat` | 标记 OpenAI 兼容协议族并暴露 baseURL/apiKey | `providers/provider.go` | OpenAI、OpenAIRes |
| `BetaFeatureCapable` | `HasBetaFeature(name)`：查询本供应商已启用的 beta 特性，供上层判断协议约束是否被 beta 豁免 | `providers/provider.go` | Anthropic（判据为配置的 `Beta` 字段） |
| `Factory` + `Register`/`New` | 按 type 构造实现 | `providers/provider.go` | 各实现 `init` 注册 |
| `Metadata` | 配置模板与探测 body | `providers/meta.go` | `RegisterMetadata` |
| `ProviderRepo` | 供应商持久化 | `repository/provider.go` | GORM 实现 |
| `models.Provider` | 供应商实体 | `models/model.go` | GORM |
| 内联凭据掩码回填 | 详情 `GroupDetail.InlineKeys` 为 `{ID, KeyMasked}[]`（服务端掩码，短 key/解密失败退化为 `****`）；明文永不出常规响应，保存时按 `id` 引用（key 空 = 保留、key 非空 = 替换，id=0 = 新增） | `handler/providerapi/types.go` + `structured.go` | `loadProviderDetail` / `syncGroups` |
| `GET /api/providers/:id/groups/:groupId/inline-keys/raw` | 受控明文拉取：解密返回 `[{id,key}]`（按凭据 id 归位，前端不依赖数组顺序）；池来源分组 400、供应商/分组不存在或越组 404（不暴露存在性）、cipher 未配置或**任一条解密失败 → 整组 500**（明文替换是覆盖动作，部分成功会静默丢条目）；响应 `Cache-Control: no-store`（走 `httpresp.SuccessNoStore`，明文不得进浏览器/代理缓存） | `handler/providerapi/inline_keys.go` | 同文件 |
| 内联 key 单条复制 | 明文行（`masked=false`）带复制按钮，走 `lib/clipboard.ts` 的 `copySecretWithToast`（机密专用：禁 `window.prompt` 兜底，prompt 的默认值参数会把明文整条弹上屏）；**掩码行刻意无复制入口**——供应商侧只有整组 raw 端点、没有单条明文出口（内联凭据 `pool_id` 为空，号池的 `GET /pools/:id/credentials/:credId/raw` 校验 `PoolID == :id`，用不上），要复制必须先点「显示」。成功提示只回显行号：前端不重建掩码（掩码单源在后端 `KeyMasked`），更不能把明文写进任何用户可见文案 | `webui/src/routes/providers/components/dialogs/inline-keys-field.tsx` | `lib/clipboard.ts` |
| `GET /api/providers/model-catalog` | 组织级模型目录聚合：每供应商 `{ProviderID, Upstream(分组白名单并集), Custom(=config.custom_models)}`，PascalCase 无 json tag，空值显式空数组（非 null）；前端三页目录与 all-models 弹窗的**唯一目录数据源**（替代 `Config.upstream_models` 客户端解析，该键为死键）；路由固定段 `model-catalog` 声明在 `/:id` 之前防参数段捕获 | `handler/providerapi/routes.go` + `model_catalog.go`（HTTP 薄层） | `service/modelsync.GetProviderModelCatalogs`（聚合逻辑，SRP） |

## 5. 特殊约定

- **工厂层 OCP**：实现接口 + `init` 中 `Register` 与 `RegisterMetadata`，不改 `New` 分发 switch
- **外围未完全 OCP（现状）**：仍可能出现 type 特判（如 auth 解析、structured 测试、`AllMetadata` preferred 顺序、chat 出站预处理仅部分 style）；新 type 需 grep 外围分支
- OpenAI 兼容供应商优先嵌入 `openaiBase` 并设置 `endpointPath`；模板/TestBody 等同构常量可能重复出现在 `meta_bodies.go`
- 黑名单影响虚拟模型候选池过滤（消费方在 virtualmodel/chat，不在本模块写选路）

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
