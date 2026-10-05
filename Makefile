.PHONY: dev build build-prod sign notarize clean shell-test test vet fmt version patch-version minor-version major-version

# Locate go even when it is not on PATH (common install locations).
GO := $(shell command -v go 2>/dev/null || ls /usr/local/go/bin/go /opt/homebrew/opt/go/bin/go 2>/dev/null | head -1)
GOPATH := $(shell $(GO) env GOPATH 2>/dev/null)
export PATH := $(dir $(GO))$(GOPATH)/bin:$(PATH)
WAILS := $(GOPATH)/bin/wails3

# ── Current version (major.minor.patch) ──────────────────────────
# Read from frontend/package.json (single source of truth for the bump).
VERSION := $(shell node -p "require('./frontend/package.json').version")

# ── Version bumping ──────────────────────────────────────────────
# Bump the patch (bugfix): 0.5.0 -> 0.5.1
patch-version:
	@node -e "const fs=require('fs');const v=require('./frontend/package.json').version.split('.').map(Number);v[2]++;fs.writeFileSync('./frontend/package.json',fs.readFileSync('./frontend/package.json','utf8').replace(/\""version\"": \"[^\"]+\"/,'\""version\"": \"'+v.join('.')+'\"'));console.log('patched version ->',v.join('.'))"

# Bump the minor (feature): 0.5.0 -> 0.6.0
minor-version:
	@node -e "const fs=require('fs');const v=require('./frontend/package.json').version.split('.').map(Number);v[1]++;v[2]=0;fs.writeFileSync('./frontend/package.json',fs.readFileSync('./frontend/package.json','utf8').replace(/\""version\"": \"[^\"]+\"/,'\""version\"": \"'+v.join('.')+'\"'));console.log('minored version ->',v.join('.'))"

# Bump the major (breaking): 0.5.0 -> 1.0.0
major-version:
	@node -e "const fs=require('fs');const v=require('./frontend/package.json').version.split('.').map(Number);v[0]++;v[1]=0;v[2]=0;fs.writeFileSync('./frontend/package.json',fs.readFileSync('./frontend/package.json','utf8').replace(/\""version\"": \"[^\"]+\"/,'\""version\"": \"'+v.join('.')+'\"'));console.log('majored version ->',v.join('.'))"

# After bumping package.json, the version is read from it directly
# (build/config.yml `info.version` is the packaging source of truth).
version:
	@node -p "require('./frontend/package.json').version"

# ── Development ──────────────────────────────────────────────────
dev:
	$(WAILS) dev

# ── Production build (with devtools for debugging) ──────────────
build:
	$(WAILS) build -devtools

# ── Production build (without devtools, smaller binary) ──────────
build-prod:
	$(WAILS) build

# ── Apple Developer signing (requires certificate) ──────────────
# Usage: make sign ID="Developer ID Application: Your Name (TEAMID)"
sign:
	codesign --force --options runtime --sign "$(ID)" \
		build/bin/forge-ade.app/Contents/MacOS/forge-ade
	codesign --force --options runtime --sign "$(ID)" \
		build/bin/forge-ade.app
	codesign -dv build/bin/forge-ade.app

# ── Notarize (requires Apple Developer account) ─────────────────
# Usage: make notarize EMAIL="you@email.com" TEAM="TEAMID"
notarize:
	ditto -c -k --keepParent build/bin/forge-ade.app build/forge-ade.zip
	xcrun notarytool submit build/forge-ade.zip \
		--apple-id "$(EMAIL)" \
		--team-id "$(TEAM)" \
		--password @keychain:AC_PASSWORD \
		--wait
	xcrun stapler staple build/bin/forge-ade.app

# ── Clean ───────────────────────────────────────────────────────
clean:
	rm -rf build/bin
	cd frontend && rm -rf dist

# ── Build the terminal animation stress-test CLI ────────────────
shell-test:
	cd cmd/shelltest && go build -o ../../shelltest .

# ── Tests & static checks ────────────────────────────────────────
test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
	cd frontend && bun run lint --fix
	cd frontend && bunx prettier --write "src/**/*.{ts,tsx,css}"
