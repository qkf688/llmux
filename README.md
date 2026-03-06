# LLMux 🚀

> 本项目基于 [atopos31/llmio](https://github.com/atopos31/llmio) 进行二次开发。

LLMux 是一个多供应商 LLM API 网关/代理：对外提供 **OpenAI / Anthropic 兼容**的 `/v1/*` 接口；对内提供一个 Web 管理后台，用于配置供应商、模型关联、虚拟模型、健康检查、模型同步，以及查看日志/指标。

## 适用场景 🎯

- 同一套客户端/SDK，需要在多个上游（OpenAI / Anthropic / 其他兼容供应商）之间切换
- 一个模型希望挂多个提供商：按权重分流、按优先级故障转移
- 希望把多个真实模型聚合为一个“虚拟模型”，做两层负载均衡
- 需要可视化管理、健康检查、模型同步、请求日志与使用指标

## 功能介绍✨

- **多供应商接入与统一出口** 🔌：对外暴露 OpenAI / Anthropic 兼容接口，内部自动完成请求/响应转换与适配。
- **模型与供应商关联管理** 🔗：可视化管理真实模型、供应商与关联关系，支持批量启停与状态查询。
- **自动/一键关联** 🤖：智能预览并执行模型与供应商的自动关联，支持清理无效关联。
- **虚拟模型（两层负载均衡 + 故障转移）** 🌀：将多个真实模型聚合为一个虚拟模型，实现灵活的路由策略（详见 `docs/virtual-models-guide.md`）。
- **健康检查** 🩺：可配置的供应商健康检查，支持批量运行与进度查询。
- **模型同步** 🔄：从供应商自动同步模型列表，支持单个/全部触发与统计查询。
- **请求日志与回放** 📝：完整记录请求日志，支持查看 Chat I/O、批量删除与清空。
- **使用指标** 📊：按天统计用量与调用次数，直观展示使用情况。
- **配置导入导出/维护** 🛠️：支持配置与数据库的导入导出，以及数据库维护操作。

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

## API（对外）🌍

### OpenAI 兼容

- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`

### Anthropic 兼容

- `POST /v1/messages`

## 截图 📷

![Dashboard](docs/screenshots/dashboard.png)
![Providers](docs/screenshots/providers.png)
![Models](docs/screenshots/models.png)
![Model interaction](docs/screenshots/model-interaction.png)
![Logs](docs/screenshots/logs.png)
![Settings](docs/screenshots/settings.png)

## Docker Compose 部署 🐳

`docker-compose.yml` 已包含示例配置（注意替换 `TOKEN` / `TZ`）：

```bash
docker compose up -d
```

```
services:
  llmux:
    image: qkf688/llmux:latest
    ports:
      - 7070:7070
    volumes:
      - ./db:/app/db
    environment:
      - GIN_MODE=release
      - TOKEN=<YOUR_TOKEN>
      - TZ=Asia/Shanghai

```



## 许可证 📄

MIT License，见 [LICENSE](LICENSE)。

## 致谢 🙏

- 原项目：[atopos31/llmio](https://github.com/atopos31/llmio)
