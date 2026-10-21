# credentials 模块（凭据：第一等资源）

> 凭据自身的属性与动作（状态、备注、明文、以后的凭据级设置）在这里实现一次；号池与供应商分组只是它的两种**归属**。
> 全局架构见 [README.md](../README.md)；号池容器与批量导入见 [pools.md](pools.md)；供应商分组只引用号池、其表单不再有凭据行（见 [providers.md](providers.md)）。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 凭据资源管理：列表（按归属过滤）/ 详情 / 创建（号池）/ 启停 / 备注 / 明文查看 / 删除 |
| 目录位置 | `handler/credentials/`；`repository/credential.go`；`models/channel.go`（`Credential` 表）；对外视图 `service/credview/` |
| 主要职责 | 提供「凭据 ID 即资源标识」的单一读写面：凭据按 `pool_id` 归属号池（内联凭据概念已于 2026-10-07 删除），但属性、动作与视图对所有凭据一视同仁，不按归属分叉 |

## 2. 职责与边界

- **对外暴露**：REST `/api/credentials…`（无归属限界：ID 就是资源标识）；`CredentialRepo`（`repository/credential.go`，仓储层不解密）；`credview.View` 视图类型
- **依赖谁**：`repository`（经 `repos()`）、`models`（`Credential` 实体 + `CredentialRecoveryFields` 状态机约定）、`common/credentialcrypto`（加解密 / 掩码单源）、`service/credview`（对外视图单源）、`handler/httpx` + `httpresp`
- **不负责什么**：号池容器（`handler/pools`）、供应商分组的号池引用（`handler/providerapi.syncGroups` 只写 `KeyGroup.PoolID`，不碰凭据）、上游选路与健康状态机判定（`service/channel`、`service/chat`）、探活与按凭据测试（`service/credprobe`、`handler/pools` 的池作用域端点）

## 3. 为什么独立成域（历史动因）

凭据原先只作为号池的子资源存在（`/api/pools/:id/credentials…`），所有写接口都带池限界校验。后果：**分组内联凭据（`group_id` 归属、`pool_id` 为空）拿不到任何按凭据 ID 的属性与动作**——被状态机判停（`temp_unsched`）后无法人工恢复，而设计定案第 5 节的状态机出口明确写了「人工恢复 / 探活成功」。同时，「凭据视图」被拼了两份（号池 `CredentialListItem` 与供应商掩码项 `InlineKeyItem`），加字段必漏一处。

改为第一等资源后：启停 / 备注 / 明文 / 以后的凭据级设置（如重试次数）只实现一次；归属只作为**列表过滤条件**与聚合端点的校验维度。

> 历史收尾（2026-10-07）：内联凭据概念整体删除，`Credential.GroupID` 与供应商侧的明文拉取端点一并退场——凭据只归属号池，分组经 `KeyGroup.PoolID` 引用。上文「内联凭据」只作历史动因记录，当前代码无此形态。本域「第一等资源」的判断不受影响，反而更纯粹：凭据不再有两套归属。

## 4. 内部结构

