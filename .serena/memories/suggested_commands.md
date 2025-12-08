# 常用命令
- 启动后端：`make run`（执行 go fmt/go mod tidy/go run .）或 `go run main.go`。
- 运行后端测试：`go test ./...`（当前主要在 handler/test_test.go）。
- 启动前端开发：`cd webui && pnpm install && pnpm run dev`。
- 前端构建/冒烟：`cd webui && pnpm run build`（或 `pnpm run preview`）。
- Docker 本地运行：`docker-compose up -d`；构建镜像：`docker build -t llmux .`。
- 运行可执行文件：`./llmio.exe`（需设置 TOKEN）。