# pools 模块（号池 / 凭据 / 端点 / 分组）

> 供应商凭据池体系。权威功能设计见 `.local/discussions/provider-channels/04-设计定案.md`（实施顺序见其中第 10 节 stage 表）。全局架构见 [README.md](../README.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 供应商号池（凭据容器）管理：号池 CRUD、凭据批量导入、按凭据测试；协议端点与凭据分组为选路提供结构化数据 |
| 目录位置 | `handler/pools/`；`repository/pool.go`、`credential.go`、`endpoint.go`、`key_group.go`；`models/channel.go`（四表）+ `models/migrate_channels.go`（存量迁移）；`common/credentialcrypto/` |
| 主要职责 | 将「凭据」从供应商实体拆出为可复用号池；协议端点与凭据分组为选路提供结构化数据（S3 起消费）。凭据自身的属性与动作（状态/启停/备注/明文/列表）见 [credentials.md](credentials.md) |

## 2. 职责与边界

- **对外暴露**：`handler/pools` REST（`/api/pools` CRUD + 列表健康统计；三个池作用域动作：`POST /api/pools/auto/import` 建专属池并导凭据、`POST /api/pools/:id/credentials/batch/import` 批量导入、`POST /api/pools/:id/credentials/:credId/test` 按凭据测试）；`PoolRepo` / `EndpointRepo` / `KeyGroupRepo`（`CredentialRepo` 已归凭据域，见 [credentials.md](credentials.md)）
- **依赖谁**：`repository`（经 `repos()`）；`models` 四表实体；凭据加解密 `common/credentialcrypto`（main 装配 `SetDefault` 注入）；`#24-B 按凭据测试` handler 另依赖 `service/channel`（`NewClientFromSelection`/`Assemble`/`SelectPrimaryEndpoint` 装配指定凭据的 client）+ `service/credprobe`（`RunCompletionProbe`：completion 探针编排的单一来源，含 BuildReq→Do→归类→细节提取，与 testapi 各测试端点共用；`DefaultTimeout` 亦出自本包）+ `common`（`ErrorTypeFromStatus`/`UpstreamErrorDetail` 归 error_type 与失败细节，与 testapi 单一来源共用），依赖方向 handler→service→providers 合法
- **不负责什么**：上游请求构造（providers）、选路/故障转移（S3 起在 chat 层）、健康状态机（S4）

## 3. 内部结构

```
handler/pools/           # 号池管理 API 域（池 CRUD + 健康统计 + 批量导入 + 按凭据测试）
  routes.go              # Register：/api/pools GET/POST/PUT/:id/DELETE/:id + /auto/import（静态段，建专属池并导凭据）+ /:id/credentials/batch/import + /:id/credentials/:credId/test（凭据自身的读/写在 handler/credentials）
  crud.go                # GetPools(列表+统计) / CreatePool / UpdatePool / DeletePool
  auto_pool.go           # CreateAutoPoolWithCredentials（供应商分组「添加 Key」的一步式落库：建 auto_created 专属池 + 复用 importCredentialRows 导凭据）+ createAutoPoolWithUniqueName（撞 pools.name 唯一索引就换 " (n)" 后缀重试，不做先查后写的预检）+ autoPoolIdentity（池名与备注的服务端生成口径）
  pool_guard.go          # requirePool（池存在性统一校验）+ getCredentialInPool（池作用域越池判定，按凭据测试用）
  credential_import.go   # BatchImportCredentials（批量粘贴导入）+ importCredentialRows + maxCredentialImportBatch 上限
  credential_probe.go    # TestCredential（#24-B 按凭据测试：指定单条凭据 channel.NewClientFromSelection 装配，探针编排走 credprobe.RunCompletionProbe，只报不改）+ 链路解析 + 失败回包（error_type/文案由 credprobe+common 单一来源产出）
  types.go               # PoolRequest / PoolListItem / PoolRefGroup + CredentialTestResult（#24）+ 批量导入 DTO + AutoPoolImportRequest/AutoPoolImportData
  repos.go               # repos() 访问 repository.Default()
repository/
  pool.go                # PoolRepo + StatsByIDs（GROUP BY 聚合健康统计）+ UpdateFields + ErrPoolNameTaken（撞 pools.name 唯一索引的哨兵错误，与 ErrApiKeyExists 同口径）
  credential.go          # CredentialRepo：ListPaged/Get/Create/UpdateFields/UpdateFieldsByIDs(按 ID)/Delete/DeleteByIDs/DeleteByPoolID/ListByPools(装配侧收敛)/ExistingHashes/RecordCredential*/IncrementFailCountAndStopIfThreshold/IncrementFailCountNoStop（Stage 33 阈值读失败只计数不判停）等（契约与消费方见 credentials.md）
  endpoint.go            # EndpointRepo
  key_group.go           # KeyGroupRepo + CountByPoolIDs（引用守卫）+ ListByPoolIDs（引用明细）+ UpdateFields
models/
  channel.go             # Pool / Credential / Endpoint / KeyGroup 四表实体
  migrate_channels.go    # 存量 Provider 迁移（建专属号池 + 凭据入池 + 回填分组 PoolID，api_key 加密搬移）
common/credentialcrypto/ # AES-256-GCM 加密 + KeyHash + MaskKey 掩码 + 密钥引导
service/credview/        # 凭据对外视图单源（号池列表项与按凭据 ID 的动作视图共用）
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `PoolRepo.StatsByIDs` | 一条 GROUP BY 返回多号池凭据统计（key 数 + 状态分布），防 N+1 | `repository/pool.go` | 同文件 |
| `CredentialRepo.DeleteByPoolID` | 级联**硬删**号池下凭据；被分组引用的池有删除守卫（`KeyGroupRepo.CountByPoolIDs`），删除分组本身不级联删池内凭据 | `repository/credential.go` | 同文件 |
| `CredentialRepo.UpdateFieldsByIDs` | 按字段 map 批量更新指定 IDs（`WHERE id IN`，**无归属限界**——凭据是第一等资源，归属过滤只在列表查询侧）；批量启停经此与恢复重置字段（`models.CredentialRecoveryFields`）共用 | `repository/credential.go` | 同文件（消费方 `handler/credentials`） |
| `CredentialRepo.DeleteByIDs` | 批量**硬删**指定 IDs 凭据（同上，无归属限界） | `repository/credential.go` | 同文件 |
| `CredentialRepo.ExistingHashes` | 批量导入查重：返回号池下现存凭据命中的 KeyHash 集合，一次查询防 N+1 | `repository/credential.go` | 同文件 |
| `CredentialRepo.ListByPools` | 装配侧收敛查询：`pool_id IN poolIDs`，按 `id ASC`；空集合返回 nil 不产生 SQL（无引用池即无候选凭据，不报错）。**凭据只归属号池**（2026-10-07 删内联凭据概念，原 `ListByGroups` 的 `group_id IN` 分支与 `Credential.GroupID` 一并删除） | `repository/credential.go` | 同文件 |
| 凭据终身统计 | `credentials.total_requests/total_errors/last_used_at` **只由 chat 真实流量写入**：成功尝试（上游 200 被接受；流式中途断流不回拨）→ `total_requests`+1 且 `last_used_at`=成功时刻；凭据级失败（401/403/429/5xx/网络超时，与冷却/判停同一 `classifyCredentialFailure` 判定域）→ `total_errors`+1；组织级出口（能力 skip、转换失败、其余 4xx）不记账；探针/探活/健康检查/testapi 走诊断口径不进 attempt，结构性零记账（与 #24「只报不改」同族，`credential_test_link_test.go` 有统计列断言锁）。落库经 `credwrite` 队列 `KindRecordSuccess/KindRecordError` 异步执行（热路径不等待 DB；drop-oldest 容忍极端背压下极小丢失，`DroppedCount` 可观测）。终身累计：不随人工恢复/启停清零（`CredentialRecoveryFields` 只清连败瞬态），历史不回填。号池详情弹窗以单列「统计」渲染 `请求 N · 失败 M · 相对时间`（null → 「从未使用」） | `service/chat/chat_credential_stats.go`（写点）+ `repository/credential.go`（落库原语） | `webui` `credential-stats-cell.tsx` + `formatCredentialLastUsed`（`components/credentials/credential-status.ts`） |
| 批量导入语义 | `POST /api/pools/:id/credentials/batch/import` `{"keys":[...]}`：批内首现 + 池内 `KeyHash` 双重去重（判定顺序：批内首现先于池内——同 key 批内重复∧池内已有时，首现行 duplicate_in_pool、重复行 duplicate_in_batch）；行级失败隔离（逐行独立落库，无共享事务；单行加密/落库异常收集为 failed 不阻断其余行）；逐行回显 `rows`（`Index`/掩码 `Key`/`Status` ∈ imported|skipped|failed/`Reason` 机器码 duplicate_in_batch、duplicate_in_pool、empty、encrypt_failed、db_failed）；单请求上限 500，超限 400；空数组 400；池不存在 404；`Default()==nil` 500 | `handler/pools/credential_import.go` `BatchImportCredentials` | `importCredentialRows`（加密/落库失败经注入桩覆盖测试） |
| 自动池语义（供应商分组「添加 Key」一步落库） | `POST /api/pools/auto/import` `{"provider_name":"供应商名","group_name":"分组名","keys":[...]}`：**建一个专属号池 + 立刻把 Key 导进去**，一次请求完成；响应 `{"pool_id","pool_name","auto_created","key_count","total","imported","skipped","failed","rows"}`（`rows` 与批量导入同源；`auto_created`/`key_count` 供前端把新池并进号池列表缓存，不等列表 refetch 才认得它是专属池）。建出的池带 `auto_created=1`（号池页据此折叠成单独一块）。**重名不报错**：直接建池，撞 `pools.name` 唯一索引就换 `" (2)"`、`" (3)"` 序号后缀重试（不做「先查后写」的预检——预检与写入之间有窗口，并发同名会双双通过预检再撞索引报 500；uniqueIndex 才是单一真相）；候选名用尽（20 个变体全占满）→ 400 业务条件，不是 500。复用已存在的池 = 往别人的池里塞凭据，绝不能做。**命名与备注口径在服务端**（`autoPoolIdentity`）：池名恒为「供应商名·分组名」、备注恒为「供应商「X」的分组「Y」自动创建」；前端只传两个原语，不参与命名（同一规则两处实现必然漂移）。供应商名/分组名**不校验非空**——用户还没填完就点「添加 Key」是正常流程，空名走可读兜底（未命名供应商 / 未命名分组），真正的归属由 `key_groups.pool_id` 承载，池名只是给人看的标识。入参与上限校验同批量导入（空 keys/超 500 → 400；`Default()==nil` → 500）；**行级失败不回滚已建的池**（行级隔离语义一致，失败行回显原因，用户可在号池页补加），但**整批失败会删掉刚建的空池**（`importCredentialRows` 唯一 error 出口在写入前 → 一行都没写 → 删池不留孤儿凭据，该前提由 `auto_pool_test.go` 的「库内凭据数为 0」断言锁住）。供应商保存路径**零改动**——照旧只提交 `{id,name,weight,pool_id}` | `handler/pools/auto_pool.go` `CreateAutoPoolWithCredentials` | 复用 `importCredentialRows` + `createAutoPoolWithUniqueName`（`models.Pool.AutoCreated` 承载标记） |
| 自动池归属标记 | `models.Pool.AutoCreated bool`（二态，非三态）：标记该池由「供应商分组→添加 Key」自动创建。**归属不记 group id**——池在分组保存之前就建好，那时分组还没落库，指不到具体分组；归属靠 `key_groups.pool_id` 反查（`PoolListItem.RefGroups` 已有供应商名+分组名）。手建池（号池页「新建号池」）恒为 `false`，`CreatePool` 不接受该字段 | `models/channel.go` | `handler/pools/auto_pool.go`（写）+ `handler/pools/crud.go` `GetPools`（读，直返实体 PascalCase，键名 `AutoCreated`） |
| `KeyGroupRepo.CountByPoolIDs` | 号池被分组引用计数（删除守卫依据） | `repository/key_group.go` | 同文件 |
| `credentialcrypto.MaskKey` / `MaskStored` | 掩码规则单一来源（前缀3+`****`+后缀4，≤8 全 `****`）；`MaskStored` 统一读路径降级策略——cipher 未配置/解密失败一律 `****` + `slog.Warn`（绝不输出密文片段）。号池凭据列表、批量导入结果、选路探针与 `service/credview` 的凭据对外视图共用（供应商侧已无凭据视图，2026-10-07 删内联凭据），改降级规则只动这一处 | `common/credentialcrypto/cipher.go` | `handler/credentials`、`handler/pools`、`service/channel`、`service/credview` 调用 |
| `KeyGroupRepo.ListByPoolIDs` | `WHERE pool_id IN ?` 批量返回引用分组（id/provider_id/pool_id 现存值）；GetPools 组装 RefGroups 明细用，照 `ListByProvider` 实体查询模式 | `repository/key_group.go` | 同文件 |
| `PoolListItem` | `/api/pools` 列表项：内嵌 `models.Pool`（PascalCase 直返，含 `AutoCreated`）+ `KeyCount`/`StatusCounts`/`ReferencedBy`/`RefGroups` | `handler/pools/types.go` | `handler/pools/crud.go` |
| `RefGroups` 引用明细 | 列表项字段 `[]PoolRefGroup{ProviderID, ProviderName, GroupID, GroupName}`（PascalCase）；未被引用时**显式空数组**（非 null、非键缺失）；`ReferencedBy` 计数由明细长度派生（单一数据源，不二次查询）；供应商已删但分组残留时 `ProviderName` 空串、条目保留（DeleteProvider 不级联删分组）；两步组装（分组列表 → ProviderRepo.ListByIDs 名称映射）不引入 JOIN 投影。**前端现状零消费**（#17 深链跳转取消，明细为惰性数据） | `handler/pools/types.go` / `crud.go` | `KeyGroupRepo.ListByPoolIDs` + `ProviderRepo.ListByIDs` |
| 凭据创建语义 | 明文 Key → `Encrypt`+`Hash` 落库密文；池内 `KeyHash` 去重（跨池同 key 允许复用）；`Default()==nil` 时 500 拒绝（不落明文）。端点在凭据域：`POST /api/credentials` | `handler/credentials/crud.go` `CreateCredential` | `credentialcrypto.Default` |
| 凭据单条访问 | 单查/更新/删除/raw 在凭据域按 ID 直取（`/api/credentials/:id…`，无归属限界）；raw 解密失败 `slog.Warn` + 500（不回退密文） | `handler/credentials/crud.go` | 同文件 |
| #24-B 按凭据测试 | `POST /api/pools/:id/credentials/:credId/test`：用**指定这一条**凭据打一次**真实最小 completion**（内容 "hi"，状态码通过判定=passed，口径单源 `common.IsPassingStatus`：200/201），**只报不改**（绝不写回 `Status`/`FailCount`/`Cooldown`）。装配点 `channel.NewClientFromSelection`（唯一「喂指定凭据」入口），**不用** `ProbeGroup`/`FirstUsableCredential`（那是组内首个可用，会把坏 key 误判成隔壁好 key 的健康）。探针体 `credentialProbeBody` 按端点协议分发（openai/responses/anthropic 三形状，anthropic/responses 带极小 max 预算，model 由 `BuildReq` sjson 注入）。**测试模型来自池级 `Pool.TestModel`**（2026-09-10 用户拍板从分组迁到池：点测入口在号池详情，配置与测试动作同屏；原 #24-B Q1 组级定案作废，`KeyGroup.TestModel` 已删，**存量组级配置全部作废不做迁移——升级后需按池重配**）——空则**直接判 `error_type=validation`、不装配/不打上游**（Q4：不回退硬编码默认模型，否则未授权模型 404 会伪装成「key 坏」）。端点取「引用该池的分组里 `(ProviderID, GroupID)` 最小者」的**主类型协议**端点（`SelectPrimaryEndpoint(provider.Type,…)`=`ProtocolOfType(Provider.Type)`；一协议一行端点由 `structured.go` 保证无歧义）——**只测主类型那条协议**，其余 `Provider.Protocols` 勾选协议不测（多协议全测=独立 TODO）。响应回显 `group_id`/`group_name`/`provider_id` + 实际所用 `model`。响应恒 `code 200`，结论在 `data.passed`（与 testapi 四端点同族，前端 `apiRequest` 在 code≠200 时丢 data）；链路配不出（无引用组 / 供应商已删 / 无启用端点 / 解密装配失败）→ `passed:false + error_type=provider`；打了 completion 被拒 → `auth`/`validation`/`provider`、连接失败/超时 → `network`/`timeout`——归类、失败文案与细节提取全部来自 **`credprobe.RunCompletionProbe`**（状态码映射 `common.ErrorTypeFromStatus`、细节 `common.UpstreamErrorDetail`；2026-09-12 completion 探针编排自各 handler 收敛至此，**取代旧 `/models` 版 `classifyProbeError` 靠错误文本 substring 判 401 的退化**）。越池/池不存在经 `getCredentialInPool` 回 404（零上游请求）。**单条点测**，不做批量测整池、不做多组端点选择器（均刻意收敛） | `handler/pools/credential_probe.go` | 前端 `useCredentialTesting` + `credential-test-button`（tooltip 按「结论 / 链路 / 失败详情」分行陈述，链路回显命中分组 + 所用模型，宽度由 `ui/tooltip` primitive 统一限宽；桌面/移动两处凭据行）；池级 test_model 编辑在 `pool-form-dialog.tsx`（请求体 `test_model`，全量 PUT 缺省即清空） |
| 前端明文显示/复制 | 号池详情每行「显示 / 复制」经 `GET /api/credentials/:id/raw` 取明文。明文生命周期是安全边界：只驻留 `useCredentialKeyReveal` 的内存 state，**不进 TanStack Query 缓存**（缓存会长期留存明文且无 invalidate 语义）；`view`（页码/筛选/搜索）或 `active`（弹窗开合）一变即整体作废、在途响应按世代号丢弃（旧世代的收尾动作也不得动新视图的 pending，否则双击守卫失效）；隐藏即删明文（再显示重新拉）；**删除凭据后调用方必须 `forget(ids)`**——被删的 key 往往正因为已泄露，行从 DOM 消失了明文就不能继续躺在 state 里；复制**不改**显示态（未显示时静默取一次），走 `copySecretWithToast`（禁 prompt 兜底），成功提示只回显 `KeyMasked`；明文态解除限宽、折行显示全量并标 `translate="no"`（浏览器整页翻译会上传可见文本） | `webui/src/components/number-pools/use-credential-key-reveal.ts` | `credential-key-cell.tsx`（桌面表格与移动卡片共用一个单元格组件）+ `lib/clipboard.ts` |
| 明文响应禁缓存 | 受控明文端点一律用 `httpresp.SuccessNoStore`（= `Cache-Control: no-store` + Success，合并成一个函数是为了让「返回明文」和「禁缓存」不可能只做一半），前端 raw 请求另带 `{cache:"no-store"}` 构成纵深防御。明文落进浏览器磁盘缓存或中间代理，前端整套生命周期守卫就形同虚设——缓存里的响应体比内存 state 活得久且不受前端管辖 | `httpresp/response.go` | `handler/credentials/crud.go` `GetCredentialRaw` |
| 号池删除语义 | 被分组引用 → 400 拒绝；未引用 → 单事务级联**硬删**凭据 + 号池本体。**硬删而非软删**：`pools.name` 唯一索引不含 `deleted_at`，软删行不可见却仍占名字——「删池后同名重建」会先过 `ExistsByName` 预检、再撞唯一索引，回 500 `Failed to create pool`，报错方向全错。号池是全库唯一「有唯一索引却软删」的表，此处起与 `virtual_models` / `api_keys` 口径统一 | `handler/pools/crud.go` `DeletePool` | `Repositories.RunInTx` |
| `credentialcrypto.Default` | 凭据加解密包级入口（main 装配注入，早于 `models.Init`） | `common/credentialcrypto/` | `main.go` |

## 5. 特殊约定

- **仓储层不解密**：`credentials.Key` 在 models/repository 层是密文原样存取；解密/掩码是上层（S2 API / S3 调度）职责
- **凭据只存加密形态**：`Config.api_key` 明文兜底链已废除，存量迁移加密搬移后清空
- **三态字段**（`Credential.CooldownUntil/LastUsedAt/LastProbeAt`、`KeyGroup.PoolID`）为指针，nil 序列化 JSON `null`（禁 omitempty）；清除必须走 `UpdateFields`（map 显式写 NULL），struct Update 跳过零值。`Credential.PoolID` 是普通 `uint`（凭据必属号池），`Credential.GroupID` 已于 2026-10-07 随内联凭据概念一并删除
- **`LastProbeAt`**：#6-3 惰性探活频控字段（成败都更新）；凭据视图 `credview.View` 已暴露；恢复 active 不经 `CredentialRecoveryFields` 清该字段
- **`StatusCounts` 用 map 而非固定字段**：S4 新增状态（如 temp_unsched）不改响应结构，前端按状态配置遍历
- **两套"失败"语义并存勿混**：`FailCount`=鉴权**连败**瞬态计数（成功/恢复清零，驱动判停状态机；2026-10-21 起号池详情凭据状态格会显示它，与 `CooldownReason`/`LastProbeAt` 一起回答「为什么不可用、坏了多久」）；`TotalErrors`=凭据级失败**终身**累计（UI「统计」列的「失败」）。行为处置读前者，两者都展示但含义不同
- **未做**：号池列表分页（数量级小）；`GET /api/pools/:id` 详情端点（列表项携带足够）；批量导入逐行独立落库（无共享事务：500 条 ≈ 2s，低频粘贴可接受；单事务改造需扩仓储接口，未做）；`(pool_id, key_hash)` 无唯一约束，并发导入同 key 双插竞态与单条创建同语义（#11 起接受，不做复合唯一索引）
- **自动池与手建池分区**：号池页按 `AutoCreated` 分两块——手建池走主列表，专属号池（供应商分组「添加 Key」时自动建出）折叠在下方单独一块（默认折叠；没人引用的孤儿专属池在这里标「未引用」，桌面表格与移动列表都标，由用户手动删）。**不常驻清理**：专属池可能因用户取消供应商弹窗而变孤儿，「当前代码仍可能产生」故不做启动时扫描抹除，只提供可视化入口让用户自己删
- **术语**：用户可见面统一叫「专属号池」（创建时用户看到的就是这个词），对照的一侧叫「手建号池」；代码与文档里不另用「自动池」这个简称
- **自动池测试模型留空**：自动池不预置 `TestModel`（建池时无从得知该组会用哪个模型），按凭据测试会判 `error_type=validation`；需要点测的自动池由用户在号池页配测试模型。供应商分组卡片上给出该提示
- 契约测试：`handler/pools/crud_test.go`、`credential_import_test.go`、`auto_pool_test.go`、`credential_probe_test.go`、`credential_test_link_test.go`、`routes_test.go` 断言打在响应体 JSON（gjson `Exists()` 区分键存在与 null）；凭据自身的 CRUD/启停契约测试在 `handler/credentials/`；仓储测试在 `repository/channel_crud_test.go`
- 路由注册顺序：本域 `/auto/import` 静态段与 `/:id/credentials/batch/import` 参数子树**同层共存**（gin 静态段优先匹配，二者互不捕获，`TestAutoPoolImport_RouteNotShadowingBatchImport` 锁定）；`/:id/credentials/batch/import` 固定段与 `/:id/credentials/:credId/test` 参数子树共存互不遮蔽（`TestRegisterRouteOrder_ImportNotShadowedByCredIDTest`）；凭据域的 `/credentials/status`、`/credentials/batch` 固定段先于 `/credentials/:id` 契约见 [credentials.md](credentials.md)

---

*本文档随 provider-channels S2 落地（2026-08-29）首次建立；S3–S7 扩展选路/状态机/同步后同步更新。*
