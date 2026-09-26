# ============================================================
# rbackup-tui Makefile
# ============================================================

BIN        := rbackup-tui
BUILD_DIR  := bin

# ---------- 安装路径 ----------
PREFIX     ?= $(HOME)/.local
BINDIR     ?= $(PREFIX)/bin
CONFDIR    ?= $(HOME)/rbackup
SCRIPT_SRC := $(HOME)/rbackup/rbackup.sh

# ---------- 版本信息（来自 git） ----------
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y-%m-%d)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")

LDFLAGS    := -X main.Version=$(VERSION) \
              -X main.BuildTime=$(BUILD_TIME) \
              -X main.GitCommit=$(GIT_COMMIT)

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
	@echo ">>> 版本: $(VERSION)  commit: $(GIT_COMMIT)  构建时间: $(BUILD_TIME)"

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
# 安装 / 卸载
# ============================================================

.PHONY: install
install: build
	@echo ">>> 安装 $(BIN) 到 $(DESTDIR)$(BINDIR)/"
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 "$(TARGET)" "$(DESTDIR)$(BINDIR)/$(BIN)$(SUFFIX)"
	@if [ -f "$(SCRIPT_SRC)" ]; then \
		echo ">>> 安装 rbackup.sh 到 $(DESTDIR)$(BINDIR)/"; \
		install -m 0755 "$(SCRIPT_SRC)" "$(DESTDIR)$(BINDIR)/rbackup.sh"; \
	fi
	@echo ""
	@echo "安装完成。版本 $(VERSION)"

.PHONY: uninstall
uninstall:
	-rm -f "$(DESTDIR)$(BINDIR)/$(BIN)$(SUFFIX)"
	-rm -f "$(DESTDIR)$(BINDIR)/$(BIN)"
	-rm -f "$(DESTDIR)$(BINDIR)/rbackup.sh"
	@echo "卸载完成。配置文件目录 $(CONFDIR) 未删除。"

# ============================================================
# 其他
# ============================================================

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: version
version:
	@echo "Version:    $(VERSION)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Build time: $(BUILD_TIME)"

.PHONY: help
help:
	@echo "rbackup-tui 构建"
	@echo ""
	@echo "  make build              编当前平台"
	@echo "  make build-win          Windows"
	@echo "  make build-linux        Linux amd64"
	@echo "  make build-linux-arm64  Linux arm64"
	@echo "  make build-all          全部"
	@echo ""
	@echo "  make install            安装到 $(PREFIX)"
	@echo "  make uninstall          卸载"
	@echo "  make version            显示版本"
	@echo "  make clean              清空 bin/"

# ---------- 打包 ----------
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
DIST_DIR := dist
PKG_NAME := rbackup-tui-$(VERSION)

.PHONY: package package-linux package-linux-arm64 package-win
package: package-linux package-linux-arm64 package-win
	@echo ">>> 打包完成，产物在 $(DIST_DIR)/"

package-linux: build-linux
	@mkdir -p $(DIST_DIR)
	@tar czf $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz \
	    -C bin/linux rbackup-tui
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz"

package-linux-arm64: build-linux-arm64
	@mkdir -p $(DIST_DIR)
	@tar czf $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz \
	    -C bin/linux rbackup-tui-arm64
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz"

package-win: build-win
	@mkdir -p $(DIST_DIR)
	@cd bin/windows && zip -q ../../$(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip rbackup-tui.exe
	@echo ">>> $(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip"

.PHONY: clean-dist
clean-dist:
	rm -rf $(DIST_DIR)
