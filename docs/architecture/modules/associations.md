# associations 模块

> 本文件描述模型↔供应商关联与自动关联域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 真实模型与供应商的多对多关联及自动关联 |
| 目录位置 | `handler/associations/`；`handler/autoassoc/`（HTTP 薄层）；`service/autoassoc/`（统一业务）；`repository/model_with_provider.go`；实体 `ModelWithProvider` |
| 主要职责 | 关联 CRUD、能力标志、状态、健康视图；自动按模板创建关联；清理无效关联 |

## 2. 职责与边界

- **负责什么**：`ModelWithProvider` 的权重、优先级、能力（tool/structured/image）、thinking、**协议覆盖（允许集合 + 主格式）**、状态、连续失败等字段管理；自动按模板创建关联；清理无效关联
- **不负责什么**：上游连通性测试执行细节（`testapi`）；定时健康检查（`health-check`）；chat 运行时选路算法（消费本表数据）
- **对外暴露**：`handler/associations` REST；`handler/autoassoc` REST（委托 service）；`service/autoassoc.Service`；`ModelWithProviderRepo`；`NewDefaultAssociation`
- **依赖谁**：`repository`（Model / Provider / ModelWithProvider / ModelTemplateItem / KeyGroup）；`service.BuildTemplateIndexFromData`（经门面注入）；`service/modelsync.GetProviderModels`（分组白名单并集 ∪ custom_models）

## 3. 内部结构

```
handler/associations/     # CRUD、status、capabilities、health 视图
handler/autoassoc/        # HTTP 预览/执行 + Trigger* 薄封装
service/autoassoc/        # 统一 Associate / CleanInvalid 业务
  ├── service.go            # Service 聚合根 + 构造 + setting/Result/LogResult + 公共入口（Preview/Associate/Trigger*）
  ├── associate.go          # Associate 路径（associate / fetchAssociateData / forEachMissingAssociation / skipAutoAssociate）
  ├── clean.go              # CleanInvalid 路径（CleanInvalid / fetchCleanData / forEachInvalidAssociation）
  ├── shared.go             # 共享 helpers（key/index/blacklist/allows）
  ├── defaults.go           # Fallback 常量 + NewDefaultAssociation
  └── types.go              # Preview / associationCandidate / NameMatcher / BuildIndexFunc
service/autoassoc_facade.go  # GetAutoAssocService / NewAutoAssocService
repository/model_with_provider.go
models.ModelWithProvider
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ModelWithProviderRepo` | 关联持久化与按模型/供应商查询删除 | `repository/model_with_provider.go` | GORM |
| `models.ModelWithProvider` | 关联实体（路由与健康检查的核心行） | `models/model.go` | GORM |
| `autoassoc.Service` | 预览/执行自动关联与清理；带开关的 Trigger* | `service/autoassoc/` | 同包 |
| HTTP associate/clean | 绑定 JSON 与 `httpresp` | `handler/autoassoc/` | 调 `GetAutoAssocService` |
| `GET /model-providers/status` | 该关联最近 10 次**有成败结论**请求的成败布尔数组（按时间升序）。口径是**白名单**：只统计 `success` 与 `error`，`processing`（在途）、`canceled`（用户主动撤走）、`interrupted`（进程先走了）三态既排除出名额也不画成失败——它们都非成非败，占名额会把真实成败挤出窗口，画成失败则给出「这家供应商在失败」的假结论而供应商没做错任何事（状态取值见 `models.ChatLogStatus*`）。白名单而非逐个排除，是为了让状态机再加新态时自动落到「不统计」一侧。**返回形状仍是两态布尔**（对外契约，前端 `status-bars` 只认成败两格），故非结论态靠排除而非新增第三格——与 `health-status` 的三态字符串是两条不同的路，不要合并。回归 `handler/associations/provider_status_contract_test.go` | `handler/associations/health.go` | 同文件（ChatLog 只读聚合直读） |
| `GET /model-providers/health-status` | 最近 limit 次健康检测的状态字符串数组（三态 `success`/`error`/`not_measured`，按时间升序） | `handler/associations/health.go` | 同文件（`HealthCheckLog` 只读聚合直读） |

