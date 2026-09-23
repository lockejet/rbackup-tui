BIN      := rbackup-tui
BUILD_DIR := bin

# 本地平台（在哪个系统上跑就编哪个）
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BIN) .

# Windows (在 MSYS2 上跑)
.PHONY: build-win
build-win:
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/windows/$(BIN).exe .

# Linux amd64 (可以在 MSYS2 上交叉编译，也可以在 Linux 上原生)
.PHONY: build-linux
build-linux:
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/linux/$(BIN) .

# Linux arm64 (树莓派等)
.PHONY: build-linux-arm64
build-linux-arm64:
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/linux/$(BIN)-arm64 .

# 全部编译
.PHONY: build-all
build-all: build-win build-linux build-linux-arm64

# 清理
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# 依赖
.PHONY: tidy
tidy:
	go mod tidy
