# system-ops 模块

> 横切接入与运维的**文档合集**（非单一业务实体）。全局架构见 [README.md](../README.md)；交互见 [interactions.md](../interactions.md)。

## 1. 模块概述

| 属性 | 值 |
|------|-----|
| 业务领域 | 横切：bootstrap + ops-io + test + 共享工具 |
| 目录位置 | 见下文子边界；勿将无关业务规则继续堆入本「合集」 |
| 主要职责 | 启动与路由挂接、鉴权、导入导出/DB 运维、连通测试、HTTP 响应与绑定辅助 |

## 2. 职责与边界

### 子边界（文档内拆分，代码目录未强制拆顶层包）

| 子边界 | 目录/入口 | 负责什么 |
|--------|-----------|----------|
| **bootstrap** | `main.go`、`middleware/`、`handler/register*.go`、`repos.go` | 进程启动、鉴权、`RegisterAll`、默认 Repo、embed WebUI |
| **ops-io** | `handler/importexport/`、`handler/database/` | 配置导入导出、DB stats/vacuum/export |
| **test** | `handler/testapi/`、`handler/testsupport/` | 供应商连通/工具/structured 测试与测试辅助 |
| **shared-http** | `httpresp/`、`handler/httpx/` | 管理端 JSON 信封；分页/绑定辅助（**渐进采用**） |
| **shared-lib** | `common/`、`consts/` | 无业务编排的工具与常量 |

- **不负责什么**：chat 选路、虚拟模型策略、协议转换、各业务 CRUD 规则
- **对外暴露**：`RegisterAll`、`Deps`、`middleware.Auth*`、`httpresp.*`、`httpx.*`、import/export/database/test REST
- **依赖谁**：聚合各 handler 子包与 service 启动接口；共享包不依赖业务

## 3. 内部结构

```
# bootstrap
main.go / middleware/ / handler/register*.go / repos.go
# ops-io
handler/importexport/ / handler/database/
# test
handler/testapi/ / handler/testsupport/
# shared
httpresp/ / handler/httpx/ / common/ / consts/
```

## 4. 关键接口契约

| 契约 | 职责 | 定义位置 | 实现方 |
|------|------|----------|--------|
| `handler.RegisterAll` | 注册全部业务路由（新域需改此函数挂接） | `handler/register.go` | 调用各子包 `Register*` |
| `handler.Deps` | 注入 Token 等启动依赖 | `handler/register.go` | `main` 构造 |
| `middleware.Auth` | 管理 API 鉴权 | `middleware/auth.go` | 同文件 |
| `repository.SetDefault` / `Default` | 默认仓储绑定 | `repository/repository.go` | `main` + handler `repos()` |
| `httpresp.Success/Error*` | 统一管理端响应 | `httpresp/` | 同包 |

## 5. 特殊约定

- **禁止**在 `main.go` 增加业务路由明细；新 API 域改 `RegisterAll` + 子包 `Register*`
- `database`/`importexport` 允许 PRAGMA/VACUUM/`c.File`/事务 SQL 等白名单直连
- `httpx`：**存在** Strict/Loose 分页与 `BindJSON`，但 handler 仍大量手写 `ShouldBindJSON`——属渐进落地，非已全量统一
- `common` 不再依赖 Gin（响应已迁 `httpresp`），避免沦为业务垃圾桶
- 前端静态：`//go:embed webui/dist`，`NoRoute` 对非 `/api` `/v1` 的 GET 回 `index.html`

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23；同日审查回写子边界。*