### `ModelWithProvider` 字段约定（路由相关）

- `MaxTokens *int`：发给上游前的输出上限裁剪阀值。`nil`/`0`/负数 = 不限，原样透传；`>0` 时超过此值会被裁剪到此上限。用于兜底客户端（如 Cursor）发超大值触发上游 400。裁剪在 `service/chat` 的 `buildRequestBodyForProvider` 中执行，passthrough 与 transform 两条路径均生效。**按协议键名穷举**：`max_tokens`（OpenAI Chat 旧名 / Anthropic）、`max_completion_tokens`（OpenAI Chat 新名）、`max_output_tokens`（OpenAI Responses）；同一 body 里出现多个则逐个钳制。漏掉某协议的键名 = 该协议上游的阀值静默空转且无报错，`max_output_tokens` 曾如此。
- `SupportsThinking *bool`：是否支持 thinking（推理）能力，**三态**——`nil`=继承 `Model.SupportsThinking`，`true`/`false`=override。解析语义见 `ModelWithProvider.SupportsThinkingResolved(model)`（纯函数：override 优先，其次继承 model，model 为 nil 时 false）。上传路径：DTO 用 `*bool` + `omitempty`；Create 直接赋值（nil 落库）；Update 在 `repos().RunInTx` 事务内执行两步——struct `Update`（其他字段部分更新，跳过零值）+ `UpdateFields` 显式写 `supports_thinking`（map 中的 nil 会写 NULL），保证原子且不破坏其余字段的部分更新语义。**注意契约**：关联 Update 中 `supports_thinking` **缺省（nil）= 显式清空 override（改回继承）**，与 Model Update 的"nil=不改"语义相反——因为该字段是三态且前端总是显式提交（inherit/true/false 之一）。
- `ToolCall / StructuredOutput / Image *bool`：三项能力 override，与 `SupportsThinking` **同范式三态**——`nil`=继承 `Model` 同名真值，`true`/`false`=override。绝大多数第三方与官方能力一致，故新建/自动关联默认 `nil`；仅当某条链路弱于（或强于）模型时才 override。
  - **为什么能力真值要同时留在 Model 层**：能力在物理上是**链路**属性而非模型属性（同一模型经不同中转时视觉/工具支持度可能不同）。只留 Model 层则无法表达「某家中转不支持视觉」；只留关联层则每加一个供应商都要人肉重填三格，且无统一真值可继承。Model 层给真值、关联层按需 override，两层缺一不可。
  - **生效值在 SQL 层解析**：`ListEnabledByModelID` 走 `JOIN models` + `COALESCE(mwp.<col>, model.<col>) = true`。禁止改回单表 `WHERE <col> = true`——那会把「继承且模型本身支持」的关联全部误杀，上层表现为请求 404 `no enabled provider` 且无日志说明真因。契约测试见 `repository/model_with_provider_capability_test.go`。
  - **上传路径**：DTO 三兄弟用 `*bool`（非 `bool`，否则无法区分「显式关闭」与「未传」）；Create 直接赋值（nil 落库 NULL）；Update 与 `supports_thinking` 一起在同一 `RunInTx` 内走 `UpdateFields` 显式写 NULL——留在 struct `Updates` 里会被 GORM 当零值跳过，override 一旦写上就再也清不掉。
  - **前端不再往 nil 写（UI 契约）**：关联弹窗用「总控下拉（继承 / 自定义）+ 三项勾选」而非三项各自三态。理由是「继承」与「显式等于模型真值」在 COALESCE 解析后**完全等价**，UI 区分它俩只是给人添麻烦。总控选「继承」时前端三个字段一律不发（→ 落 NULL）；选「自定义」才按勾选发布尔值。因此**关联侧 `nil` 现在只由「继承」产生**，前端 mock 与测试构造 `ModelWithProvider` 时，三兄弟应为 `null` 才代表继承态。能力总控的枚举（`inherit` / `custom`）与措辞与同弹窗内 `thinking_levels_mode` 刻意一致，两块不必学两套。
  - **回填的静默翻转坑（前端必须守住）**：打开弹窗时，override 为 null（继承）的项，勾选初值必须取**模型真值**而非 `false`。否则用户从「继承」切到「自定义」一保存，能力就从"跟随模型（可能全开）"变成"全禁用"，界面上看不出任何异常。回归由 `use-model-providers-association-dialog.test.tsx` 的「继承态的勾选初值取模型真值」锁住。
