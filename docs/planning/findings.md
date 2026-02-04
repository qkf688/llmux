# 发现和研究记录

## 项目结构分析

### 后端结构
```
handler/
├── api.go          # API 处理函数
├── chat.go         # 聊天功能
├── home.go         # 主页和静态资源
└── test.go         # 测试相关

models/
└── init.go         # 数据库初始化

service/
├── chat.go         # 聊天业务逻辑
└── balancer.go     # 负载均衡

providers/
└── provider.go     # LLM 提供商接口
```

### 前端结构
```
webui/src/
├── routes/
│   └── database.tsx    # 数据库管理页面
├── lib/
│   └── api.ts          # API 客户端
└── components/
    └── ui/             # UI 组件库
```

---

## 现有功能分析

### 1. 数据库管理页面 (`database.tsx`)
**当前功能**:
- 显示数据库统计信息（文件大小、使用率、表统计等）
- JSON 配置导出（提供商、模型、关联、模板、设置）
- JSON 配置导入（合并/覆盖模式）
- 数据库压缩（VACUUM）
- 文件预览功能

**按钮布局**:
```
[返回] [刷新] [导出] [导入] [压缩]
```

**需要调整**:
- 在"导出"和"导入"之间添加"导出数据库"按钮
- 新布局: `[返回] [刷新] [导出配置] [导出数据库] [导入] [压缩]`

### 2. API 端点 (`handler/api.go`)
**现有导出相关端点**:
- `GET /api/system/export-config` - 导出 JSON 配置

**需要添加**:
- `GET /api/system/export-database` - 导出数据库文件

### 3. 数据库初始化 (`models/init.go`)
**当前实现**:
- `Init(ctx, path)` - 初始化数据库连接
- 数据库路径通过参数传入
- 使用 GORM + SQLite

**需要添加**:
- `GetDBPath()` - 获取当前数据库文件路径

---

## 技术要点

### 1. Go 文件下载实现
```go
// 设置响应头
c.Header("Content-Description", "File Transfer")
c.Header("Content-Transfer-Encoding", "binary")
c.Header("Content-Disposition", "attachment; filename=xxx.db")
c.Header("Content-Type", "application/octet-stream")

// 发送文件
c.File(dbPath)
```

### 2. TypeScript 文件下载实现
```typescript
// 获取 Blob
const blob = await response.blob();

// 创建下载链接
const url = window.URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = filename;
document.body.appendChild(a);
a.click();

// 清理
window.URL.revokeObjectURL(url);
document.body.removeChild(a);
```

### 3. SQLite 并发安全
- SQLite 支持多个读操作同时进行
- 导出时读取文件不会阻塞其他读操作
- 如果导出期间有写操作，文件内容可能不完全一致，但不会损坏
- 不需要特殊的锁机制

---

## 设计决策

### 1. 为什么不创建临时副本？
**原因**:
- 增加磁盘 I/O 和空间占用
- 增加代码复杂度
- SQLite 支持多读，直接读取是安全的
- 用户可以选择在系统空闲时导出

### 2. 为什么不自动脱敏？
**原因**:
- 可能破坏数据完整性
- 用户需要完整备份用于恢复
- 通过警告对话框提醒用户即可
- 用户可以自行决定如何保管文件

### 3. 为什么使用独立按钮？
**原因**:
- 清晰明确，用户一眼就能看到
- 不影响现有的"导出配置"功能
- 避免增加交互复杂度（下拉菜单、对话框选项等）

---

## 依赖关系

### 后端依赖
- `github.com/gin-gonic/gin` - Web 框架
- `gorm.io/gorm` - ORM
- `github.com/glebarez/sqlite` - SQLite 驱动
- `log/slog` - 日志库

### 前端依赖
- `react` - UI 框架
- `@radix-ui/react-alert-dialog` - 警告对话框组件
- `sonner` - Toast 提示库
- `lucide-react` - 图标库

---

## 安全考虑

### 1. 路径安全
- 使用 `filepath.Abs()` 获取绝对路径
- 避免路径遍历攻击
- 不接受用户输入的路径参数

### 2. 敏感信息
- 数据库文件包含 API 密钥、配置等敏感信息
- 通过警告对话框明确提醒用户
- 由用户负责保管导出的文件

### 3. 权限控制
- 后端需要有读取数据库文件的权限
- 如果系统有认证机制，需要验证用户权限
- 当前实现假设本地部署，无需额外认证

---

## 测试策略

### 单元测试
- 后端: 测试 `GetDBPath()` 函数
- 后端: 测试 `ExportDatabase()` 处理函数
- 前端: 测试 `exportDatabase()` API 函数

### 集成测试
- 完整导出流程测试
- 错误场景测试（文件不存在、权限不足等）
- 并发测试（导出期间执行读写操作）

### 手动测试
- UI 交互测试
- 文件下载测试
- 文件完整性验证（用 SQLite 工具打开）

---

## 参考资料

### 项目规范
- [CLAUDE.md](../../CLAUDE.md) - 项目编码规范
- [设计文档](../plans/2026-02-04-database-export-design.md) - 详细设计方案

### 技术文档
- [Gin 文件下载](https://gin-gonic.com/docs/examples/upload-file/)
- [SQLite 并发](https://www.sqlite.org/lockingv3.html)
- [Fetch API - Blob](https://developer.mozilla.org/en-US/docs/Web/API/Blob)
