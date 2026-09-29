mkdb:
	mkdir -p db

tidy:
	go mod tidy

fmt:
	go fmt ./...

run: fmt tidy mkdb
	go run .

add: fmt tidy
	git add .

# 全量 Go 测试（含 gate_500 / 上游状态码口径等机械门）。改完 Go 代码想单跑测试用这个。
test:
	go test ./... -count=1

# 一键自查（Stage 3 起）：格式化 + vet + 全量测试。改完 Go 代码提交前跑这个。
check: fmt
	go vet ./...
	$(MAKE) test

.PHONY: webui test check

webui: 
	cd webui && pnpm install && pnpm run build