- `ThinkingLevels *[]string`：思考档位白名单，**三态**——`nil`=继承 `Model.ThinkingLevels`，`[]`=显式不约束（任意档位透传），非空=override 白名单。解析语义见 `ModelWithProvider.ThinkingLevelsResolved(model)`（纯函数：override 优先，其次继承 model，model 为 nil 时 nil）。GORM `serializer:json`。上传路径：DTO 用 `*[]string` + `omitempty`；Create 直接赋值（nil 落库）；Update 在同一 `RunInTx` 事务内与 `supports_thinking` 一起走 `UpdateFields` 显式写 NULL——**注意**：GORM `serializer:json` 只对 struct `Update` 生效，map-based `UpdateFields` 不走 serializer，需手动 `json.Marshal` 转 JSON 字符串（见 `models.SerializeStringSlicePtrForUpdate` 共享辅助函数，Model 侧用 `SerializeThinkingLevelsForUpdate`，`Protocols` 与 `ThinkingLevels` 共用前者）。**注意契约**：与 `supports_thinking` 一致，关联 Update 中 `thinking_levels` **缺省（nil）= 显式清空 override（改回继承）**。
- `Protocols *[]string` / `PrimaryProtocol *string`：**协议覆盖**（2026-10-19），两项各自三态——`nil` = 继承供应商那层（集合 = 供应商已启用端点集，主格式 = `ProtocolOfType(Provider.Type)`），非 nil = override。`Protocols` 空切片没有业务含义（一条协议都不许走请用 `Status` 停用整个关联），写入侧归一为 `nil`；`PrimaryProtocol` 必须落在生效集合内（集合已设时比集合，未设时比供应商已启用协议），保存时校验、运行时同一份判据（`channel.ResolveProtocolScope`）。GORM `serializer:json`；上传路径同 `ThinkingLevels`（DTO `*[]string` / `*string` + `omitempty`；Update 走 `UpdateFields` 显式写 NULL，map 更新不走 serializer，需手动 `json.Marshal`，复用 `models.SerializeStringSlicePtrForUpdate`）。**覆盖优先于供应商启停**：关联勾了被停用的端点照样用；供应商没配的协议也能勾，端点按 `Provider.Config.base_url` 合成（与协议检测同一做法）。前端关联弹窗有「继承 / 自定义」总控 + 三条勾选 + 主格式下拉；协议检测报告的「一键勾选 N 条可用协议（主格式 X）」写的就是这两项（整项覆盖，见 `test-dialog.tsx`）。**勾选集合 = 可用且未被判为转译的协议**（错误信封指向别的家族说明是上游网关在转，勾它等于把转换交给上游，故排除；指纹没采到不算转译证据，照勾）；主格式在「无法区分」时**按同一条证据口径排序**（自己该有的原生字段回来了几个 → 当前主类型 → 耗时 → 协议名），当前主类型只在打平时优先，不会出现「某条缺 2 项却仍被推荐」；集合为空（一条都不可用，或可用协议全是转译路径）时不给按钮。
- 裁剪联动：`SupportsThinkingResolved` 为 `false` 时，`service/chat` 的 `buildRequestBodyForProvider` 会删除请求体中的 `thinking`/`reasoning_effort`/`reasoning`/`output_config.effort` 字段（`stripThinkingFields` 纯函数，`output_config.effort` 删后空对象连壳删除），避免不支持 thinking 的上游报 400/静默忽略。
- 钳制联动：`SupportsThinkingResolved` 为 `true` 时，`service/chat` 的 `buildRequestBodyForProvider` 会经 `ThinkingLevelsResolved` 解析白名单，构建 `transform.ThinkingClampConfig` 并在 passthrough/transform 两条路径对 `reasoning_effort` 执行就近钳制 + budget 联动（方案 E）。详见 `protocol-transform.md` 钳制章节。

