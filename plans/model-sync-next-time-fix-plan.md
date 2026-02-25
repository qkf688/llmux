# 模型同步日志"下次"时间显示问题修复计划

## 问题描述

用户报告"模型同步日志"页面显示的时间戳"上次: 2025/12/26 21:49:35 · 下次: 2025/12/26 22:49:35"没有更新，而当前时间已经是2025/12/27了。

## 问题根源分析

### 代码位置
- 后端 API: E:\a-2025-11-projects-git\llmio\handler\api.go:2618-2720 (GetModelSyncStats 函数)
- 自动同步: E:\a-2025-11-projects-git\llmio\service\model_sync.go:165-195 (checkAndSync 函数)

### 问题原因

在 GetModelSyncStats 函数中，"下次"同步时间的计算逻辑如下：

`go
// 计算下次同步时间
var nextSyncAt *time.Time
if syncEnabled && lastLog.SyncedAt.IsZero() == false {
    next := lastLog.SyncedAt.Add(time.Duration(syncInterval) * time.Hour)
    nextSyncAt = &next
}
`

核心问题：nextSyncAt 是基于最后一次同步日志的时间戳 (lastLog.SyncedAt) 计算的静态值，而不是基于当前时间动态计算的。

实际影响：
- 上次同步：2025/12/26 21:49:35
- 同步间隔：1小时
- 计算结果：下次 = 21:49:35 + 1小时 = 22:49:35
- 当前时间：2025/12/27（已经过去一天多）

即使当前时间已经远超计划的下次同步时间，如果没有实际执行新的同步操作，lastLog.SyncedAt 仍然是 21:49:35，导致显示的"下次"时间也停留在过去。

## 修复方案

### 方案：基于当前时间动态计算下次同步时间

修改 GetModelSyncStats 函数中的时间计算逻辑，让"下次"时间始终正确反映应该在什么时候同步，而不是基于历史记录的静态值。

### 修改代码

文件：E:\a-2025-11-projects-git\llmio\handler\api.go

位置：第 2648-2654 行

原代码：
`go
// 计算下次同步时间
var nextSyncAt *time.Time
if syncEnabled && lastLog.SyncedAt.IsZero() == false {
    next := lastLog.SyncedAt.Add(time.Duration(syncInterval) * time.Hour)
    nextSyncAt = &next
}
`

修改为：
`go
// 计算下次同步时间（基于当前时间动态计算）
var nextSyncAt *time.Time
if syncEnabled {
    if lastLog.SyncedAt.IsZero() {
        // 如果从未同步过，下次同步就是现在
        now := time.Now()
        nextSyncAt = &now
    } else {
        // 计算从上次同步开始，经过多少个间隔周期
        elapsed := time.Since(lastLog.SyncedAt)
        intervalsElapsed := int(elapsed.Hours()) / syncInterval
        // 下次同步时间 = 上次同步 + (已过周期数 + 1) * 间隔
        next := lastLog.SyncedAt.Add(time.Duration(intervalsElapsed+1) * time.Duration(syncInterval) * time.Hour)
        nextSyncAt = &next
    }
}
`

### 逻辑说明

1. 从未同步过：显示"现在"作为下次同步时间
2. 已同步过：
   - 计算从上次同步到现在经过了多少个完整的间隔周期
   - 下次同步 = 上次同步时间 + (已过周期数 + 1) × 间隔时间

示例：
- 上次同步：2025/12/26 21:49:35
- 同步间隔：1小时
- 当前时间：2025/12/27 10:00:00
- 经过时间：约 12 小时 10 分钟
- 已过周期：12
- 下次同步：21:49:35 + (12+1)×1小时 = 2025/12/27 10:49:35

## 验证步骤

1. 修改代码后重新编译运行
2. 访问"模型同步日志"页面
3. 检查"下次"时间是否正确显示为未来的时间
4. 等待或手动触发一次同步
5. 刷新页面，确认时间更新

## 影响范围

- 仅修改一个函数的时间计算逻辑
- 不涉及数据库结构变更
- 不影响同步功能本身
- 前端无需修改

## 文件修改清单

| 文件 | 操作 | 行号 |
|------|------|------|
| handler/api.go | 修改 GetModelSyncStats 函数中的时间计算逻辑 | 2648-2654 |
