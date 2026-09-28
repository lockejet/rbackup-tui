# ============================================================
# rbackup-tui Makefile
# ============================================================

BIN        := rbackup-tui
BUILD_DIR  := bin
DIST_DIR   := dist
STAGE_DIR  := .staging

# ---------- 安装路径 ----------
PREFIX     ?= $(HOME)/.local
BINDIR     ?= $(PREFIX)/bin

# ---------- 打包源文件 ----------
SCRIPT_FILE  := rbackup.sh
CONFIG_FILES := config1.ini.example config2.ini.example
DOC_FILES    := README.md LICENSE

# ---------- 版本 ----------
BUILD_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PKG_VERSION  ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
PKG_NAME     := rbackup-tui-$(PKG_VERSION)

LDFLAGS := -X main.Version=$(BUILD_VERSION) \
           -X main.BuildTime=$(date +%Y-%m-%d) \
           -X main.GitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)

# ---------- 平台 ----------
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    GOOS   := linux
    SUFFIX :=
else
    GOOS   := windows
    SUFFIX := .exe
endif

TARGET := $(BUILD_DIR)/$(GOOS)/$(BIN)$(SUFFIX)

# ============================================================
# 构建
# ============================================================

.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)/$(GOOS)
	go build -ldflags "$(LDFLAGS)" -o $(TARGET) .
	@echo ">>> $(TARGET)"

.PHONY: build-win
build-win:
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
	    -o $(BUILD_DIR)/windows/$(BIN).exe .

.PHONY: build-linux
build-linux:
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" \
	    -o $(BUILD_DIR)/linux/$(BIN) .

.PHONY: build-linux-arm64
build-linux-arm64:
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" \
	    -o $(BUILD_DIR)/linux/$(BIN)-arm64 .

.PHONY: build-all
build-all: build-win build-linux build-linux-arm64

# ============================================================
# 打包（子目录方案）
# ============================================================

.PHONY: package package-linux package-linux-arm64 package-win
package: package-linux package-linux-arm64 package-win
	@echo ""
	@echo ">>> 打包完成，产物在 $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/

package-linux: build-linux
	@rm -rf $(STAGE_DIR)/$(BIN)-linux-amd64
	@mkdir -p $(STAGE_DIR)/$(BIN)-linux-amd64 $(DIST_DIR)
	@cp bin/linux/$(BIN) $(STAGE_DIR)/$(BIN)-linux-amd64/
	@cp $(SCRIPT_FILE) $(STAGE_DIR)/$(BIN)-linux-amd64/
	@cp $(CONFIG_FILES) $(STAGE_DIR)/$(BIN)-linux-amd64/
	@cp $(DOC_FILES) $(STAGE_DIR)/$(BIN)-linux-amd64/
	@chmod +x $(STAGE_DIR)/$(BIN)-linux-amd64/$(BIN)
	@chmod +x $(STAGE_DIR)/$(BIN)-linux-amd64/$(SCRIPT_FILE)
	@tar czf $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz -C $(STAGE_DIR) $(BIN)-linux-amd64
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz"

package-linux-arm64: build-linux-arm64
	@rm -rf $(STAGE_DIR)/$(BIN)-linux-arm64
	@mkdir -p $(STAGE_DIR)/$(BIN)-linux-arm64 $(DIST_DIR)
	@cp bin/linux/$(BIN)-arm64 $(STAGE_DIR)/$(BIN)-linux-arm64/$(BIN)
	@cp $(SCRIPT_FILE) $(STAGE_DIR)/$(BIN)-linux-arm64/
	@cp $(CONFIG_FILES) $(STAGE_DIR)/$(BIN)-linux-arm64/
	@cp $(DOC_FILES) $(STAGE_DIR)/$(BIN)-linux-arm64/
	@chmod +x $(STAGE_DIR)/$(BIN)-linux-arm64/$(BIN)
	@chmod +x $(STAGE_DIR)/$(BIN)-linux-arm64/$(SCRIPT_FILE)
	@tar czf $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz -C $(STAGE_DIR) $(BIN)-linux-arm64
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz"

package-win: build-win
	@rm -rf $(STAGE_DIR)/$(BIN)-windows-amd64
	@mkdir -p $(STAGE_DIR)/$(BIN)-windows-amd64 $(DIST_DIR)
	@cp bin/windows/$(BIN).exe $(STAGE_DIR)/$(BIN)-windows-amd64/
	@cp $(SCRIPT_FILE) $(STAGE_DIR)/$(BIN)-windows-amd64/
	@cp $(CONFIG_FILES) $(STAGE_DIR)/$(BIN)-windows-amd64/
	@cp $(DOC_FILES) $(STAGE_DIR)/$(BIN)-windows-amd64/
	@cd $(STAGE_DIR) && zip -qr ../$(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip $(BIN)-windows-amd64
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip"

# ============================================================
# 安装 / 卸载
# ============================================================

.PHONY: install
install: build
	@echo ">>> 安装 $(BIN) 到 $(DESTDIR)$(BINDIR)/"
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 "$(TARGET)" "$(DESTDIR)$(BINDIR)/$(BIN)$(SUFFIX)"
	@if [ -f "$(SCRIPT_FILE)" ]; then \
		install -m 0755 "$(SCRIPT_FILE)" "$(DESTDIR)$(BINDIR)/$(SCRIPT_FILE)"; \
	fi
	@echo ">>> 安装完成"

.PHONY: uninstall
uninstall:
	-rm -f "$(DESTDIR)$(BINDIR)/$(BIN)$(SUFFIX)"
	-rm -f "$(DESTDIR)$(BINDIR)/$(BIN)"
	-rm -f "$(DESTDIR)$(BINDIR)/$(SCRIPT_FILE)"
	@echo ">>> 卸载完成（未删除配置目录）"

# ============================================================
# 其他
# ============================================================

.PHONY: check-clean
check-clean:
	@if ! git diff-index --quiet HEAD --; then \
		echo "警告: 工作区有未提交改动"; \
		git status --short; \
	fi

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(STAGE_DIR)

.PHONY: version
version:
	@echo "Build version: $(BUILD_VERSION)"
	@echo "Package version: $(PKG_VERSION)"
	@echo "Git commit: $(LDFLAGS)"

.PHONY: help
help:
	@echo "rbackup-tui 构建与打包"
	@echo ""
	@echo "构建:"
	@echo "  make build               编当前平台"
	@echo "  make build-win           Windows exe"
	@echo "  make build-linux         Linux amd64"
	@echo "  make build-linux-arm64   Linux arm64"
	@echo "  make build-all           全部"
	@echo ""
	@echo "打包（生成 tar.gz / zip）:"
	@echo "  make package             全部平台"
	@echo "  make package-linux       仅 Linux amd64"
	@echo "  make package-linux-arm64 仅 Linux arm64"
	@echo "  make package-win         仅 Windows amd64"
	@echo ""
	@echo "安装:"
	@echo "  make install             装到 $(PREFIX)"
	@echo "  make uninstall           卸载"
	@echo ""
	@echo "其他:"
	@echo "  make clean               清空 bin/ dist/ .staging/"
	@echo "  make version             显示版本"
	@echo "  make help                本帮助"