### 响应键契约（`/api/model-providers`）

`handler/associations` 的 Create / Update / List 直接序列化 `models.ModelWithProvider`，对外键名为 **PascalCase**（实体多数字段无 json tag），前端按同名 interface 消费（`webui/src/lib/api/modules/catalog/model-providers.ts`）。三态字段（`ToolCall` / `StructuredOutput` / `Image` / `WithHeader` / `Status` / `CustomerHeaders` / `MaxTokens` / `SupportsThinking` / `ThinkingLevels` / `Protocols` / `PrimaryProtocol`）的 nil 必须序列化成 JSON `null` 且键保留——键缺失会让前端把「继承」误判成 override 并回写错误值。前端 `ModelWithProvider` interface 对这批键**禁 `?` 可选标记**（optional 会放行消费方漏配）。该契约由 `crud_response_shape_test.go` 的 `modelWithProviderResponseKeys` 表在响应体 JSON 上断言：走 `testutil.AssertAPIResponseKeysInState` 断**键集合逐字相等**（少键与多键都红——实体新增字段导致的响应多键必须改表才放行，这正是「只逐键断存在」防不住的方向）。Create / Update / List 三路径各断一次键集合与**有值态**方向（`KeyStateSet`：三态键为 `null` 即红）；**空值态**（`KeyStateUnset`：三态键有值即红）**只在 List 路径断一次**——Create/Update 走 handler 写不出「全继承」行（`WithHeader` / `CustomerHeaders` / `Status` 必被赋非 nil），该行由 repository 直造。表内已含 `gorm.Model` 的 `CreatedAt` / `UpdatedAt` / `DeletedAt` 三键（本实体嵌 `gorm.Model`，`models.Model` 则没有 `DeletedAt`）；`DeletedAt` 标 `AlwaysNull`（存活行恒 null，不参与方向校验，有值即红）。实体增删业务字段时同步加/删一行即可。

> ⚠️ **同名能力语义在两个实体上键名相反**：`ModelWithProvider.ToolCall` 是 PascalCase（无 tag），`Model.tool_call` 是 snake_case（显式 tag）。跨实体复制字段时逐一核对，前端 interface 亦然。

### `provider_model` 写入闸（上游模型名非空）

`provider_model` 是关联的**定义性字段**——它直接进出站 body 的 `model`，空值必被上游 400，而该 400 不被 `classifyCredentialFailure` 认作凭据级失败，会落组织级失败分支：候选淘汰 + 衰减 + 失败记在供应商头上（极端情况自动停用该关联），真因却是本地表单留空。`BuildReq` 无任何兜底路径，「不指定上游名」不是合法配置形态，故**不给兜底，直接拒**。

闸的判据是 **`TrimSpace` 后为空**，不是 `binding:"required"`——后者只认空串，`"   "` 能过；而**全空格恰恰是 Update 侧唯一能真正写进库的形态**（struct `Updates` 跳过空串零值=原值保留，全空格非零值照写）。两路径语义按 DTO 的三态 (`ProviderModel *string`) 分流：**Create 必填**（nil 或 trim 后为空 → 400），**Update 缺省 (nil) = 不改**（部分更新语义，与其余字段一致），传了但 trim 后为空 → 400。闸必须落在后端：Create 的空串只被前端 toast 拦了一道，直接调接口与 `handler/importexport`（整批写零值行，不映射该字段）都绕得开。

拒绝经 `httpresp.BadRequest`（HTTP 200 + 信封 code=400），被拒不落库、不改旧值。契约测试：`crud_provider_model_guard_test.go`（Create 空串/全空格/键缺失三态、Update 全空格拒 + 省略保留 + 部分更新仍生效）。存量脏行按开发阶段原则直接修数据，**禁止**为兼容写发送侧适配层。

### 删除语义（硬删，2026-10-12）

`ModelWithProviderRepo` 的四个删除方法（`Delete` / `DeleteByIDs` / `DeleteByProviderID` / `DeleteByModelID`）一律 `Unscoped()` **硬删**，与 `pools` / `credentials` / `virtual_models` / `api_keys` 口径统一。

