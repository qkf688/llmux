# credentials 模块（凭据：第一等资源）

> 凭据自身的属性与动作（状态、备注、明文、以后的凭据级设置）在这里实现一次；号池与供应商分组只是它的两种**归属**。
> 全局架构见 [README.md](../README.md)；号池容器与批量导入见 [pools.md](pools.md)；供应商侧的内联 key 表单见 [providers.md](providers.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 凭据资源管理：列表（按归属过滤）/ 详情 / 创建（号池）/ 启停 / 备注 / 明文查看 / 删除 |
| 目录位置 | `handler/credentials/`；`repository/credential.go`；`models/channel.go`（`Credential` 表）；对外视图 `service/credview/` |
| 主要职责 | 提供「凭据 ID 即资源标识」的单一读写面，让号池凭据（`pool_id`）与分组内联凭据（`group_id`）共用同一套属性、动作与视图 |

## 2. 职责与边界

- **对外暴露**：REST `/api/credentials…`（无归属限界：ID 就是资源标识）；`CredentialRepo`（`repository/credential.go`，仓储层不解密）；`credview.View` 视图类型
- **依赖谁**：`repository`（经 `repos()`）、`models`（`Credential` 实体 + `CredentialRecoveryFields` 状态机约定）、`common/credentialcrypto`（加解密 / 掩码单源）、`service/credview`（对外视图单源）、`handler/httpx` + `httpresp`
- **不负责什么**：号池容器（`handler/pools`）、供应商分组与内联 key 的表单同步（`handler/providerapi.syncGroups`）、上游选路与健康状态机判定（`service/channel`、`service/chat`）、探活与按凭据测试（`service/credprobe`、`handler/pools` 的池作用域端点）

## 3. 为什么独立成域（历史动因）

凭据原先只作为号池的子资源存在（`/api/pools/:id/credentials…`），所有写接口都带池限界校验。后果：**分组内联凭据（`group_id` 归属、`pool_id` 为空）拿不到任何按凭据 ID 的属性与动作**——被状态机判停（`temp_unsched`）后无法人工恢复，而设计定案第 5 节的状态机出口明确写了「人工恢复 / 探活成功」。同时，「凭据视图」被拼了两份（号池 `CredentialListItem` 与供应商掩码项 `InlineKeyItem`），加字段必漏一处。

改为第一等资源后：启停 / 备注 / 明文 / 以后的凭据级设置（如重试次数）只实现一次，号池页与供应商弹窗共用；归属只作为**列表过滤条件**与聚合端点的校验维度。

## 4. 内部结构

```
handler/credentials/     # 凭据域 API（第一等资源）
  routes.go              # Register：/credentials GET/POST、/credentials/status PATCH、/credentials/batch DELETE、/:id GET/PATCH/DELETE、/:id/raw GET（固定段先于 :id）
  crud.go                # ListCredentials（pool_id/group_id/status/q 过滤 + 分页）/ GetCredential / CreateCredential / GetCredentialRaw / UpdateCredential / DeleteCredential + requireCredential + parseUintQuery
  batch.go               # BatchUpdateCredentialStatus（含恢复重置字段）/ BatchDeleteCredentials
  types.go               # CredentialListData / CreateCredentialRequest / UpdateCredentialRequest / 批量 DTO / CredentialRawData
  repos.go               # repos() → repository.Default()
service/credview/
  view.go                # View 结构 + ViewOf/ViewsOf（掩码走 MaskStored，两个消费域同一形状）
repository/credential.go # CredentialRepo：ListPaged/Get/Create/UpdateFields/UpdateFieldsByIDs(按 ID)/Delete/DeleteByIDs/DeleteByPoolID/ExistingHashes/RecordCredential*（见 pools.md 契约表）
```

## 5. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `GET /api/credentials` | 列表：`pool_id` / `group_id` 归属过滤（可都不带 = 全量）+ `status`（`all`/空 = 不过滤）+ `q`（备注子串）+ 分页；非法状态或非法归属 ID → 400 | `handler/credentials/crud.go` `ListCredentials` | 同文件 |
| `POST /api/credentials` | 创建：`pool_id` **必填**（分组内联凭据的唯一写入口是供应商表单同步 `syncGroups`，两条写路径会让归属/去重语义漂移）；池不存在 404、空 key 400、池内 `KeyHash` 重复 400（跨池同 key 允许） | `handler/credentials/crud.go` `CreateCredential` | 同文件 |
| `PATCH /api/credentials/:id` | 更新：`note` 可清空、`status` 可切；**切回 `active` 即人工恢复**，连同清 `fail_count`/`cooldown_reason`（单一来源 `models.CredentialRecoveryFields`）；无字段 400、非法状态 400、凭据不存在 404 | `handler/credentials/crud.go` `UpdateCredential` | 同文件 |
| `PATCH /api/credentials/status` | 批量启停（`{ids, status}`）：**不分归属**，一次调用可同时启停号池凭据与分组内联凭据（内联 key 的恢复路径）；空 ids 400、非法状态 400 | `handler/credentials/batch.go` | `CredentialRepo.UpdateFieldsByIDs` |
| `GET /api/credentials/:id/raw` | 明文受控端点：`SuccessNoStore`（禁缓存）+ 解密失败 500（不回退密文） | `handler/credentials/crud.go` `GetCredentialRaw` | `httpresp.SuccessNoStore` |
| `credview.View` / `ViewOf` / `ViewsOf` | 凭据对外形状单源（掩码 + 状态 + 冷却 + 连败 + 统计 + 时间；三态指针禁 omitempty）；`ViewsOf(nil)` 回非 nil 空切片（JSON `[]`） | `service/credview/view.go` | `handler/credentials`（列表/详情/更新）与 `handler/providerapi`（分组内联 key 项）共用 |
| 路由注册顺序 | `/credentials/status` 与 `/credentials/batch` 固定段先于 `/credentials/:id`，避免 `status`/`batch` 被当成凭据 ID（测试 `TestRegisterRouteOrder_StaticSegmentsBeforeID`） | `handler/credentials/routes.go` | 同文件 |

## 6. 特殊约定

- **写路径无归属限界**：凭据 ID 即资源标识，`PATCH`/`DELETE` 不再校验「属不属于某个池/组」；归属校验只出现在**列表过滤**与**分组级聚合端点**（`providerapi` 的内联明文查看）里
- **状态是凭据属性、不属于表单提交内容**：供应商表单保存只写 key 与归属（`syncGroups` 不碰 `status`/`fail_count`），所以「换 key」不会自动清判停状态——恢复要么点「启用」（本域接口），要么等组内惰性探活成功
- **探活 / 按凭据测试仍留在池域**：两者都依赖池级配置（`Pool.TestModel` 是测试模型来源）或走诊断口径（只报不改），不属于凭据属性面
- **测试**：`handler/credentials/{crud,batch}_test.go`（契约断言打在响应体 JSON）、`service/credview/view_test.go`
- **迁移记录**：原 `/api/pools/:id/credentials` 系列（列表/创建/详情/更新/删除/raw/批量启停/批量删除）已删除（开发阶段不留兼容层），前端 `lib/api/modules/catalog/credentials.ts` 与号池页、供应商弹窗共用

---

*本文档随「凭据提为第一等资源」改造（2026-09-28）首次建立。*