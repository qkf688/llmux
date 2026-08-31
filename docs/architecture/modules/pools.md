# pools 模块（号池 / 凭据 / 端点 / 分组）

> 供应商凭据池体系。权威功能设计见 `.local/discussions/provider-channels/04-设计定案.md`（实施顺序见其中第 10 节 stage 表）。全局架构见 [README.md](../README.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 供应商号池（凭据池）管理：号池 CRUD、凭据批量导入/启停、协议端点、凭据分组 |
| 目录位置 | `handler/pools/`；`repository/pool.go`、`credential.go`、`endpoint.go`、`key_group.go`；`models/channel.go`（四表）+ `models/migrate_channels.go`（存量迁移）；`common/credentialcrypto/` |
| 主要职责 | 将「凭据」从供应商实体拆出为可复用号池；协议端点与凭据分组为选路提供结构化数据（S3 起消费） |

## 2. 职责与边界

- **对外暴露**：`handler/pools` REST（`/api/pools` CRUD + 列表健康统计）；`PoolRepo` / `CredentialRepo` / `EndpointRepo` / `KeyGroupRepo`
- **依赖谁**：`repository`（经 `repos()`）；`models` 四表实体；凭据加解密 `common/credentialcrypto`（main 装配 `SetDefault` 注入）
- **不负责什么**：上游请求构造（providers）、选路/故障转移（S3 起在 chat 层）、健康状态机（S4）

## 3. 内部结构

```
handler/pools/           # 号池管理 API 域（S2：CRUD + 健康统计 + 凭据单条/批量操作；S6 前端接入）
  routes.go              # Register：/api/pools GET/POST/PUT/:id/DELETE/:id + /:id/credentials 系列
  crud.go                # GetPools(列表+统计) / CreatePool / UpdatePool / DeletePool
  credentials.go         # ListCredentials（分页/筛选/搜索+掩码）+ maskKey / requirePool（池存在性统一校验）
  credential_crud.go     # CreateCredential / GetCredential / GetCredentialRaw / UpdateCredential / DeleteCredential
  credential_batch.go    # BatchUpdateCredentialStatus / BatchDeleteCredentials / BatchImportCredentials（批量粘贴导入）+ maxCredentialImportBatch 上限
  types.go               # PoolRequest / PoolListItem / CredentialListItem + 凭据请求/批量 DTO
  repos.go               # repos() 访问 repository.Default()
repository/
  pool.go                # PoolRepo + StatsByIDs（GROUP BY 聚合健康统计）+ UpdateFields
  credential.go          # CredentialRepo + DeleteByPoolID + UpdateFieldsByIDs/DeleteByIDs（池内限界）+ ExistingHashes（批量导入查重）+ UpdateFields
  endpoint.go            # EndpointRepo
  key_group.go           # KeyGroupRepo + CountByPoolIDs（引用守卫）+ UpdateFields
models/
  channel.go             # Pool / Credential / Endpoint / KeyGroup 四表实体
  migrate_channels.go    # 存量 Provider 迁移（生成 1 端点+1 分组+1 内联凭据，api_key 加密搬移）
common/credentialcrypto/ # AES-256-GCM 加密 + KeyHash + 密钥引导
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `PoolRepo.StatsByIDs` | 一条 GROUP BY 返回多号池凭据统计（key 数 + 状态分布），防 N+1 | `repository/pool.go` | 同文件 |
| `CredentialRepo.DeleteByPoolID` | 级联删除号池下凭据（软删）；分组内联凭据不受影响 | `repository/credential.go` | 同文件 |
| `CredentialRepo.UpdateFieldsByIDs` | 按字段 map 批量更新同一号池下 IDs（`WHERE pool_id AND id IN`，越池 ID 不命中）；批量启停经此与恢复重置字段（`models.CredentialRecoveryFields`）共用 | `repository/credential.go` | 同文件 |
| `CredentialRepo.DeleteByIDs` | 批量软删同一号池下 IDs 凭据（池内限界，防越池误删） | `repository/credential.go` | 同文件 |
| `CredentialRepo.ExistingHashes` | 批量导入查重：返回号池下现存（未软删）凭据命中的 KeyHash 集合，一次查询防 N+1 | `repository/credential.go` | 同文件 |
| `CredentialRepo.ListByGroups` | 装配侧收敛查询：`group_id IN groupIDs OR pool_id IN poolIDs`，按 `id ASC`；两组皆空返回 nil 不产生 SQL；凭据 GroupID/PoolID 二选一，绝不双计（S3 选路热路径用，避免全表扫描） | `repository/credential.go` | 同文件 |
| 批量导入语义 | `POST .../batch/import` `{"keys":[...]}`：批内首现 + 池内 `KeyHash` 双重去重（判定顺序：批内首现先于池内——同 key 批内重复∧池内已有时，首现行 duplicate_in_pool、重复行 duplicate_in_batch）；行级失败隔离（逐行独立落库，无共享事务；单行加密/落库异常收集为 failed 不阻断其余行）；逐行回显 `rows`（`Index`/掩码 `Key`/`Status` ∈ imported|skipped|failed/`Reason` 机器码 duplicate_in_batch、duplicate_in_pool、empty、encrypt_failed、db_failed）；单请求上限 500，超限 400；空数组 400；池不存在 404；`Default()==nil` 500 | `handler/pools/credential_batch.go` `BatchImportCredentials` | `importCredentialRows`（加密/落库失败经注入桩覆盖测试） |
| `KeyGroupRepo.CountByPoolIDs` | 号池被分组引用计数（删除守卫依据） | `repository/key_group.go` | 同文件 |
| `PoolListItem` | `/api/pools` 列表项：内嵌 `models.Pool`（PascalCase 直返）+ `KeyCount`/`StatusCounts`/`ReferencedBy` | `handler/pools/types.go` | `handler/pools/crud.go` |
| 凭据创建语义 | 明文 Key → `Encrypt`+`Hash` 落库密文；池内 `KeyHash` 去重（跨池同 key 允许复用）；`Default()==nil` 时 500 拒绝（不落明文） | `handler/pools/credential_crud.go` `CreateCredential` | `credentialcrypto.Default` |
| 凭据单条访问 | 单查/更新/删除/raw 均校验 `cred.PoolID == :id`，越池一律 404；raw 解密失败 `slog.Warn` + 500（不回退密文） | `handler/pools/credential_crud.go` | 同文件 |
| 号池删除语义 | 被分组引用 → 400 拒绝；未引用 → 单事务级联删凭据 + 号池（软删） | `handler/pools/crud.go` `DeletePool` | `Repositories.RunInTx` |
| `credentialcrypto.Default` | 凭据加解密包级入口（main 装配注入，早于 `models.Init`） | `common/credentialcrypto/` | `main.go` |

## 5. 特殊约定

- **仓储层不解密**：`credentials.Key` 在 models/repository 层是密文原样存取；解密/掩码是上层（S2 API / S3 调度）职责
- **凭据只存加密形态**：`Config.api_key` 明文兜底链已废除，存量迁移加密搬移后清空
- **三态字段**（`Credential.GroupID/PoolID`、`KeyGroup.PoolID`）为 `*uint`，nil 序列化 JSON `null`（禁 omitempty）；清除必须走 `UpdateFields`（map 显式写 NULL），struct Update 跳过零值
- **`StatusCounts` 用 map 而非固定字段**：S4 新增状态（如 temp_unsched）不改响应结构，前端按状态配置遍历
- **未做**：号池列表分页（数量级小）；`GET /api/pools/:id` 详情端点（列表项携带足够）；引用分组名明细（S6 前端需求）；批量导入逐行独立落库（无共享事务：500 条 ≈ 2s，低频粘贴可接受；单事务改造需扩仓储接口，未做）；`(pool_id, key_hash)` 无唯一约束，并发导入同 key 双插竞态与单条创建同语义（#11 起接受，不做复合唯一索引）
- 契约测试：`handler/pools/crud_test.go`、`credentials_test.go`、`credential_crud_test.go`、`credential_batch_test.go`、`credential_import_test.go` 断言打在响应体 JSON（gjson `Exists()` 区分键存在与 null）；仓储测试在 `repository/channel_crud_test.go`
- 路由注册顺序：`/pools/:id/credentials/batch/*` 固定段（含 `/batch/import`）**先于** `/pools/:id/credentials/:credId` 声明，避免 gin 把 `batch` 当成 `credId` 捕获；行为判别测试 `TestRegisterRouteOrder_BatchBeforeCredID` 覆盖三条批量路由

---

*本文档随 provider-channels S2 落地（2026-08-29）首次建立；S3–S7 扩展选路/状态机/同步后同步更新。*
