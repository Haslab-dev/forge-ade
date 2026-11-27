# ForgeADE monorepo — build orchestration for all application targets.
#
# Layout: core/ (shared Go domain), app/native (MyGo UI),
# app/cli (headless tools), web (landing page). See docs/ARCHITECTURE.md.

.DEFAULT_GOAL := help
.SHELLFLAGS := -eu -o pipefail -c

# Locate go even when it is not on PATH (common install locations).
GO := $(shell command -v go 2>/dev/null || ls /Users/hy4-mac-002/homebrew/bin/go /opt/homebrew/bin/go /usr/local/go/bin/go /opt/homebrew/opt/go/bin/go 2>/dev/null | head -1)
GOPATH := $(shell $(GO) env GOPATH 2>/dev/null)
export PATH := $(dir $(GO)):/Users/hy4-mac-002/homebrew/bin:$(GOPATH)/bin:$(PATH)

# ── Current version (major.minor.patch) & build ──────────────────
# web/package.json is the single source of truth for version bumps.
PKG_JSON := web/package.json
VERSION := $(shell node -p "require('./$(PKG_JSON)').version")
BUILD := $(shell node -p "require('./$(PKG_JSON)').build || 1")

NATIVE_BIN   := build/forge-ade
NATIVE_APP   := build/ForgeADE.app
NATIVE_DIR   := app/native
CLI_BIN_DIR  := build

define BUMP
node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('$(PKG_JSON)','utf8'));const v=p.version.split('.').map(Number);$(1);p.version=v.join('.');p.build=(p.build||0)+1;fs.writeFileSync('$(PKG_JSON)',JSON.stringify(p,null,2)+'\n');console.log('version ->',p.version,'(build',p.build+')');const plistPath='$(NATIVE_DIR)/packaging/Info.plist';if(fs.existsSync(plistPath)){let plist=fs.readFileSync(plistPath,'utf8');plist=plist.replace(/<key>CFBundleShortVersionString<\\/key>\\s*<string>[^<]+<\\/string>/,'<key>CFBundleShortVersionString</key>\\n\\t<string>'+p.version+'</string>');plist=plist.replace(/<key>CFBundleVersion<\\/key>\\s*<string>[^<]+<\\/string>/,'<key>CFBundleVersion</key>\\n\\t<string>'+p.build+'</string>');fs.writeFileSync(plistPath,plist);}"
endef

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── Native (MyGo) ─────────────────────────────────────────────────
.PHONY: native
native: ## Build the native desktop binary (release flags)
	cd $(NATIVE_DIR) && $(GO) build -trimpath -ldflags="-s -w" -o ../../$(NATIVE_BIN) ./cmd/forge-ade

.PHONY: native-dev
native-dev: ## Build the native binary with debug symbols
	cd $(NATIVE_DIR) && $(GO) build -o ../../$(NATIVE_BIN) ./cmd/forge-ade

.PHONY: native-e2e
native-e2e: ## Build the native binary with the e2e automation plugin
	cd $(NATIVE_DIR) && $(GO) build -tags e2e -o ../../$(NATIVE_BIN)-e2e ./cmd/forge-ade

CODESIGN_IDENTITY ?= $(shell security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(Apple Development:[^"]*\)".*/\1/p' | head -n1)
ifeq ($(strip $(CODESIGN_IDENTITY)),)
CODESIGN_IDENTITY := -
endif

.PHONY: native-app
native-app: native ## Build the signed macOS .app bundle
	@rm -rf $(NATIVE_APP)
	@mkdir -p $(NATIVE_APP)/Contents/MacOS $(NATIVE_APP)/Contents/Resources
	cp $(NATIVE_BIN) $(NATIVE_APP)/Contents/MacOS/forge-ade
	cp $(NATIVE_DIR)/packaging/Info.plist $(NATIVE_APP)/Contents/Info.plist
	cp $(NATIVE_DIR)/packaging/AppIcon.icns $(NATIVE_APP)/Contents/Resources/AppIcon.icns
	xattr -cr $(NATIVE_APP) 2>/dev/null || true
	codesign --force --deep -s "$(CODESIGN_IDENTITY)" $(NATIVE_APP)
	@echo "signed with: $(CODESIGN_IDENTITY)"
	@echo "bundle: $(NATIVE_APP)"

.PHONY: native-run
native-run: ## Run the native app from source
	cd $(NATIVE_DIR) && $(GO) run ./cmd/forge-ade

# ── CLI ───────────────────────────────────────────────────────────
.PHONY: cli
cli: ## Build all CLI binaries (forge-agent, forge-mcp, shelltest)
	$(GO) build -trimpath -ldflags="-s -w" -o $(CLI_BIN_DIR)/forge-agent ./app/cli/forge-agent
	$(GO) build -o $(CLI_BIN_DIR)/forge-mcp ./app/cli/forge-mcp
	$(GO) build -o $(CLI_BIN_DIR)/shelltest ./app/cli/shelltest

# ── Quality ───────────────────────────────────────────────────────
.PHONY: test
test: ## Run all Go tests
	$(GO) test ./...

.PHONY: cover
cover: ## Run Go tests with coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## Run go vet on all packages
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format Go code
	$(GO) fmt ./...

.PHONY: lint
lint: ## Run golangci-lint (if installed)
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed — see .golangci.yml"

# ── Version ───────────────────────────────────────────────────────
.PHONY: version
version: ## Print the current version
	@node -p "require('./$(PKG_JSON)').version + ' (build ' + (require('./$(PKG_JSON)').build || 1) + ')'"

.PHONY: patch-version
patch-version: ## Bump patch version (1.0.19 -> 1.0.20)
	@$(call BUMP,v[2]++)

.PHONY: minor-version
minor-version: ## Bump minor version (1.0.19 -> 1.1.0)
	@$(call BUMP,v[1]++;v[2]=0)

.PHONY: major-version
major-version: ## Bump major version (1.0.19 -> 2.0.0)
	@$(call BUMP,v[0]++;v[1]=0;v[2]=0)

.PHONY: bump-build
bump-build: ## Bump the build number only
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('$(PKG_JSON)','utf8'));p.build=(p.build||0)+1;fs.writeFileSync('$(PKG_JSON)',JSON.stringify(p,null,2)+'\n');console.log('build ->',p.build)"

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf build dist coverage.out