与号池那张表不同，**本表无唯一索引**，软删残留不会挡同名重建——所以这条改动治的不是 bug，是**纯包袱**：软删只打 `deleted_at` 标记、行永远留在库里，而本表没有任何读取面能看见它们（无恢复入口、无回收站、`handler/database` 统计页与列表页均走软删作用域，健康检测日志与请求日志存的是**名称快照**而非回查 ID，也无外键指向本表），唯一效果是每删一次累积一行看不见的数据。回归 `TestModelWithProviderRepo_DeleteIsHard`（四个方法各断一次，判据打在 `Unscoped` 物理行数上——软删行的「不可见」正是它的伪装，普通计数改前改后都是 0）。

> 同批改动的还有 `KeyGroupRepo.Delete` 与 `EndpointRepo.Delete`（同理由，见 [channel-routing.md](channel-routing.md)）。

### `customer_headers` 写入闸（4a，与 api-keys 同源）

Create / Update 对 `req.CustomerHeaders` 走 `common.SanitizeOutboundHeaders`（`common/outboundheaders.go`），与 api-keys key 级头**同一份净化器**、两域口径禁止分叉：① 名规范化为 MIME Title-Case（存储/回显/发送一套样子）；② 值拒 CR/LF/NUL 等控制字符；③ 规范化后撞名整体拒绝（Go map 迭代序随机，「后者赢」对 map 输入不可确定，唯一确定语义是拒）。这是语法关不是头名黑名单——Cookie / Authorization 等合法名照放，但凭据头最终由 auth_type 单源执法（见 chat-gateway.md / providers.md），配置压不动。拒绝经 `httpresp.BadRequest`（HTTP 200 + 信封 code=400，message 指明哪条头与原因），被拒不落库、不改旧值。存量脏行按开发阶段原则直接修数据，**禁止**为兼容写发送侧适配层。前端两弹窗折叠共用 `@/lib/header-rows`（`rowsToHeaders` / `headersToRows`），只校验到形状层（空名行跳过不打断输入），语法权威在服务端。契约测试：`customer_headers_guard_test.go`（HTTP 层断 400、不落库、规范化回显）。

## 5. 特殊约定

- chat 与 healthcheck 将关联行视为选路与启停的数据源；本模块是配置写入口
- **自动关联单一入口（已收敛）**：
  - 业务：`service/autoassoc`（`Associate` / `CleanInvalid` / `Preview*` / `Trigger*IfEnabled`）
  - HTTP：`handler/autoassoc` 只做绑定与响应
  - Provider CRUD：`TriggerAutoAssociate` / `TriggerAutoClean` → service
  - 同步后：`modelsync.ActionHooks` → `service.GetAutoAssocService().Associate/CleanInvalid`（开关在 `triggerAutoActions`）
  - 模板匹配：`service/model_template.go` 的 `TemplateIndex`（经 `autoassoc_facade` 注入 `BuildIndexFunc`）
- 规则权威：
  - 尊重 `Model.AutoAssociate`（**二态 bool**：`false` 跳过；Create 缺省由 handler 兜底为 true，无「未设置」态——该字段曾声明 `*bool` 三态但从未可表达，已收回，见 `models.Model` 字段注释）
  - 跳过 `Provider.Blacklisted`
  - 默认 `Weight` 读 `SettingKeyAutoWeightDecayDefault`、`Priority` 读 `SettingKeyAutoPriorityDecayDefault`（schema 默认 100/100；缺失/非法回退到 `DefaultWeightFallback`/`DefaultPriorityFallback` 常量，由调用方 `service.associate` 传入 `NewDefaultAssociation`）
  - 无效关联：provider 不存在，或 `ProviderModel` 不在组织级目录（各分组白名单并集 ∪ `custom_models`）；`GetProviderModels` 读库失败则不删。该目录与生产选路的可服务判定集**同集**（2026-09-28 起 `custom_models` 进选路判定，供应商级授权、全部分组可服务），清理判「无效」与生产判「无组可服务」不会互相矛盾

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日收敛 autoassoc 回写。*
