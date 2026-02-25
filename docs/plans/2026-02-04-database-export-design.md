# 数据库文件导出功能设计文档

## 文档信息
- **创建日期**: 2026-02-04
- **功能**: 在数据库管理页面添加完整数据库文件导出功能
- **影响范围**: 前端 (database.tsx, api.ts) + 后端 (handler/api.go, models)

## 1. 功能概述

### 1.1 目标
在现有的数据库管理页面 (`http://localhost:7070/database`) 添加完整 SQLite 数据库文件 (llmio.db) 导出功能，与现有的 JSON 配置导出功能并存。

### 1.2 使用场景
- **完整备份**: 用户需要备份整个数据库，包括所有表和数据
- **数据迁移**: 将数据库文件迁移到其他环境
- **灾难恢复**: 保留完整的数据库快照用于恢复
- **离线分析**: 下载数据库文件进行离线分析

### 1.3 设计原则
- **功能独立**: 不影响现有的 JSON 配置导出/导入功能
- **安全提醒**: 导出前警告用户文件包含敏感信息
- **简单高效**: 直接读取文件流，避免复杂的复制操作
- **用户友好**: 清晰的 UI 和明确的操作提示

## 2. 架构设计

### 2.1 整体架构
```
用户操作 → 前端按钮 → 警告对话框 → API调用 → 后端处理 → 文件下载
```

### 2.2 技术栈
- **前端**: React 19 + TypeScript + Radix UI
- **后端**: Go + Gin Framework
- **数据库**: SQLite 3

### 2.3 文件结构
```
webui/src/
├── routes/database.tsx          # 添加导出数据库按钮和对话框
└── lib/api.ts                   # 添加 exportDatabase API 函数

handler/
└── api.go                       # 添加 ExportDatabase 处理函数

models/
└── init.go                      # 添加 GetDBPath 辅助函数
```

## 3. 前端实现

### 3.1 UI 设计

#### 3.1.1 按钮布局
在页面顶部按钮组中添加新按钮，顺序为：
```
[返回] [刷新] [导出配置] [导出数据库] [导入] [压缩]
```

#### 3.1.2 按钮样式
```tsx
<Button
  variant="outline"
  onClick={() => setShowExportDbDialog(true)}
  disabled={exportingDb || loading}
  className="gap-2 text-xs sm:text-sm"
  size="sm"
>
  <Database className="h-3 w-3 sm:h-4 sm:w-4" />
  导出数据库
</Button>
```

#### 3.1.3 警告对话框
使用 `AlertDialog` 组件：

```tsx
<AlertDialog open={showExportDbDialog} onOpenChange={setShowExportDbDialog}>
  <AlertDialogContent>
    <AlertDialogHeader>
      <AlertDialogTitle>导出完整数据库文件</AlertDialogTitle>
      <AlertDialogDescription>
        <div className="space-y-2">
          <p>⚠️ 警告：数据库文件包含所有敏感信息，包括：</p>
          <ul className="list-disc list-inside space-y-1 ml-2">
            <li>API 密钥和访问令牌</li>
            <li>提供商配置信息</li>
            <li>模型配置和关联关系</li>
            <li>系统设置和日志</li>
          </ul>
          <p className="font-medium">请妥善保管导出的文件，避免泄露敏感信息。</p>
          <p className="text-sm text-muted-foreground">
            文件格式：SQLite 数据库 (.db)，可用于完整备份和恢复。
          </p>
        </div>
      </AlertDialogDescription>
    </AlertDialogHeader>
    <AlertDialogFooter>
      <AlertDialogCancel>取消</AlertDialogCancel>
      <AlertDialogAction onClick={handleExportDatabase}>
        确认导出
      </AlertDialogAction>
    </AlertDialogFooter>
  </AlertDialogContent>
</AlertDialog>
```

### 3.2 状态管理

```typescript
// 在 DatabasePage 组件中添加状态
const [showExportDbDialog, setShowExportDbDialog] = useState(false);
const [exportingDb, setExportingDb] = useState(false);
```

### 3.3 导出处理函数

