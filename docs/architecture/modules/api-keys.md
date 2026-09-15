# api-keys 模块（调用方 API Key 管理）

> 多 api-key 方案切片 A（2026-09，`.local/方案/多api-key与key级自定义出站头-方案.md`）。管理「谁在调 /v1」：一人多把 key，替代 users 表单 key + 轮换的老模式。

## 1. 模块概述

- **业务领域**：/v1 代理 API 的调用方 key 生命周期——创建（自定义值或系统生成）、列表（明文回显供复制）、重命名、启停、删除。
- **切片进度**：`/v1` 鉴权已切到查 `api_keys.api_key_hash`（#4-2，2026-09-15），users 单 key 路径已全退；日志归因快照已落地（#4-3，2026-09-15：`chat_logs.api_key_id`/`api_key_name` 写入时快照，/v1 全部四个建行点打穿，日志页「密钥」列展示，见 [logs-metrics.md](logs-metrics.md) §5）；前端管理页已落地（#5，2026-09-16：`webui/src/routes/api-keys/` 独立页「API Keys」——octopus 式卡片列表（行内不渲染密钥值，明文仅经复制按钮进剪贴板；解密失败降级行的复制入口禁用）+ 编辑弹窗密钥区（默认掩码，点「显示明文」才上屏，关窗回落——IP+HTTP 剪贴板不可用时的手动复制兜底）/创建含 409 冲突特判/重命名/启停/硬删确认，API 层 `lib/api/modules/api-keys`，`createApiKey` 因 409 真实状态码弃用 `apiRequest` 走 `fetchWithAuth` 手动解包）；**key 级自定义出站头已落地（#6，2026-09-16，切片 B 完结）**：`api_keys.headers` 明文 JSON 列（伪装客户端头族是展示配置非凭据，鉴权直读不经 cipher），弹窗键值行编辑 + Codex/Claude Code 一键预设，转发时经唯一装配点 `chatcore.BuildHeaders` 按四层优先级链注入（见 §4 与方案 §3）。

## 2. 职责与边界

- **做**：`api_keys` 表的 CRUD 与密钥三态（明文规范化 → AES-GCM 密文 + SHA-256 hash）；bootstrap 首把 `default` key 的生成归 `service/auth`（本模块提供存储与规则 `common/apikeyvalue.go`）。
- **不做**：/v1 请求期鉴权判定（`middleware/auth.go:AuthAPIKey`，已接线本表 `repository.ApiKeyRepo.FindByHash`，见 §4 调用方契约）；上游供应商凭据（`pools`/credentials 表——那是「网关→上游」方向的 key，与这里「调用方→网关」方向相反，勿混）；key 值修改（轮换 = 新建 + 停用/删除旧，方案 §2 定案）。

## 3. 内部结构

```
models/api_key.go               # ApiKey 实体：OwnerID/Name/Key(密文)/KeyHash(uniqueIndex)/Headers(明文JSON,#6)/Enabled
common/apikeyvalue.go           # 值规则单一来源：NormalizeCallerKeyValue / GenerateCallerKeyValue
repository/api_key.go           # ApiKeyRepo（唯一冲突→ErrApiKeyExists；Delete 硬删）
handler/apikeys/
├── routes.go                  # Register：POST/GET /api-keys, PATCH/DELETE /api-keys/:id
├── repos.go                   # repository.Default() 注入（同 pools/auth 模式）
└── crud.go                    # 编解码 + 加解密 + 409/400 映射（DTO snake_case，见 §4）
webui/src/lib/api/modules/api-keys/api-keys.ts  # 前端 API 层（#5）：ApiKeyItem 逐键对齐 DTO；createApiKey 走 fetchWithAuth 手动解包以消费真实 409 body
webui/src/hooks/api/use-api-keys.ts             # queryKey 工厂 + useApiKeys/useCreate/useUpdate/useDelete（invalidate 列表）
webui/src/routes/api-keys/                      # 「API Keys」管理页（#5）：index + hooks/use-api-keys-page 编排 + sections（octopus 式卡片列表：行内不渲染密钥值、复制/编辑/启停/删除，解密失败降级行禁复制）+ dialogs（编辑：改名 + 掩码/显示明文/复制 + key 级 headers 编辑区，创建：名称 + 可选值 + headers；删除确认）+ route-config 挂导航
├── constants/header-presets.ts  # #6 Codex / Claude Code 出站头预设种子（头名抄 new-api 实战清单，值可改）
└── utils/header-fields.ts       # #6 Record↔编辑行转换 + 预设按名覆盖合并（纯函数）
```

## 4. 关键接口契约

