# 请求日志性能优化：列表/详情拆分（设计）

日期：2026-03-13

## 背景

当前请求日志列表接口 `GET /api/logs` 会返回日志的原始请求/响应大字段：

- `RequestHeaders`
- `RequestBody`
- `ResponseHeaders`
- `ResponseBody`
- `RawResponseBody`

当开启“记录原始响应体”等选项后，单条日志可能非常大，导致：

- 列表接口 JSON 体积显著增大（gzip 解压 + JSON 解析成本上升）
- 前端列表页一次性将整页日志放入 state，渲染/交互明显变慢
- 后端列表接口存在 N+1 查询（每条日志额外查询虚拟模型/提供商信息），进一步放大延迟

## 目标

- 列表页加载速度不再随日志原始字段体积显著下降。
- 保持现有列表筛选/分页语义不变。
- 详情弹窗/导出时仍可获取完整请求/响应内容（按需加载）。
- 列表接口去除 N+1，固定少量查询完成 enrich 信息。

## 非目标

- 不做数据库表结构调整（如拆表、压缩存储）——该部分可作为后续方案（C）单独推进。
- 不改变现有“记录原始请求/响应”的写入逻辑。

## API 设计

### 1) 列表接口（保持路径不变）

- Method: `GET`
- Path: `/api/logs`
- Query 参数：保持与现有一致
  - `page`
  - `page_size`
  - `provider_name`
  - `name`
  - `status`
  - `style`
  - `user_agent`
- 新增可选参数：
  - `include_raw`：`true|false`，默认 `false`

#### 响应字段

当 `include_raw=false`（默认）时：

- 返回日志摘要字段（列表需要），并且 **不返回** 以下大字段：
  - `RequestHeaders`
  - `RequestBody`
  - `ResponseHeaders`
  - `ResponseBody`
  - `RawResponseBody`

当 `include_raw=true` 时：

- 返回与当前实现兼容的完整字段（便于临时排障/兼容旧行为）。

> 兼容策略：前端将默认使用 `include_raw=false`；该参数主要用于排障或回滚兜底。

### 2) 详情接口（新增）

- Method: `GET`
- Path: `/api/logs/:id`

#### 响应

- 成功：返回单条日志详情（包含全部字段，尤其是上述原始请求/响应大字段）。
- 失败：当 `id` 不存在时返回 `404 NotFound`。

## 后端实现设计

### 1) 列表查询字段裁剪

在 `GET /api/logs` 中：

- 默认使用显式 `Select(...)` 仅查询摘要字段，避免 SQLite 读取大 TEXT 列带来的 IO/CPU 开销。
- 当 `include_raw=true` 时，沿用现有查询（或切换到包含大字段的 Select）。

摘要字段建议包含：

- `id, created_at`
- `name, provider_model, provider_name, status, style`
- `user_agent, remote_ip`
- `error, retry`
- `proxy_time, first_chunk_time, chunk_time, tps`
- token 统计字段（如 `prompt_tokens, completion_tokens, total_tokens, prompt_tokens_details` 等，保持与当前列表展示一致）
- `chat_io`

### 2) 去除 N+1 enrich 查询

现状：每条 log 都会查询一次 `virtual_models` + 一次 `providers`。

改为批量方式：

1. 收集当前页日志的 `name` 集合，批量查询 enabled 的 `virtual_models`，得到 `virtualModelNames` set。
2. 收集当前页日志的 `provider_name` 集合，批量查询 `providers`，得到 `providerTypeByName` map。
3. enrich 时仅用内存 set/map 计算：
   - `is_virtual_model = name in virtualModelNames`
   - `has_format_conversion = (style != "" && providerType != "" && style != providerType)`
   - `source_format/target_format` 同理

### 3) 详情接口

新增 handler（例如 `GetRequestLogDetail`）：

- 查询 `models.ChatLog` by `id`
- 返回与列表一致的 enrich 字段（可复用同一套批量 enrich 逻辑的单条版本，或简单按需查询）
- 返回完整 raw 字段（由该接口负责承担大 payload）

## 前端实现设计

### 1) 列表页默认不取 raw

`getLogs(...)` 默认调用 `/api/logs`，不传 `include_raw`（或显式 `include_raw=false`）。

列表 UI 依赖 raw 的展示需要调整：

- 当前移动端列表会展示 `RequestHeaders/ResponseHeaders` 字节数；当列表不返回 raw 时，应显示 `-` 或隐藏该行。

### 2) 详情弹窗按需加载

`LogDetailDialog` 打开时：

- 若当前 `selectedLog` 缺少请求/响应内容字段，则调用 `GET /api/logs/:id` 获取详情并更新 `selectedLog`。
- 加载期间对“请求响应内容”区域显示 loading/占位，避免空白闪烁。

### 3) 导出请求响应

“导出请求响应”操作需要确保已拥有详情数据：

- 若 raw 字段不存在，先拉取详情，再执行导出。
- 若请求失败，toast 提示并保持对话框可用。

## 测试

### Go 单测

- `GET /api/logs` 在默认模式不应返回 raw 字段（或确保为空字符串/未选择列导致为零值）。
- `GET /api/logs?include_raw=true` 返回 raw 字段（与旧行为兼容）。
- `GET /api/logs/:id` 返回完整 raw 字段，并且 `404` 行为正确。
- 列表接口 enrich 逻辑不应产生 N+1（可通过 mock/spy 难度较大；更现实的是确保实现不在循环里发 DB 查询，并为 enrich helper 写单测覆盖）。

### 前端

- 日志页列表能正常加载与分页/筛选（不依赖 raw 字段）。
- 点“详情”时能加载并展示请求/响应内容。
- 导出功能在未提前加载详情时也能正常工作。

