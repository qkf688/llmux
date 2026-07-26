# 命名规范

> 本文件描述项目的命名约定（现状描述）。编码规范与提交约定见 `AGENTS.md`。

## 1. 目录命名

| 对象 | 约定 | 示例 |
|------|------|------|
| Go 顶层包目录 | 小写单词 / 短名 | `handler/`, `service/`, `providers/`, `repository/` |
| handler 业务子包 | 小写，业务域缩写或复数 | `providerapi/`, `modelapi/`, `virtualmodels/`, `associations/` |
| service 业务子包 | 小写，常为单数领域名 | `chat/`, `virtualmodel/`, `healthcheck/`, `modelsync/` |
| 前端路由目录 | kebab-case | `virtual-models/`, `model-providers/`, `health-check-logs/` |
| 前端页面内子目录 | kebab-case 或固定语义名 | `components/sections/`, `components/dialogs/`, `hooks/` |
| 共享工具子包 | 小写 | `common/maputil/`, `handler/httpx/` |

## 2. 文件命名

| 对象 | 约定 | 示例 |
|------|------|------|
| Go 源文件 | snake_case | `chat_balance.go`, `model_with_provider.go` |
| Go 测试文件 | `*_test.go` 与实现同目录 | `provider_test.go` |
| 路由注册 | `register*.go` 或子包 `routes.go` | `handler/register_v1.go`, `providerapi/routes.go` |
| service 门面 | `*_facade.go` 或同名包装 | `chat_facade.go`, `virtual_model.go` |
| 前端组件/页面 | kebab-case 文件 | `models-desktop-table.tsx`, `use-models-page.ts` |
| 前端路由 re-export | 与路由名一致的顶层文件 | `routes/models.tsx` → `./models/index` |
| 前端类型/校验 | 语义目录 + 固定名 | `schemas/forms.ts`, `types/` |
| 配置/常量 | 短名或领域名 | `consts.go`, `setting_schema.go` |

## 3. 代码命名

| 对象 | 约定 | 示例 |
|------|------|------|
| Go 导出类型/函数 | UpperCamelCase | `RegisterAll`, `Provider`, `BalanceChat` |
| Go 局部变量 | lowerCamelCase | `syncService`, `modelID` |
| Go 接口 | 名词或 `XxxRepo` / 能力名 | `Provider`, `Selector`, `ModelRepo` |
| JSON / API 字段 | snake_case | `provider_model`, `max_retry` |
| 协议 style 常量 | `Style` 前缀 | `StyleOpenAI`, `StyleAnthropic`, `StyleOpenAIRes` |
| 设置键常量 | `SettingKey` 前缀 | `SettingKey*` |
| 前端组件 | PascalCase | `ModelsPage`, `ThemeProvider` |
| 前端函数/hooks | camelCase，`use` 前缀 | `useModelsPage`, `fetchWithAuth` |
| 前端 store selector | `select` 前缀常见 | `selectIsAuthenticated` |
| 虚拟模型策略名 | snake_case 字符串 | `priority`, `round_robin`, `random` |
| 供应商 type 字符串 | 小写连字符风格 | `openai`, `openai-res`, `anthropic` |

## 4. API 路径约定（现状）

| 类别 | 约定 | 示例 |
|------|------|------|
| 代理 API | `/v1/*` | `/v1/chat/completions`, `/v1/messages`, `/v1/responses` |
| 管理 API | `/api/*`，需鉴权（`TOKEN` 未设置时跳过） | `/api/providers`, `/api/settings` |
| 前端 SPA | 浏览器 path，由 `route-config.ts` 定义 | `/models`, `/virtual-models` |
| 静态资源 | `/assets` + SPA fallback | embed `webui/dist` |

## 5. 数据库与实体

| 对象 | 约定 | 示例 |
|------|------|------|
| GORM 实体 | 导出结构体，表名由 GORM 约定 | `Provider`, `ModelWithProvider`, `ChatLog` |
| 仓储接口 | `XxxRepo` | `ProviderRepo`, `ChatLogRepo` |
| 聚合根入口 | `Repositories` + `Default`/`SetDefault` | `repository.New(models.DB)` |
| 包内仓储访问器 | 每包一个 `repos.go`，导出未公开的 `repos()` | `service/chat/repos.go`, `handler/modelapi/repos.go` |
| 仓储方法命名 | `List*`/`Get*`/`Exists*`/`Count*`/`Create`/`Update*`/`Delete*`；硬删以 `Unscoped` 结尾或在注释标明 | `ListEnabledByModelID`, `DeleteByModelIDAndNameUnscoped` |
| 测试中的仓储绑定 | `models.Init` 后必须 `repository.SetDefault(repository.New(models.DB))`，清理时置 `nil` | `handler/testsupport.InitTestDB` |

---

*本文档由 Project Architecture Documenter skill 生成，生成日期：2026-07-23*
