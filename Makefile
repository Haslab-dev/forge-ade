# ForgeADE monorepo — build orchestration for all application targets.
#
# Layout: core/ (shared Go domain), app/native (MyGo UI), app/webview (Wails),
# app/cli (headless tools), app/web (reserved). See docs/ARCHITECTURE.md.

.DEFAULT_GOAL := help
.SHELLFLAGS := -eu -o pipefail -c

# Locate go even when it is not on PATH (common install locations).
GO := $(shell command -v go 2>/dev/null || ls /usr/local/go/bin/go /opt/homebrew/opt/go/bin/go 2>/dev/null | head -1)
GOPATH := $(shell $(GO) env GOPATH 2>/dev/null)
export PATH := $(dir $(GO))$(GOPATH)/bin:$(PATH)
WAILS := $(GOPATH)/bin/wails3

# ── Current version (major.minor.patch) & build ──────────────────
# frontend/package.json is the single source of truth for version bumps.
PKG_JSON := app/webview/frontend/package.json
VERSION := $(shell node -p "require('./$(PKG_JSON)').version")
BUILD := $(shell node -p "require('./$(PKG_JSON)').build || 1")

NATIVE_BIN   := dist/forge-native
NATIVE_APP   := dist/forge-native.app
NATIVE_DIR   := app/native
WEBVIEW_DIR  := app/webview
CLI_BIN_DIR  := dist

define BUMP
node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('$(PKG_JSON)','utf8'));const v=p.version.split('.').map(Number);$(1);p.build=(p.build||0)+1;fs.writeFileSync('$(PKG_JSON)',JSON.stringify(p,null,2)+'\n');console.log('version ->',p.version,'(build',p.build+')')"
endef

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── Native (MyGo) ─────────────────────────────────────────────────
.PHONY: native
native: ## Build the native desktop binary (release flags)
	cd $(NATIVE_DIR) && $(GO) build -trimpath -ldflags="-s -w" -o ../../$(NATIVE_BIN) ./cmd/forge-native

.PHONY: native-dev
native-dev: ## Build the native binary with debug symbols
	cd $(NATIVE_DIR) && $(GO) build -o ../../$(NATIVE_BIN) ./cmd/forge-native

.PHONY: native-e2e
native-e2e: ## Build the native binary with the e2e automation plugin
	cd $(NATIVE_DIR) && $(GO) build -tags e2e -o ../../$(NATIVE_BIN)-e2e ./cmd/forge-native

.PHONY: native-app
native-app: native ## Build the signed macOS .app bundle
	@mkdir -p $(NATIVE_APP)/Contents/MacOS $(NATIVE_APP)/Contents/Resources
	cp $(NATIVE_BIN) $(NATIVE_APP)/Contents/MacOS/forge-native
	cp $(NATIVE_DIR)/packaging/Info.plist $(NATIVE_APP)/Contents/Info.plist
	codesign --force -s - $(NATIVE_APP) 2>/dev/null || true
	@echo "bundle: $(NATIVE_APP)"

.PHONY: native-run
native-run: ## Run the native app from source
	cd $(NATIVE_DIR) && $(GO) run ./cmd/forge-native

# ── Webview (Wails) ───────────────────────────────────────────────
.PHONY: webview-dev
webview-dev: ## Run the Wails app in dev mode (hot reload)
	cd $(WEBVIEW_DIR) && $(WAILS) dev

.PHONY: webview-build
webview-build: ## Build the Wails app (with devtools)
	cd $(WEBVIEW_DIR) && $(WAILS) build -devtools -tags private_mac_apis

.PHONY: webview-package
webview-package: ## Build + package the Wails .app bundle
	cd $(WEBVIEW_DIR) && $(WAILS) package

.PHONY: webview-bindings
webview-bindings: ## Regenerate Wails TS bindings from Go services
	cd $(WEBVIEW_DIR) && $(WAILS) generate bindings

.PHONY: frontend-install
frontend-install: ## Install frontend dependencies (bun)
	cd $(WEBVIEW_DIR)/frontend && bun install

.PHONY: frontend-build
frontend-build: ## Build the web frontend (tsc + vite)
	cd $(WEBVIEW_DIR)/frontend && bun run build

.PHONY: frontend-test
frontend-test: ## Run frontend tests (vitest)
	cd $(WEBVIEW_DIR)/frontend && bun run test

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
fmt: ## Format Go + frontend code
	$(GO) fmt ./...
	cd $(WEBVIEW_DIR)/frontend && bun run lint --fix && bunx prettier --write "src/**/*.{ts,tsx,css}"

.PHONY: lint
lint: ## Run golangci-lint (if installed)
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed — see .golangci.yml"

# ── Version ───────────────────────────────────────────────────────
.PHONY: version
version: ## Print the current version
	@node -p "require('./$(PKG_JSON)').version + ' (build ' + (require('./$(PKG_JSON)').build || 1) + ')'"

.PHONY: patch-version
patch-version: ## Bump patch version (0.8.6 -> 0.8.7)
	@$(call BUMP,v[2]++)

.PHONY: minor-version
minor-version: ## Bump minor version (0.8.6 -> 0.9.0)
	@$(call BUMP,v[1]++;v[2]=0)

.PHONY: major-version
major-version: ## Bump major version (0.8.6 -> 1.0.0)
	@$(call BUMP,v[0]++;v[1]=0;v[2]=0)

.PHONY: bump-build
bump-build: ## Bump the build number only
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('$(PKG_JSON)','utf8'));p.build=(p.build||0)+1;fs.writeFileSync('$(PKG_JSON)',JSON.stringify(p,null,2)+'\n');console.log('build ->',p.build)"

# ── Release (macOS) ───────────────────────────────────────────────
.PHONY: sign
sign: ## Sign the Wails app. Usage: make sign ID="Developer ID Application: Name (TEAMID)"
	codesign --force --options runtime --sign "$(ID)" \
		$(WEBVIEW_DIR)/build/bin/ForgeADE.app/Contents/MacOS/ForgeADE
	codesign --force --options runtime --sign "$(ID)" \
		$(WEBVIEW_DIR)/build/bin/ForgeADE.app
	codesign -dv $(WEBVIEW_DIR)/build/bin/ForgeADE.app

.PHONY: notarize
notarize: ## Notarize. Usage: make notarize EMAIL=… TEAM=…
	ditto -c -k --keepParent $(WEBVIEW_DIR)/build/bin/ForgeADE.app build/ForgeADE.zip
	xcrun notarytool submit build/ForgeADE.zip \
		--apple-id "$(EMAIL)" \
		--team-id "$(TEAM)" \
		--password @keychain:AC_PASSWORD \
		--wait
	xcrun stapler staple $(WEBVIEW_DIR)/build/bin/ForgeADE.app

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf dist coverage.out
	rm -rf $(WEBVIEW_DIR)/build/bin $(WEBVIEW_DIR)/frontend/dist