```typescript
const handleExportDatabase = async () => {
  setShowExportDbDialog(false);
  setExportingDb(true);
  try {
    await exportDatabase();
    toast.success("数据库导出成功");
  } catch (error) {
    toast.error(`导出失败: ${error instanceof Error ? error.message : '未知错误'}`);
    console.error(error);
  } finally {
    setExportingDb(false);
  }
};
```

### 3.4 API 函数 (lib/api.ts)

```typescript
/**
 * 导出完整数据库文件
 * 文件名格式: llmio_backup_YYYYMMDD_HHMMSS.db
 */
export async function exportDatabase(): Promise<void> {
  const token = localStorage.getItem("authToken");

  const response = await fetch(`${API_BASE}/system/export-database`, {
    method: 'GET',
    headers: {
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
    },
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || '导出数据库失败');
  }

  // 从响应头获取文件名
  const contentDisposition = response.headers.get('Content-Disposition');
  const filenameMatch = contentDisposition?.match(/filename=(.+)/);
  const filename = filenameMatch ? filenameMatch[1] : 'llmio_backup.db';

  // 创建 Blob 并触发下载
  const blob = await response.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  window.URL.revokeObjectURL(url);
  document.body.removeChild(a);
}
```

## 4. 后端实现

### 4.1 API 端点

#### 4.1.1 路由注册
在路由配置中添加：
```go
api.GET("/system/export-database", ExportDatabase)
```

#### 4.1.2 处理函数 (handler/api.go)

```go
// ExportDatabase 导出完整数据库文件
func ExportDatabase(c *gin.Context) {
	// 1. 获取数据库文件路径
	dbPath := models.GetDBPath()

	// 2. 检查文件是否存在并获取文件信息
	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		common.NotFound(c, "数据库文件不存在")
		return
	}
	if err != nil {
		slog.Error("无法访问数据库文件", "error", err, "path", dbPath)
		common.InternalServerError(c, "无法访问数据库文件")
		return
	}

	// 3. 检查文件大小（可选警告）
	if fileInfo.Size() > 100*1024*1024 { // 100MB
		slog.Warn("导出的数据库文件较大", "size", fileInfo.Size(), "path", dbPath)
	}

	// 4. 生成带时间戳的文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("llmio_backup_%s.db", timestamp)

	// 5. 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// 6. 记录导出操作
	slog.Info("数据库导出", "filename", filename, "size", fileInfo.Size())

	// 7. 直接发送文件
	c.File(dbPath)
}
```

### 4.2 数据库路径获取 (models/init.go)

```go
// GetDBPath 返回当前数据库文件的绝对路径
func GetDBPath() string {
	// 如果有配置文件，从配置读取
	// 否则返回默认路径
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./llmio.db"
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		slog.Warn("无法获取数据库绝对路径，使用相对路径", "path", dbPath, "error", err)
		return dbPath
	}

	return absPath
}
```

## 5. 错误处理

### 5.1 前端错误处理

| 错误类型 | 处理方式 |
|---------|---------|
| 网络错误 | 显示 "网络连接失败，请检查网络后重试" |
| 404 错误 | 显示 "数据库文件不存在，请联系管理员" |
| 403 错误 | 显示 "没有权限访问数据库文件" |
| 500 错误 | 显示 "服务器内部错误，请稍后重试" |
| 下载中断 | 浏览器自动处理，用户可重新尝试 |

### 5.2 后端错误处理

```go
// 错误场景及处理
1. 文件不存在: 返回 404 + "数据库文件不存在"
2. 权限不足: 返回 500 + "无法访问数据库文件"
3. 文件读取失败: 返回 500 + "读取数据库文件失败"
4. 文件过大: 记录警告日志，但继续导出
```

### 5.3 日志记录

```go
// 成功导出
slog.Info("数据库导出", "filename", filename, "size", fileInfo.Size())

// 文件过大警告
slog.Warn("导出的数据库文件较大", "size", fileInfo.Size(), "path", dbPath)

// 错误日志
slog.Error("无法访问数据库文件", "error", err, "path", dbPath)
```

## 6. 安全考虑

