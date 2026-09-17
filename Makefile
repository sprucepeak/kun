.PHONY: install build build-all build-linux build-darwin build-windows upx vet test fmt lint clean

# ── 本地安装（带 -ldflags="-s -w" 压缩瘦身，去除调试符号）──
install:
	go install -ldflags="-s -w" .

# ── 本地平台快速构建（含全量数据库驱动，CGO_ENABLED=0，纯静态瘦身）──
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/kun .

# ── 跨平台全编译：内置全量驱动（MySQL / Postgres / SQLite / ClickHouse），纯静态 ──
build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/kun-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/kun-linux-arm64 .

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/kun-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ./bin/kun-darwin-arm64 .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/kun-windows-amd64.exe .

# ── UPX 可选二次极致压缩（进一步缩减约 60% 体积，需系统安装 upx）──
upx:
	@which upx > /dev/null 2>&1 && upx --best --lzma ./bin/kun* || echo "upx not installed, skipped"

# 静态检查
vet:
	go vet ./...

# 打包模板 zip 文件
pack-tpl:
	go run pack.go

# 格式化(仅脚手架自身代码,不含 tpl/ 生成目标模板)
fmt:
	gofmt -w main.go cmd/ internal/ config/ pkg/ tpl/embed.go

# 整理依赖
lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipped"

clean:
	rm -rf ./bin
