# 完成任务前需做的事
- 后端改动：运行 `go test ./...` 确认通过；必要时 `go fmt` 保持格式。
- 前端改动：运行 `cd webui && pnpm run build`（或 `pnpm run preview`）验证构建；视情况 `pnpm run lint`。
- 如涉及 Docker，确认 `docker-compose up -d`/`docker build` 脚本执行正常。
- 检查未泄露 TOKEN/API Key，配置放入环境变量或 .env（不入仓库）。
- 不要主动提交或修改 git 历史；保持现有未提交改动。