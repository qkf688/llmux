# 设计文档：模型同步日志页新增“最近新增模型”Tab

- 日期：2026-02-18
- 主题：model-sync-logs 最近新增模型展示

## 背景与目标
当前 `/model-sync-logs` 页面仅展示同步日志与详情。需要新增一个 Tab，展示“最近一次整体同步”中新增的模型列表，包含模型名称、提供商名称、新增时间。

## 范围
- **包含**：新增后端专用 API、数据库新增 batch_id、前端 Tab 切换与列表展示。
- **不包含**：跨时间范围筛选、模型详情跳转、权限与角色控制变更。

## 关键决策
- **新增 batch_id**：对“整体同步”进行精确分组，避免多批日志混淆。
- **新增专用 API**：`GET /api/model-sync/recent-added-models` 由后端聚合最近批次新增模型。

## 数据模型变更
`models.ModelSyncLog` 增加字段：
- `BatchID *uint`（索引），同一批次同步日志共享同一 batch_id。

影响：
- 历史日志 `batch_id` 为 NULL，不参与最近批次聚合。

## 后端设计
### 1) 同步批次标识
- 在 `SyncAllProviders` 中生成 `batch_id`（例如使用时间戳或序列生成）。
- 每个 `SyncProviderModels` 产生的日志更新 `batch_id`。

### 2) 新增聚合 API
**Endpoint**：`GET /api/model-sync/recent-added-models`

**响应结构**：
```json
{
  "data": [
    {
      "model_name": "gpt-4-turbo",
      "provider_name": "OpenAI",
      "added_at": "2026-02-18T10:30:00Z"
    }
  ],
  "sync_time": "2026-02-18T10:30:00Z",
  "total_count": 15
}
```

**聚合逻辑**：
1. 查询最新 `batch_id`（仅 `batch_id IS NOT NULL`）。
2. 获取该批次 `status=success` 且 `added_count > 0` 的日志。
3. 展开 `AddedModels`，生成扁平列表。
4. 按 `added_at` 倒序排序返回。

## 前端设计
- 页面新增 Tab 切换：`同步日志` / `最近新增模型`。
- 当切换到“最近新增模型”时调用 `getRecentAddedModels()`。
- 展示：模型名称、提供商名称、新增时间（`formatDate`）。
- 顶部小提示：最近同步时间 + 新增数量。

## 异常与空状态
- 无批次或无新增模型：显示“暂无最近新增模型”。
- API 失败：toast 错误提示。

## 迁移与兼容
- 通过 GORM 自动迁移新增 `batch_id` 字段。
- 旧日志不影响新功能。

## 测试建议
- 后端：
  - 最新批次聚合逻辑正确。
  - 无批次/无新增时返回空数据。
- 前端：
  - Tab 切换正常。
  - 空态与错误提示显示正常。

## 影响评估
- 数据库新增字段与索引。
- 新增 API 与前端展示逻辑，无破坏性变更。
