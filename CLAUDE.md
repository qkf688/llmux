# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

LLMux 是一个多供应商 LLM API 网关/代理服务，基于 [atopos31/llmio](https://github.com/atopos31/llmio) 二次开发。对外提供 OpenAI/Anthropic 兼容的 `/v1/*` 接口，对内提供 Web 管理后台。

**核心功能**：
- 多供应商统一接入（OpenAI/Anthropic 等）
- 模型与供应商关联管理
- 虚拟模型（两层负载均衡 + 故障转移）
- 健康检查、模型同步、请求日志与指标

## 常用命令

### 开发环境

```bash
# 构建前端（首次必做，用于 Go embed）
make webui
# 或
cd webui && pnpm install && pnpm run build

# 格式化 + 整理依赖 + 创建 db 目录 + 启动服务（默认端口 7070）
make run
# 或
go run .

# 格式化代码
make fmt

# 整理依赖
make tidy
```

### 前端开发

```bash
cd webui

# 安装依赖
pnpm install

# 开发模式（端口 5173，代理 /api 到 localhost:7070）
pnpm dev

# 构建生产版本
pnpm run build

# 代码检查
pnpm lint

# 运行测试
pnpm test
pnpm test:watch
```

### Windows 脚本

```powershell
.\run.bat        # 构建 webui + 启动服务（自动释放 7070 端口）
.\build.bat      # 构建 webui + 编译 Go 二进制
```

### 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./service/virtualmodel/...

# 运行单个测试
go test -run TestFunctionName ./path/to/package

# 前端测试（Vitest + Testing Library + jsdom）
cd webui && pnpm test
```

### Docker

```bash
# 使用 docker-compose 部署
docker compose up -d

# 查看日志
docker compose logs -f
```

## 架构设计

### 后端分层架构

```
main.go                       # 入口：路由注册、服务启动
├── handler/                  # HTTP 处理层（Gin handlers）
│   ├── chat.go              # 聊天请求处理
│   ├── api_model.go         # 模型管理 API
│   ├── api_virtual_models.go  # 虚拟模型 API
│   ├── api_association.go   # 模型-供应商关联 API
│   ├── api_health.go        # 健康检查 API
│   ├── api_settings.go      # 设置 API
│   ├── modelapi/            # 模型相关子包
│   ├── autoassoc/           # 自动关联子包
│   ├── settings/            # 设置相关子包
│   └── testapi/             # 测试 API 子包
├── service/                 # 业务逻辑层（核心引擎）
│   ├── chat/                # 聊天请求处理（重试、日志、查找供应商）
│   │   └── preprocess/openai/  # OpenAI 消息预处理（tool_call_ids 等）
│   ├── transform/           # ★ 协议转换核心（40+ 文件）
│   │   ├── transformer.go   # 转换引擎入口
│   │   ├── transform_openai_request.go / response.go / stream_*.go
│   │   ├── transform_anthropic.go
│   │   ├── transform_responses.go（OpenAI Responses API ↔ 统一格式）
│   │   └── golden_test.go   # 协议转换金测试
│   ├── anthropic/           # Anthropic 协议适配
│   ├── responses/           # OpenAI Responses API 格式处理
│   ├── virtualmodel/        # 虚拟模型服务（加权随机、轮询、优先级）
│   ├── healthcheck/         # 健康检查服务
│   ├── modelsync/           # 模型同步服务
│   └── chatcore/            # 聊天核心工具（请求头、模型选择）
├── providers/               # 供应商实现层
│   ├── provider.go          # Provider 接口定义
│   ├── openai.go            # OpenAI 实现
│   └── anthropic.go         # Anthropic 实现
├── models/                  # 数据模型层（GORM）
│   ├── init.go              # 数据库初始化
│   ├── model.go             # 数据库模型定义
│   └── unified/             # 统一请求/响应格式
├── middleware/              # 中间件
│   └── auth.go              # 认证中间件（JWT + per-user API key）
├── balancer/                # 负载均衡器
├── common/                  # 公共工具
│   └── response.go          # 统一响应格式
└── consts/                  # 常量定义
```

### 前端架构

```
webui/
├── src/
│   ├── main.tsx              # 入口
│   ├── App.tsx               # 路由配置（react-router-dom v7）
│   ├── routes/               # 页面路由组件（每页一个文件夹）
│   │   ├── home/             # 主页（Dashboard）
│   │   ├── providers/        # 供应商管理
│   │   ├── models/           # 模型管理
│   │   ├── model-providers/  # 模型-供应商关联
│   │   ├── virtual-models/   # 虚拟模型管理
│   │   ├── logs/             # 请求日志
│   │   ├── model-sync-logs/  # 模型同步日志
│   │   ├── database/         # 数据库管理
│   │   └── settings/         # 系统设置
│   ├── components/           # 可复用组件
│   │   ├── ui/               # 基础 UI（封装 Radix UI + Tailwind + CVA）
│   │   ├── charts/           # 图表组件（Recharts）
│   │   └── theme-provider.tsx # next-themes 暗色模式
│   ├── lib/                  # 工具函数
│   │   ├── api.ts            # API 调用门面
│   │   ├── api/              # 按模块拆分的 API 层
│   │   │   ├── core/client.ts          # 基础 HTTP 客户端
│   │   │   ├── core/download.ts        # 文件下载
│   │   │   └── modules/
│   │   │       ├── catalog/            # 供应商/模型/关联/虚拟模型 CRUD
│   │   │       ├── logs/               # 聊天日志
│   │   │       └── system/             # 指标/健康检查/同步/系统设置
│   │   ├── utils.ts          # 通用工具
│   │   ├── formatters.ts     # 格式化函数
│   │   ├── errors.ts         # 错误处理
│   │   └── provider-models.ts # 供应商模型数据处理
│   ├── hooks/                # 自定义 Hooks
│   └── types/                # TypeScript 类型定义
└── vite.config.ts            # Vite 7 + Tailwind CSS 4 + SWC
```

**技术栈**：
- **构建**: Vite 7 + `@vitejs/plugin-react-swc`
- **样式**: Tailwind CSS 4 + `tw-animate-css` + `class-variance-authority`
- **UI 组件**: Radix UI（Dialog/Select/Checkbox 等）+ Lucide React 图标
- **路由**: react-router-dom v7
- **状态管理**: zustand
- **表单**: react-hook-form + zod
- **图表**: recharts
- **路径别名**: `@` → `src/`

**路由页面组织模式**（以 `models/` 为例）：每个页面文件夹下包含：
- `components/sections/` — 桌面/移动端分段组件
- `components/dialogs/` — 弹窗组件
- `components/shared/` — 页面内共享组件
- `utils/` — 页面级工具函数
- `schemas/` / `types/` — 表单校验和类型定义

### 关键数据流

**聊天请求流程**：
1. `handler/chat.go` 接收 `/v1/chat/completions` 或 `/v1/messages` 请求
2. `service/chat/` 根据模型名称选择：
   - 真实模型 → 通过 `balancer/` 选择供应商 → `providers/` 发起上游请求
   - 虚拟模型 → `service/virtualmodel/` 选择真实模型 → 递归处理
3. 流式响应通过 SSE 返回，非流式直接返回 JSON
4. 请求日志存储到 SQLite（`models.ChatLog`）

**虚拟模型负载均衡**：
- `service/virtualmodel/service.go` 实现多种策略：
  - `weighted_random.go`：加权随机
  - `round_robin.go`：轮询
  - `select_ordered.go`：优先级顺序
  - `select_single.go`：单一模型

**模型同步**：
- `service/modelsync/` 从供应商 API 拉取模型列表
- 支持自动同步（定时任务）和手动触发
- 过滤规则可配置（白名单/黑名单）

## 重要约定

### 数据库

- **ORM**: GORM
- **数据库文件**: `./db/llmio.db`（SQLite）
- **自动迁移**: `models/init.go` 中的 `Init()` 函数
- **主要表**：
  - `providers`: 供应商配置
  - `models`: 真实模型
  - `model_providers`: 模型-供应商关联（多对多）
  - `virtual_models`: 虚拟模型
  - `virtual_model_mappings`: 虚拟模型映射
  - `chat_logs`: 请求日志
  - `settings`: 系统设置（JSON 存储）

### 认证

- 管理后台 `/api/*`：JWT 登录（用户名固定 `admin`，密码输入）
  - 登录：`POST /api/auth/login`（请求体 `{password}`，返回 `{token, user}`）
  - JWT 签名密钥由环境变量 `JWT_SECRET` 提供（必填）
  - 首启动 bootstrap admin 账号，密码由 `ADMIN_PASSWORD` 指定（未设则随机生成并打印日志）
  - API key 轮换 / 改密码：settings 页"账户" tab
- 代理 `/v1/*`：per-user API key
  - OpenAI 格式：`Authorization: Bearer <API_KEY>`
  - Anthropic 格式：`x-api-key: <API_KEY>`

### 前端嵌入

- 前端构建产物通过 `//go:embed webui/dist` 嵌入到 Go 二进制
- 生产环境直接访问 `http://localhost:7070` 即可使用管理界面
- 开发环境前端独立运行在 `http://localhost:5173`，通过 Vite 代理访问后端

### 协议转换

协议转换在两层完成，核心引擎位于 `service/transform/`：

- `service/transform/transformer.go` — **转换引擎入口**，内部分发到各格式处理器
- `service/transform/transform_anthropic.go` — Anthropic ↔ 统一格式
- `service/transform/transform_openai_request.go` / `transform_openai_response.go` — OpenAI Chat API ↔ 统一格式
- `service/transform/transform_openai_stream_*.go` — 流式响应转换（Legacy / Realtime / Responses 流）
- `service/transform/transform_responses.go` — OpenAI Responses API ↔ 统一格式
- `service/anthropic/` — Anthropic 特有的请求/响应编解码（入口/出口适配）
- `service/responses/` — OpenAI Responses API 格式编解码
- `models/unified/` — 统一格式定义（所有协议归一化为该格式）

**转换矩阵**：任何外部格式（OpenAI Chat / OpenAI Responses / Anthropic）都能互转，流式响应通过 SSE 实现。

## 开发注意事项

1. **修改前端后必须重新构建**：前端代码通过 `embed` 嵌入，修改后需运行 `make webui` 重新构建
2. **数据库迁移自动执行**：启动时 GORM 会自动执行 `AutoMigrate`
3. **日志使用 slog**：使用标准库 `log/slog` 进行结构化日志记录
4. **并发安全**：虚拟模型的轮询计数器使用 `sync.Mutex` 保护
5. **测试隔离**：测试文件使用 `_test.go` 后缀，测试数据库使用内存模式（`:memory:`）

## 环境变量

支持 `.env` 文件加载（启动时自动读取项目根 `.env`，文件不存在则跳过；模板见 `.env.example`）。

- `JWT_SECRET`: 管理后台 JWT 签名密钥（**必填**）
- `ADMIN_PASSWORD`: 首启动 admin 密码（未设则随机生成并打印日志）
- `GIN_MODE`: Gin 运行模式（`debug`/`release`/`test`）
- `TZ`: 时区设置（如 `Asia/Shanghai`）

## 相关文档

- `README.md`: 项目介绍和快速开始
- `AGENTS.md`: Agent 相关文档（如果存在）
