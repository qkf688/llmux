# LLMux 🚀

> 本项目基于 [atopos31/llmio](https://github.com/atopos31/llmio) 进行二次开发。

LLMux 是一个多供应商 LLM API 网关/代理：对外提供 **OpenAI / Anthropic 兼容**的 `/v1/*` 接口；对内提供一个 Web 管理后台，用于配置供应商、模型关联、虚拟模型、健康检查、模型同步，以及查看日志/指标。

## 适用场景 🎯

- 同一套客户端/SDK，需要在多个上游（OpenAI / Anthropic / 其他兼容供应商）之间切换
- 一个模型希望挂多个提供商：按权重分流、按优先级故障转移
- 希望把多个真实模型聚合为一个“虚拟模型”，做两层负载均衡
- 需要可视化管理、健康检查、模型同步、请求日志与使用指标

## 功能介绍（做得到的）✨

- **多供应商接入与统一出口** 🔌：对外暴露 OpenAI / Anthropic 兼容接口（`/v1/*`），内部按统一结构做请求/响应转换与适配。
- **模型与供应商关联管理** 🔗：通过 `/api/models`、`/api/providers`、`/api/model-providers` 管理真实模型、供应商与关联关系；支持批量启停与状态查询（如 `/api/model-providers/status`）。
- **自动/一键关联** 🤖：提供预览与执行接口（`/api/model-providers/auto-associate/preview`、`/api/model-providers/auto-associate`），并支持清理无效关联（`/api/model-providers/clean-invalid/*`）。
- **虚拟模型（两层负载均衡 + 故障转移）** 🌀：通过 `/api/virtual-models` 将多个真实模型聚合为一个虚拟模型；路由策略与执行细节见 `docs/virtual-models-guide.md`。
- **健康检查** 🩺：可配置、可批量运行，并提供批次进度查询（`/api/health-check/*`）。
- **模型同步** 🔄：从供应商同步模型列表，支持单个/全部触发与统计/日志查询（`/api/model-sync/*`）。
- **请求日志与回放** 📝：查询请求日志、拉取 Chat I/O、批量删除与清空（`/api/logs/*`）。
- **使用指标** 📊：按天用量与调用计数（`/api/metrics/use/:days`、`/api/metrics/counts`）。
- **配置导入导出/维护** 🛠️：导出配置、导出数据库、导入配置，以及 vacuum（`/api/system/*`、`/api/maintenance/vacuum`）。
- **参数校验与修复** ✅：对常见参数范围做校验/修复（如 temperature/top_p 等），详细见 `docs/API_PARAMETERS.md`。
- **供应商黑名单** 🚫：自动/一键关联时可跳过被拉黑供应商（`/api/providers/blacklist`；实现说明见 `docs/provider-blacklist-implementation-summary.md`）。

## 快速开始（本地）🚀

### 环境要求 📋

- Go 1.25+
- Node.js 20+（仅在需要构建 WebUI 时）
- pnpm（仅在需要构建 WebUI 时）

### 1) 构建 WebUI（首次必做；用于 Go `embed`）

```bash
make webui
# 或者
cd webui && pnpm install && pnpm run build
```

### 2) 启动服务

```bash
# 可选：不设置 TOKEN 则不启用鉴权（仅建议本地）
export TOKEN=your-auth-token
make run
```

PowerShell（Windows）：

```powershell
$env:TOKEN="your-auth-token"
go run .
# 或者（会先构建 webui，再启动服务，并尝试释放 7070 端口）
.\run.bat
```

### 3) 访问 🌐

- API：`http://localhost:7070`
- 管理界面（生产/内置页面）：`http://localhost:7070`
- 管理界面（开发模式）：`http://localhost:5173`（见 `webui/vite.config.ts`，会把 `/api` 代理到 `http://localhost:7070`）

## 认证与数据存储 🔒

- `TOKEN` 为空：不启用鉴权（仅建议本地开发）。
- OpenAI 兼容接口与管理接口：使用 `Authorization: Bearer <TOKEN>`。
- Anthropic 兼容接口：优先 `Authorization: Bearer <TOKEN>`，也兼容 `x-api-key: <TOKEN>`。
- 数据库：SQLite 文件固定在 `./db/llmio.db`（容器部署时建议挂载 `./db:/app/db`）。
- 端口：当前固定为 `7070`（见 `main.go` 的 `router.Run(":7070")`）。

## API（对外）🌍

### OpenAI 兼容

- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`

### Anthropic 兼容

- `POST /v1/messages`

### 尚未实现

- `POST /v1/count_tokens`（`main.go` 中标记为 TODO）

参数支持范围与验证/修复规则见：[docs/API_PARAMETERS.md](docs/API_PARAMETERS.md)

## 最小可用示例（curl）📡

### OpenAI Chat Completions

```bash
curl http://localhost:7070/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 64
  }'
```

### Anthropic Messages（用 `x-api-key`）

```bash
curl http://localhost:7070/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_TOKEN" \
  -d '{
    "model": "claude-3-5-sonnet-latest",
    "max_tokens": 64,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 配置流程（推荐：管理后台）⚙️

大多数用户只需要按下面顺序点一遍后台即可跑通：

1. **添加 Provider** 🔧：在“供应商”页面添加上游配置（也可以先用 `GET /api/providers/template` 获取模板）。
2. **同步模型** 🔄：在后台触发同步（`POST /api/model-sync/:id` 或 `POST /api/model-sync/all`）。
3. **建立关联** 🔗：把“真实模型”与“供应商模型”关联起来（`/api/model-providers`）。
4. **（可选）虚拟模型** 🌀：把多个真实模型聚合为一个虚拟模型，实现两层负载均衡与故障转移（`/api/virtual-models`）。

详细说明：

- [docs/virtual-models-guide.md](docs/virtual-models-guide.md)
- [docs/provider-blacklist-implementation-summary.md](docs/provider-blacklist-implementation-summary.md)

## 截图 📷

![Dashboard](docs/screenshots/dashboard.png)
![Providers](docs/screenshots/providers.png)
![Models](docs/screenshots/models.png)
![Model interaction](docs/screenshots/model-interaction.png)
![Logs](docs/screenshots/logs.png)
![Settings](docs/screenshots/settings.png)

## Docker 部署 🐳

### Docker Compose（示例）

`docker-compose.yml` 已包含示例配置（注意替换 `TOKEN` / `TZ`）：

```bash
docker compose up -d
```

### 本地构建镜像

```bash
docker build -t llmux .
docker run -d \
  -p 7070:7070 \
  -e TOKEN=your-token \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  -v ./db:/app/db \
  llmux
```

## 开发与测试 🛠️

```bash
# 后端
go test ./...
make run

# 只跑参数转换/校验相关测试（示例）
go test ./service/transform -run TestValidateUnifiedRequest -v

# 前端
cd webui
pnpm install
pnpm run lint
pnpm run build
pnpm run dev
```

## 文档 📚

- [docs/API_PARAMETERS.md](docs/API_PARAMETERS.md) - API 参数与验证规则
- [docs/virtual-models-guide.md](docs/virtual-models-guide.md) - 虚拟模型与两层负载均衡
- [docs/provider-blacklist-implementation-summary.md](docs/provider-blacklist-implementation-summary.md) - 供应商黑名单
- [CLAUDE.md](CLAUDE.md) - 项目规范与架构说明

## 许可证 📄

MIT License，见 [LICENSE](LICENSE)。

## 致谢 🙏

- 原项目：[atopos31/llmio](https://github.com/atopos31/llmio)
