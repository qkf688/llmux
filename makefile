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

# 一键自查（Stage 3 起）：格式化 + vet + 全量测试（含 gate_500_test.go 500 收口
# 机械门与 internal_error 契约测试）。改完 Go 代码提交前跑这个。
check: fmt
	go vet ./...
	go test ./... -count=1

.PHONY: webui

webui: 
	cd webui && pnpm install && pnpm run build