### 6.1 敏感信息保护
- **警告提示**: 导出前明确警告用户文件包含敏感信息
- **用户责任**: 由用户负责保管导出的文件
- **不做脱敏**: 保持数据完整性，不自动清除敏感字段

### 6.2 并发安全
- **SQLite 多读**: 导出时不会阻塞正常的读操作
- **写操作影响**: 导出期间如有写操作，文件内容可能不完全一致，但不会损坏
- **建议时机**: 建议在系统空闲时导出，但不强制要求

### 6.3 权限控制
- **文件访问**: 后端需要有读取数据库文件的权限
- **API 认证**: 如果系统有认证机制，需要验证用户权限
- **路径安全**: 使用绝对路径，避免路径遍历攻击

## 7. 测试计划

### 7.1 功能测试
- [ ] 点击"导出数据库"按钮，显示警告对话框
- [ ] 点击"取消"，关闭对话框，不执行导出
- [ ] 点击"确认导出"，成功下载数据库文件
- [ ] 验证下载的文件名格式正确 (llmio_backup_YYYYMMDD_HHMMSS.db)
- [ ] 验证下载的文件可以用 SQLite 工具打开
- [ ] 验证文件内容完整（所有表和数据）

### 7.2 错误测试
- [ ] 数据库文件不存在时，显示错误提示
- [ ] 网络中断时，显示错误提示
- [ ] 文件权限不足时，显示错误提示
- [ ] 导出过程中刷新页面，不影响系统稳定性

### 7.3 并发测试
- [ ] 导出期间执行读操作，验证不受影响
- [ ] 导出期间执行写操作，验证不会崩溃
- [ ] 同时多个用户导出，验证不会冲突

### 7.4 性能测试
- [ ] 小数据库 (<10MB): 导出时间 <1秒
- [ ] 中等数据库 (10-50MB): 导出时间 <5秒
- [ ] 大数据库 (>50MB): 显示警告，但能正常导出

## 8. 实施步骤

### 8.1 后端实现
1. 在 `models/init.go` 添加 `GetDBPath()` 函数
2. 在 `handler/api.go` 添加 `ExportDatabase()` 函数
3. 在路由配置中注册新端点
4. 测试 API 端点是否正常工作

### 8.2 前端实现
1. 在 `lib/api.ts` 添加 `exportDatabase()` 函数
2. 在 `database.tsx` 添加状态和处理函数
3. 添加"导出数据库"按钮
4. 添加警告对话框组件
5. 测试完整流程

### 8.3 集成测试
1. 启动后端服务
2. 启动前端开发服务器
3. 访问 `/database` 页面
4. 执行完整的导出流程
5. 验证下载的文件

### 8.4 文档更新
1. 更新用户手册，说明数据库导出功能
2. 更新 API 文档，记录新端点
3. 更新 CHANGELOG

## 9. 文件清单

### 9.1 需要修改的文件
- `webui/src/routes/database.tsx` - 添加按钮和对话框
- `webui/src/lib/api.ts` - 添加 API 函数
- `handler/api.go` - 添加处理函数
- `models/init.go` - 添加辅助函数
- `main.go` 或路由配置文件 - 注册新路由

### 9.2 需要创建的文件
- 无（所有修改都在现有文件中）

## 10. 兼容性

### 10.1 浏览器兼容性
- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

### 10.2 操作系统兼容性
- Windows 10+
- macOS 10.15+
- Linux (主流发行版)

### 10.3 数据库兼容性
- SQLite 3.x

## 11. 未来扩展

### 11.1 可能的增强功能
- 定时自动备份
- 备份文件加密
- 备份文件压缩 (gzip)
- 备份历史管理
- 远程备份存储 (S3, OSS 等)
- 增量备份

### 11.2 暂不实现的功能
- 数据库文件导入（恢复）- 风险较高，需要更复杂的设计
- 自动脱敏 - 可能破坏数据完整性
- 备份加密 - 增加复杂度，用户可自行加密

## 12. 总结

本设计方案在保持现有功能不变的前提下，添加了完整数据库文件导出功能。通过清晰的警告提示和简单的实现方式，为用户提供了便捷的数据库备份能力。设计遵循项目规范，注重安全性和用户体验，实施步骤明确，易于开发和测试。
