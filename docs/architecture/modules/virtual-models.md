# virtual-models 模块

> 本文件描述虚拟模型与策略选路域。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 虚拟模型聚合、映射、策略选择（第一层 LB） |
| 目录位置 | `handler/virtualmodels/`；`service/virtualmodel/`；`service/virtual_model.go`；`repository/virtual_model.go`；实体 `VirtualModel`、`VirtualModelMapping` |
| 主要职责 | 将虚拟模型名解析为有序真实模型列表；CRUD 与映射管理；循环依赖校验 |

## 2. 职责与边界

- **负责什么**：虚拟模型与映射的配置；`Selector` 策略（`priority` / `round_robin` / `random`）；候选池（含黑名单过滤）；统计查询
- **不负责什么**：真实模型上的供应商选择（第二层 LB，属 `chat-gateway`）；上游 HTTP；关联能力标志管理
- **对外暴露**：`virtualmodel.Service`（`SelectRealModel` / `SelectRealModelsOrdered` 等）；`RegisterSelector`/`GetSelector`；`handler/virtualmodels` REST；`VirtualModelRepo` / `VirtualModelMappingRepo`
- **依赖谁**：`models`/`gorm`；被 `service/chat` 调用

## 3. 内部结构

```
handler/virtualmodels/     # CRUD、mappings、batch、stats
service/virtual_model.go   # 门面
service/virtualmodel/
├── service.go
├── selector.go            # 注册表
├── priority_selector.go
├── round_robin_selector.go
├── random_selector.go
├── candidate_pool.go / validation.go / stats.go
repository/virtual_model.go
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Selector` | 从候选真实模型中选择或排序 | `service/virtualmodel/selector.go` | priority / round_robin / random |
| `RegisterSelector` / `GetSelector` | 策略注册与查找 | `service/virtualmodel/selector.go` | 各 selector `init` |
| `Service.SelectRealModel(s)` | 业务入口：虚拟名 → 真实模型 | `service/virtualmodel/` | 同包 |
| `VirtualModel` / `VirtualModelMapping` | 配置实体 | `models/model.go` | GORM |

## 5. 特殊约定

- 两层负载均衡：虚拟→真实（本模块）+ 真实→供应商（chatcore/balancer）
- **扩展最小改动集（现状）**：新增 `*_selector.go` + `RegisterSelector` 即可挂进分发；但现有 priority/round_robin/random 多为 **薄包装转发 `Service.selectBy*`**，新算法常仍要改 Service 方法；前端策略白名单（如 forms schema）需同步
- 用户向操作说明见 [docs/virtual-models-guide.md](../../virtual-models-guide.md)

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
