# health-check 模块

> 本文件描述健康检查域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 关联可用性探测、自动禁用/恢复、健康日志 |
| 目录位置 | `handler/healthcheck/`；`service/healthcheck/`；`service/healthcheck.go`；`repository/health_check_log.go`；实体 `HealthCheckLog` |
| 主要职责 | 定时/手动探测模型-供应商关联；记录日志；经 Hook 触发权重/优先级调整与状态变更 |

## 2. 职责与边界

- **负责什么**：`HealthChecker` 单例调度；单条/批量检查；结果处理中经 `ModelWithProviderRepo` 更新关联 Status/连续失败；经 Hook 触发权重/优先级调整；健康日志与保留；健康相关设置读取
- **不负责什么**：通用 chat 代理路径；**权重/优先级衰减算法本体**（`AdjustmentHooks` → `service/adjustment`）；供应商 CRUD
- **对外暴露**：`GetHealthChecker()`、`Start`/`Stop`/`Restart`、`GetHealthCheckSettings`、`AdjustmentHooks`；管理端 run/logs API
- **依赖谁**：`repository`（关联读写、健康日志、保留清理，经 `repos()`）、`models`（实体/设置键）、`providers.Metadata`（HealthCheckBody）、`service/channel`（打上游的端点/分组/凭据装配，诊断口径）；入口由 `main` 启动

## 3. 内部结构

```
handler/healthcheck/        # 触发与日志相关 handler
service/healthcheck.go      # 门面
service/healthcheck/        # Start、check_single/batch、result、settings、retention
repository/health_check_log.go
models.HealthCheckLog
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `HealthChecker` | 后台探测调度 | `service/healthcheck/` | 单例实现 |
| `AdjustmentHooks` | 将探测结果交给 adjustment 做权重/优先级调整 | `service/healthcheck/` | `service/adjustment` 侧 `init` 注入 |
| `HealthCheckLogRepo` | 健康日志持久化 | `repository/health_check_log.go` | GORM |

## 5. 特殊约定

- 探针 body 来自 `providers.Metadata`，不在 healthcheck 硬编码各供应商协议细节
- **打上游走诊断口径装配（#21）**：`doCheck` 经 `channel.Assemble` + `SelectPrimaryEndpoint`（协议 = `ProtocolOfType(Provider.Type)`）+ `GroupCandidates`（全部组、ID ASC，不按白名单过滤）+ `FirstUsableCredential`（组内首个可用）+ `NewClientFromSelection`，**任一分组通过即判健康**——本处失败的代价是自动禁用关联，「首个分组的 key 坏了」不等于关联不可用（生产路由本来会故障转移）；成功即短路不多花钱，失败请求不产生 token 费用。刻意不碰 `DefaultSelector()`：诊断流量要可复现，且一轮巡检不得推进生产轮询指针
- **通过口径与全项目同源**（Stage 5，2026-09-17）：`requestUpstream` 判「上游是否答对」走 `common.IsPassingStatus`（**通过集 = 200 与 201**），不再自写 `== 200`。这条判定在本链路的代价不是报错而是**自动禁用关联**（`handleCheckResult` 累计到阈值即停用），旧口径把回 201 的兼容网关判死、线上静默少一条可用线路，且停用原因不入库。状态码未通过仍回 `HealthCheckError`（带状态码 + 上游原文）供分组级排障——**只统一判据，不统一失败形状**（诊断侧的 `error_type` 分类语义不引进本模块）
- **内容形状门**（Stage 19，2026-09-23）：状态码通过之后成功分支再判「内容是不是所测协议的合法响应」——判据单源 `transform.ValidateResponseShape` 四条口径（达 1 MiB 读体上限判「形状不可验」/ 空回复判过 / 非空体过 `common.JSONContentError` 与 chat 提交门同源 / 须被所测协议解析入口解析出响应对象），响应形状口径与请求体同源（`chooseTestProbe` ← `providers.WireFormatOf`，未知 type 同回退 OpenAI）。动机：上游被反代/网关改写到登录页时回 200 + HTML，旧口径判健康 → 健康检查绿、真实聊天每条报错，坏线路留在轮询里（Stage 5 盲区的镜像）。**失败契约**：形状/读体/超时失败为带状态码的普通 error（原文仅错误文本内 ≤200B 预览），仅状态码未通过回 `HealthCheckError`（带原文）；失败走既有失败链（error 日志 → 连败 → 衰减/停用），故字段类型怪异的响应（协议转换链会坏、直通链会过）会被判不健康并可能到阈值停用——刻意口径差，错误文本可归因、autoEnable 可自愈（`shape.go` 注释有完整推演）。**诊断时间预算**同此收口：`requestUpstream` 派生 `common.DiagnosticCompletionTimeout` 整体 ctx（防滴流卡死整轮巡检）+ transport 超时引 `common.DiagnosticResponseHeaderTimeout`（单源 + `gate_diag_timeout_test.go` 机械门，见 `framework-conventions.md §7`）；未注册 wire format 显式失败不回退，代价是漏配适配器会到阈值停用关联（刻意）
- **逐组失败原因合并进 `health_check_logs.error`**（带分组名）：该字段是排查入口，只报「failed」等于丢掉线索
- **Hook 解耦范围（现状）**：仅权重/优先级调整走 `AdjustmentHooks`；关联启停/失败计数仍在 healthcheck 结果路径内写库——不是「全部副作用都经 Hook」
- 禁止 healthcheck 直接 import `service/adjustment` 或 `service/chat` 形成环；Hook 为包级注入（实现在 `service/adjustment` 的 `init`）
- **`with_header` 在健康检查路径是空语义**：本链路是后台任务、无终端用户入站请求可透传，关联的 `with_header=true` 实际只生效 `customer_headers`（`check_single.go`）。勿按「三链路同源白名单透传」误解——共用 `common` 判据的只有 chat 生产转发与 testapi 诊断两条

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写；2026-09-15 记录 healthcheck 路径 `with_header` 的空语义（白名单收口仅涉 chat+testapi 两链路）；2026-09-17 记录上游状态码通过口径收口（Stage 5）；2026-09-23 记录内容形状门与诊断超时单源（Stage 19）。*
