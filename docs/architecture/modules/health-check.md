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

- **负责什么**：`HealthChecker` 单例调度；单条/批量检查；**结果处理中直接更新关联 Status/连续失败**；经 Hook 触发权重/优先级调整；健康日志与保留；健康相关设置读取
- **不负责什么**：通用 chat 代理路径；**权重/优先级衰减算法本体**（`AdjustmentHooks` → chat）；供应商 CRUD
- **对外暴露**：`GetHealthChecker()`、`Start`/`Stop`/`Restart`、`GetHealthCheckSettings`、`AdjustmentHooks`；管理端 run/logs API
- **依赖谁**：`models`（含直连写关联行）、`providers.Metadata`（HealthCheckBody）、settings 键；入口由 `main` 启动

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
| `AdjustmentHooks` | 将探测结果交给 chat 做权重/优先级调整 | `service/healthcheck/` | `service/chat` 侧注入 |
| `HealthCheckLogRepo` | 健康日志持久化 | `repository/health_check_log.go` | GORM |

## 5. 特殊约定

- 探针 body 来自 `providers.Metadata`，不在 healthcheck 硬编码各供应商协议细节
- **Hook 解耦范围（现状）**：仅权重/优先级调整走 `AdjustmentHooks`；关联启停/失败计数仍在 healthcheck 结果路径内写库——不是「全部副作用都经 Hook」
- 禁止 healthcheck 直接 import chat 内部调整函数形成环；Hook 为包级注入

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