| 接口 | 契约 |
|------|------|
| `POST /api/api-keys` | 体 `{name, value?, headers?}`；`value` 省略=系统生成 `sk-llmux-`+32hex；缺前缀自动补 `sk-llmux-`、补后总长 <16 拒绝（信封 400）；hash 撞库→**真实 HTTP 409**（`httpresp.Conflict`，应用层不预查——方案 §5）；`headers`=key 级自定义出站头（可选，管理员显式通道不设黑名单，#6）。响应 `data` = apiKeyResponse，恒 `Cache-Control: no-store`。⚠️ 前端 `apiRequest` 对非 2xx 直接 throw 并**丢弃响应体**（`lib/api/core/client.ts`），故 #5 的 `createApiKey` 已按 HTTP 409 特判走 `fetchWithAuth` 手动解包消费 body（新调用点勿改回 `apiRequest`，有契约测试钉死）。 |
| `GET /api/api-keys` | 全量列表（含 disabled）；`api_key` 为解密明文（管理端复制用），单行解密失败降级 `"****"` + slog.Warn（同 MaskStored 口径）；no-store。 |
| `PATCH /api/api-keys/:id` | `{name?, enabled?, headers?}` 三态指针=部分更新（`headers`：JSON 键缺席=不改、对象含 `{}`=整体覆盖/清空——序列化收在 handler，GORM serializer:json 对 map-Updates 不生效）；不提供改值端点；rows=0→信封 404。 |
| `DELETE /api/api-keys/:id` | **硬删**（Unscoped）：软删残留会永久挡死同值重建（uniqueIndex 不含 deleted_at）；在途流不受影响（鉴权只在入口执行一次）。 |
| `apiKeyResponse` | `{id, name, api_key, enabled, created_at, headers}`（snake_case；`headers` 键恒在无 omitempty：`null`=未配置，前端 interface 用 `Record<string,string> \| null`）；前端必须逐键对齐（AGENTS §5 契约测试纪律）。 |
| `repository.ApiKeyRepo` | 加解密不在 repo 层（存取已加密行，与 CredentialRepo 同边界）；空 Key/KeyHash 的 Create 显式拒绝（防占死唯一索引空槽）。 |
| `common.NormalizeCallerKeyValue / GenerateCallerKeyValue` | 规则跨 handler/apikeys 与 service/auth（bootstrap default key）两域共用，故落 common；产物恒满足「带前缀且 ≥16」。 |
| /v1 调用方鉴权（`middleware.AuthAPIKey`） | 按 `credentialcrypto.HashKey(明文)` SQL 点查 `api_keys.api_key_hash`（无内存索引）；放行注入 owner（`ContextKeyUser`）与 `CallerKey{ID,Name,Headers}` 快照（`ContextKeyCallerKey`，#4-3/#6 共用；Headers 浅拷贝自 `api_keys.headers` 明文 JSON 列直读，不经 cipher，定入 context 即定格）；401 分态「missing / Invalid API key / API key is disabled / owner not found」，仓储非 not-found 故障回 500；鉴权不解密、不依赖 `credentialcrypto.Default()` 装配态（hash 为纯 SHA-256 单源 `HashKey`）。 |
| key 级出站头装配（#6） | `CallerKey.Headers` → `handler/v1.newReqMeta` → `models.ReqMeta.Headers`（整包透传，读方与签名零改动）→ `chatcore.BuildHeaders` **唯一装配点**按四层优先级链注入（入站白名单 < key 级 < 关联级；撞名关联级赢）。写入闸在 apikeys handler：名规范化为 MIME Title-Case、非法名/控制字符值 400、规范化后重名拒绝（Go map 迭代序随机，「后者赢」对 map 输入不可确定，见 `headers_guard.go`）；这是语法关不是头名黑名单。 |
| bootstrap 首启 | users 空时 admin + `default` key 同事务创建（防半套状态；不带 headers，切片 B 零改动）；key 明文仅 stderr 打印一次（不进 slog 收集器，同密码先例）。 |

## 5. 安全边界

- 明文只出现在管理 API（JWT 保护 + no-store）、首启 stderr 与**管理端前端 react-query 内存缓存**（#5 起：列表端点恒回明文，前端必然缓存全量明文，默认 gcTime 5min；no-store 只挡 HTTP/磁盘层——方案 §2 复制模型的既定破例，与号池「列表回掩码+按需 raw」不同构）。登出清缓存属全站横切项（find-issues 2026-09-16 已登记）。
- 启停/删除不打断已鉴权通过的在途请求（octopus 同款语义，方案 §2）。
- **key 级头不参与诊断/连通测试**（#6 定案边界）：testapi/healthcheck 链路结构上不带调用 key 上下文，管理端「测试」按钮的结果**不能**证明 key 级头生效；验证靠真实流量 + 日志「密钥」列归因（弹窗文案已写明）。做预览端点会演化出第二套真相，方案 §6 已关。
- **key 级头按 key 全量扇出**（审查 W2 补记）：headers 与供应商无关，注入该 key 能路由到的**每一个**上游（虚拟路径同理）——勿在 key 级头存放某供应商专用凭据/头值。`Authorization` 对 openai/bearer 式供应商被凭据覆写（死通道）；**anthropic 默认 `x-api-key` 认证分支不清理 Authorization**（providers/anthropic.go），会原样送出——「恒被覆写」不成立，措辞勿回退成绝对断言；x-api-key 分支补 `Del("Authorization")` 与否涉关联级存量行为，已登 find-issues 待裁。
- `api_keys.headers` 是**明文** JSON 列（与 Key 列的 AES-GCM 密文不同构）：伪装客户端头族属展示配置不是凭据，鉴权 FindByHash 直读；管理员显式通道**名不设黑名单**但过语法关（规范化 + 非法名/CRLF 值 400，`headers_guard.go`，审查 W3——防误填打挂该 key 全量流量并误伤无关凭据冷却）。
- PATCH 体显式 `"headers": null` 等价**键缺席=不改**（Go json 对 null 置 nil 指针，反直觉但成文）；想清空必须传 `{}`。