```
handler/credentials/     # 凭据域 API（第一等资源）
  routes.go              # Register：/credentials GET/POST、/credentials/status PATCH、/credentials/batch DELETE、/:id GET/PATCH/DELETE、/:id/raw GET（固定段先于 :id）
  crud.go                # ListCredentials（pool_id/status/q 过滤 + 分页）/ GetCredential / CreateCredential / GetCredentialRaw / UpdateCredential / DeleteCredential + requireCredential + parseUintQuery
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
| `GET /api/credentials` | 列表：`pool_id` 归属过滤（不带 = 全量）+ `status`（`all`/空 = 不过滤）+ `q`（备注子串）+ 分页；非法状态或非法归属 ID → 400。**`group_id` 过滤已随内联凭据概念删除**（凭据必属号池，分组只是引用方） | `handler/credentials/crud.go` `ListCredentials` | 同文件 |
| `POST /api/credentials` | 创建：`pool_id` **必填**（凭据只归属号池，别无写入口）；池不存在 404、空 key 400、池内 `KeyHash` 重复 400（跨池同 key 允许） | `handler/credentials/crud.go` `CreateCredential` | 同文件 |
| `PATCH /api/credentials/:id` | 更新：`note` 可清空、`status` 可切；**切回 `active` 即人工恢复 = 立刻可用**，连同清 `fail_count`/`cooldown_reason`/`cooldown_until`（单一来源 `models.CredentialRecoveryFields`；2026-10-08 起含解除冷却——只清失败计数会留下「状态是启用、选路仍跳过」的假象）；无字段 400、非法状态 400、凭据不存在 404 | `handler/credentials/crud.go` `UpdateCredential` | 同文件 |
| `PATCH /api/credentials/status` | 批量启停（`{ids, status}`）：**不分归属**，一次调用可同时启停任意池的凭据；空 ids 400、非法状态 400 | `handler/credentials/batch.go` | `CredentialRepo.UpdateFieldsByIDs` |
| `GET /api/credentials/:id/raw` | 明文受控端点：`SuccessNoStore`（禁缓存）+ 解密失败 500（不回退密文） | `handler/credentials/crud.go` `GetCredentialRaw` | `httpresp.SuccessNoStore` |
| `credview.View` / `ViewOf` / `ViewsOf` | 凭据对外形状单源（掩码 + 状态 + 冷却 + 连败 + 统计 + 时间；三态指针禁 omitempty）；`ViewsOf(nil)` 回非 nil 空切片（JSON `[]`） | `service/credview/view.go` | `handler/credentials`（列表/详情/更新）独家使用——供应商侧已无凭据视图（2026-10-07） |
| 路由注册顺序 | `/credentials/status` 与 `/credentials/batch` 固定段先于 `/credentials/:id`，避免 `status`/`batch` 被当成凭据 ID（测试 `TestRegisterRouteOrder_StaticSegmentsBeforeID`） | `handler/credentials/routes.go` | 同文件 |

## 6. 特殊约定

- **写路径无归属限界**：凭据 ID 即资源标识，`PATCH`/`DELETE` 不再校验「属不属于某个池/组」；归属校验只出现在**列表过滤**与**分组级聚合端点**（`providerapi` 的内联明文查看）里
- **状态是凭据属性、不属于表单提交内容**：供应商表单保存只写 key 与归属（`syncGroups` 不碰 `status`/`fail_count`），所以「换 key」不会自动清判停状态——恢复要么点「启用」（本域接口），要么等组内惰性探活成功
- **启停按钮的显示判据是前端单源**（Stage 40）：`components/credentials/credential-status.ts` 的 `credentialToggleAction` 按「现在能不能用」决定显示哪个词——用不了（人工停用 / 判停 / 冷却中）显示「启用」，能用显示「停用」；号池页电脑行与手机行都调它，禁止在页面里各写一份 `status === "disabled"` 判断。按「是不是人工关的」判会把一条还能自愈的凭据钉成人工终态（后台不再自动拉回）
- **「凭据为什么不可用」的文案与时间口径也在前端单源**（2026-10-22）：同一文件的 `describeCredentialFailureReason`（后端机器码 `auth_fail`/`network`/`http_429`/`http_<码>` → 中文，**空值返回空串**由调用方据此不渲染、未登记码原样露出不藏成「未知」）与 `formatRelativeTime`（相对时间内核，**「没有值」的措辞由调用方给**——「上次成功使用」传「从未使用」、「上次探测」传「从未探测」，时间算式只有一份）。消费方目前是号池详情弹窗的凭据状态格（`components/number-pools/pool-detail-dialog.tsx`），**只在凭据确实出过问题时渲染**（有冷却原因或连败次数）——健康凭据不挂这些信息，避免噪音。后端 `CooldownReason` 是**机器码**（供程序判断），管理端读到的应是判断依据本身，故由前端翻译；后端刻意不保存上游原始报错全文（只留归类），故前端不可能也不该显示原文
- **探活 / 按凭据测试仍留在池域**：两者都依赖池级配置（`Pool.TestModel` 是测试模型来源）或走诊断口径（只报不改），不属于凭据属性面
- **测试**：`handler/credentials/{crud,batch}_test.go`（契约断言打在响应体 JSON）、`service/credview/view_test.go`
- **迁移记录**：原 `/api/pools/:id/credentials` 系列（列表/创建/详情/更新/删除/raw/批量启停/批量删除）已删除（开发阶段不留兼容层），前端 `lib/api/modules/catalog/credentials.ts` 与号池页、供应商弹窗共用

---

*本文档随「凭据提为第一等资源」改造（2026-09-28）首次建立。*