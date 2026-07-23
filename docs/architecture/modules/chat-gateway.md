# chat-gateway 模块

> 本文件描述 LLM 代理网关的职责、边界、对外暴露、依赖、接口契约。
> 全局架构见 [README.md](../README.md)；模块间交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | LLM 请求代理：解析、选路、调用上游、流式回写、重试与日志 |
| 目录位置 | `handler/v1/`；`service/chat/`；`service/chatcore/`；`service/chat_facade.go`；`balancer/` |
| 主要职责 | 将客户端 OpenAI/Anthropic 兼容请求路由到可用供应商并记录结果 |
| 包形态 | **编排聚合包**（多变更原因共存于 `service/chat`，非单一窄域） |

## 2. 职责与边界

- **负责什么**：按协议 style 解析请求；解析模型名（真实/虚拟）；能力匹配；选择 `ModelWithProvider`；执行上游 HTTP（含重试）；流式/非流式处理；写入 `ChatLog`/`ChatIO`/统计；失败时权重/优先级衰减；注入健康检查 `AdjustmentHooks`
- **不负责什么**：协议字段级互转细节（`protocol-transform`）；供应商 CRUD（`providers` 管理端）；关联/虚拟模型配置 CRUD；健康检查与模型同步调度
- **对外暴露**：`service` 门面中的 `Beforer`/`Processer`/`ProvidersWithMetaBymodelsName`/`BalanceChat`/`RecordLog` 等；`handler/v1` 的 `/v1/models`、`/chat/completions`、`/responses`、`/messages`
- **依赖谁**：`providers`、`protocol-transform`、`virtual-models`、`models`（多直连 DB）、`balancer`/`chatcore`、`consts`

## 3. 内部结构

```
handler/v1/                 # /v1 入口与 chatHandler
service/
├── chat_facade.go          # 根包门面 re-export
├── chat/                   # 编排核心
│   ├── registry.go         # Beforer/Processer 注册表
│   ├── before.go           # 请求前处理
│   ├── process.go          # 响应/SSE 处理
│   ├── chat_balance*.go    # 选路与虚拟模型故障转移
│   ├── chat_attempt.go     # 单次上游尝试
│   ├── chat_record*.go     # 日志与 IO
│   ├── stats.go            # 统计写入
│   ├── adjustment.go       # 权重/优先级调整
│   └── preprocess/openai/  # OpenAI 预处理
└── chatcore/               # 优先级+权重选择、Header 构造
balancer/                   # 加权随机纯算法
```

各子目录职责：
- `handler/v1/`：HTTP 接入与 style 分发
- `service/chat/`：端到端编排与注册表
- `service/chatcore/`：供应商选择算法与请求头
- `balancer/`：无状态加权随机

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `Beforer` / `Processer` | 按 style 预处理请求 / 处理上游响应 | `service/chat`（门面 re-export） | style 注册实现 |
| `BalanceChat` | 在候选供应商上执行带重试的转发 | `service/chat_facade.go` | `service/chat` |
| `ProvidersWithMetaBymodelsName` | 按模型名解析候选供应商元信息（含虚拟模型分支） | `service/chat` | `service/chat` |
| `chatcore.SelectByPriorityAndWeight` | 在关联列表上按优先级与权重选供应商 | `service/chatcore/` | `service/chatcore` |
| `balancer.WeightedRandom` | 加权随机选取 | `balancer/balancer.go` | 泛型算法 |

## 5. 特殊约定

- **扩展最小改动集（现状）**：style 注册 `Beforer`/`Processer` + `register_v1` 路由 + transform 适配/流路由 + `consts`；不在 `main.go` 写业务路由
- **数据访问**：主要直连 `models.DB`/`gorm.G`，不强制走 `repository`（DIP 未在本路径落地）
- **职责聚合（现状）**：选路/重试、日志 IO、Stats、权重调整同属 `service/chat`，改日志策略与改选路策略会碰到同一包
- 虚拟模型路径：先由 `virtualmodel` 产出有序真实模型，再在真实模型层做 provider 级选路（两层 LB）
- `service/chat_facade.go` 为兼容 re-export，不是第二实现

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写。*
