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

- **负责什么**：`ModelWithProvider` 的权重、优先级、能力（tool/structured/image）、状态、连续失败等字段管理；自动按模板创建关联；清理无效关联
- **不负责什么**：上游连通性测试执行细节（`testapi`）；定时健康检查（`health-check`）；chat 运行时选路算法（消费本表数据）
- **对外暴露**：`handler/associations` REST；`handler/autoassoc` REST（委托 service）；`service/autoassoc.Service`；`ModelWithProviderRepo`；`NewDefaultAssociation`
- **依赖谁**：`repository`（Model / Provider / ModelWithProvider / ModelTemplateItem）；`service.BuildTemplateIndexFromData`（经门面注入）；`service/modelsync.GetProviderModels`（读 Config 模型列表）

## 3. 内部结构

```
handler/associations/     # CRUD、status、capabilities、health 视图
handler/autoassoc/        # HTTP 预览/执行 + Trigger* 薄封装
service/autoassoc/        # 统一 Associate / CleanInvalid 业务
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

## 5. 特殊约定

- chat 与 healthcheck 将关联行视为选路与启停的数据源；本模块是配置写入口
- **自动关联单一入口（已收敛）**：
  - 业务：`service/autoassoc`（`Associate` / `CleanInvalid` / `Preview*` / `Trigger*IfEnabled`）
  - HTTP：`handler/autoassoc` 只做绑定与响应
  - Provider CRUD：`TriggerAutoAssociate` / `TriggerAutoClean` → service
  - 同步后：`modelsync.ActionHooks` → `service.GetAutoAssocService().Associate/CleanInvalid`（开关在 `triggerAutoActions`）
  - 模板匹配：`service/model_template.go` 的 `TemplateIndex`（经 `autoassoc_facade` 注入 `BuildIndexFunc`）
- 规则权威：
  - 尊重 `Model.AutoAssociate`（`false` 跳过）
  - 跳过 `Provider.Blacklisted`
  - 默认 `Weight=5`，`Priority` 读 `SettingKeyAutoPriorityDecayDefault`（schema 默认 100）
  - 无效关联：provider 不存在，或 `ProviderModel` 不在 Config 的 upstream/custom 列表；拉列表失败则不删

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日收敛 autoassoc 回写。*
