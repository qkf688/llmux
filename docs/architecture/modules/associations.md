# associations 模块

> 本文件描述模型↔供应商关联与自动关联域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 真实模型与供应商的多对多关联及自动关联 |
| 目录位置 | `handler/associations/`；`handler/autoassoc/`；`repository/model_with_provider.go`；实体 `ModelWithProvider` |
| 主要职责 | 关联 CRUD、能力标志、状态、健康视图；自动关联与无效关联清理 |

## 2. 职责与边界

- **负责什么**：`ModelWithProvider` 的权重、优先级、能力（tool/structured/image）、状态、连续失败等字段管理；自动按模板创建关联；清理无效关联
- **不负责什么**：上游连通性测试执行细节（`testapi`）；定时健康检查（`health-check`）；chat 运行时选路算法（消费本表数据）
- **对外暴露**：`handler/associations` 与 `handler/autoassoc` REST；`ModelWithProviderRepo`；autoassoc 内部默认关联构造
- **依赖谁**：`models`、`repository`（部分）、`service.BuildTemplateIndexFromData`（自动关联）

## 3. 内部结构

```
handler/associations/   # CRUD、status、capabilities、health 视图
handler/autoassoc/      # associate / clean
repository/model_with_provider.go
models.ModelWithProvider
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `ModelWithProviderRepo` | 关联持久化与按模型/供应商查询删除 | `repository/model_with_provider.go` | GORM |
| `models.ModelWithProvider` | 关联实体（路由与健康检查的核心行） | `models/model.go` | GORM |
| autoassoc associate/clean | 批量创建/清理关联 | `handler/autoassoc/` | 同包 |

## 5. 特殊约定

- chat 与 healthcheck 将关联行视为选路与启停的数据源；本模块是配置写入口
- **自动关联双轨（现状）**：
  - HTTP：`handler/autoassoc`（多直连 GORM）
  - 同步后：`modelsync.ActionHooks` → `service/model_sync_auto_actions`（与 handler 逻辑并行，非同一 service 入口）
  - 模板匹配：`service/model_template.go` 的 `TemplateIndex`
- modelsync 经 Hook 避免直接依赖 handler 包，但**业务规则尚未收敛为单一 AutoAssociate 服务**

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
