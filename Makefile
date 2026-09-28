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
VERSION      ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
PKG_VERSION  ?= $(VERSION)
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

# ---------- 发布 ----------
REPO  ?= $(shell git remote get-url origin 2>/dev/null | \
           sed -E 's|.*[:/]([^/]+/[^/]+?)(\.git)?$$|\1|')
NOTES ?= $(DIST_DIR)/RELEASE_NOTES.md

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
# 打包
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
# 发布（GitHub Release）
# ============================================================

# 前置检查：gh 已安装、已登录、tag 存在、仓库可解析
.PHONY: release-check
release-check:
	@command -v gh >/dev/null 2>&1 || { \
		echo "错误: 未安装 gh CLI"; \
		echo "安装: https://cli.github.com/"; \
		exit 1; \
	}
	@gh auth status >/dev/null 2>&1 || { \
		echo "错误: gh 未登录"; \
		echo "运行: gh auth login"; \
		exit 1; \
	}
	@[ -n "$(REPO)" ] || { \
		echo "错误: 无法从 git remote 解析仓库"; \
		echo "请检查: git remote -v"; \
		exit 1; \
	}
	@git rev-parse "$(VERSION)" >/dev/null 2>&1 || { \
		echo "错误: tag '$(VERSION)' 不存在"; \
		echo "先打 tag: git tag -a $(VERSION) -m '...'"; \
		echo "然后推送: git push origin $(VERSION)"; \
		exit 1; \
	}
	@echo ">>> 仓库: $(REPO)"
	@echo ">>> 版本: $(VERSION)"

# 自动生成 release notes
# 从上一个 tag 到当前 tag 之间的提交，格式为 "- <subject>"
.PHONY: release-notes
release-notes:
	@mkdir -p $(DIST_DIR)
	@echo ">>> 生成 $(NOTES)"
	@PREV_TAG=$$(git describe --tags --abbrev=0 "$(VERSION)^" 2>/dev/null || echo ""); \
	if [ -z "$$PREV_TAG" ]; then \
		echo "## $(VERSION)" > "$(NOTES)"; \
		echo "" >> "$(NOTES)"; \
		echo "首个发布版本。" >> "$(NOTES)"; \
		echo "" >> "$(NOTES)"; \
		echo "### 提交列表" >> "$(NOTES)"; \
		echo "" >> "$(NOTES)"; \
		git log --pretty="- %s" "$(VERSION)" >> "$(NOTES)"; \
	else \
		echo "## $(VERSION)" > "$(NOTES)"; \
		echo "" >> "$(NOTES)"; \
		echo "自 $$PREV_TAG 以来的改动：" >> "$(NOTES)"; \
		echo "" >> "$(NOTES)"; \
		git log --pretty="- %s" "$$PREV_TAG..$(VERSION)" >> "$(NOTES)"; \
	fi
	@echo ""
	@echo "--- $(NOTES) 内容 ---"
	@cat "$(NOTES)"
	@echo "--- 结束 ---"
	@echo ""

# 打包 + 创建 Release + 上传附件
# 不自动打 tag、不自动 push tag，由用户手动操作
.PHONY: release
release: release-check package release-notes
	@echo ""
	@echo ">>> 创建 GitHub Release: $(VERSION)"
	@if gh release view "$(VERSION)" --repo "$(REPO)" >/dev/null 2>&1; then \
		echo ">>> Release $(VERSION) 已存在，只上传附件（--clobber 覆盖同名）"; \
		gh release upload "$(VERSION)" \
		    $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz \
		    $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz \
		    $(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip \
		    --clobber \
		    --repo "$(REPO)"; \
	else \
		gh release create "$(VERSION)" \
		    $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz \
		    $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz \
		    $(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip \
		    --title "$(VERSION)" \
		    --notes-file "$(NOTES)" \
		    --repo "$(REPO)"; \
	fi
	@echo ""
	@echo ">>> 发布完成"
	@echo ">>> URL: $$(gh release view $(VERSION) --repo $(REPO) --json url --jq '.url')"

# 只上传附件（Release 已存在时用，比如重新编译后）
.PHONY: release-upload
release-upload: release-check package
	@echo ">>> 上传附件到 Release: $(VERSION)"
	@gh release upload "$(VERSION)" \
	    $(DIST_DIR)/$(PKG_NAME)-linux-amd64.tar.gz \
	    $(DIST_DIR)/$(PKG_NAME)-linux-arm64.tar.gz \
	    $(DIST_DIR)/$(PKG_NAME)-windows-amd64.zip \
	    --clobber \
	    --repo "$(REPO)"
	@echo ">>> 上传完成"

# 删除 Release（保留 tag）
.PHONY: release-delete
release-delete:
	@[ -n "$(VERSION)" ] || { echo "错误: 请指定 VERSION"; exit 1; }
	@[ -n "$(REPO)" ] || { echo "错误: 请指定 REPO"; exit 1; }
	@echo ">>> 删除 Release: $(VERSION) ($(REPO))"
	@gh release delete "$(VERSION)" --yes --repo "$(REPO)" || \
		echo ">>> Release 不存在或删除失败"
	@echo ">>> 已删除（tag 保留）"

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

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(STAGE_DIR)

.PHONY: version
version:
	@echo "BUILD_VERSION = $(BUILD_VERSION)"
	@echo "VERSION       = $(VERSION)"
	@echo "PKG_VERSION   = $(PKG_VERSION)"
	@echo "PKG_NAME      = $(PKG_NAME)"
	@echo "REPO          = $(REPO)"

.PHONY: help
help:
	@echo "rbackup-tui 构建与发布"
	@echo ""
	@echo "构建:"
	@echo "  make build               编当前平台"
	@echo "  make build-win           Windows exe"
	@echo "  make build-linux         Linux amd64"
	@echo "  make build-linux-arm64   Linux arm64"
	@echo "  make build-all           全部"
	@echo ""
	@echo "打包:"
	@echo "  make package             全部平台（生成 dist/*.tar.gz / *.zip）"
	@echo "  make package-linux       仅 Linux amd64"
	@echo "  make package-linux-arm64 仅 Linux arm64"
	@echo "  make package-win         仅 Windows amd64"
	@echo ""
	@echo "发布（GitHub Release）:"
	@echo "  make release             打包 + 创建 Release + 上传附件"
	@echo "  make release-upload      只上传附件"
	@echo "  make release-notes       只生成 notes 文件"
	@echo "  make release-delete      删除 Release（保留 tag）"
	@echo ""
	@echo "安装:"
	@echo "  make install             装到 $(PREFIX)"
	@echo "  make uninstall           卸载"
	@echo ""
	@echo "其他:"
	@echo "  make clean               清空 bin/ dist/ .staging/"
	@echo "  make version             显示版本信息"
	@echo "  make help                本帮助"
	@echo ""
	@echo "发布参数:"
	@echo "  VERSION=v1.1.2           指定版本（默认取 git 最近 tag）"
	@echo "  NOTES=xxx.md             指定 notes 路径（默认 dist/RELEASE_NOTES.md）"
	@echo "  REPO=user/repo           手动指定仓库（默认从 git remote 解析）"
