# 项目概览
- 目标：多供应商 LLM API 代理服务（OpenAI/Anthropic 兼容），提供负载均衡、权重分配、监控日志和管理后台。
- 后端：Go 1.25、Gin、GORM、SQLite。入口 `main.go`；业务/请求/中间件/供应商适配分别位于 `service/` `handler/` `middleware/` `providers/`；公共工具在 `common/`；负载策略在 `balancer/`；数据模型在 `models/`。
- 前端：React 19 + TypeScript + Tailwind + Vite，代码在 `webui/`，构建产物 `webui/dist/` 由 Go 服务托管。
- 数据库：SQLite 文件 `db/llmio.db`（GORM 实体在 `models/`）。
- 文档与运维：`docs/`；打包/镜像脚本 `docker*` 与 `makefile`、`run.bat`/`run.sh`。
- 认证与配置：依赖环境变量 `TOKEN`，端口默认 7070，可通过 Docker/Compose 部署。