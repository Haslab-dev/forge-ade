.PHONY: dev build build-prod sign notarize clean shell-test test vet fmt version patch-version minor-version major-version

# Locate go even when it is not on PATH (common install locations).
GO := $(shell command -v go 2>/dev/null || ls /usr/local/go/bin/go /opt/homebrew/opt/go/bin/go 2>/dev/null | head -1)
GOPATH := $(shell $(GO) env GOPATH 2>/dev/null)
export PATH := $(dir $(GO))$(GOPATH)/bin:$(PATH)
WAILS := $(GOPATH)/bin/wails3

# ── Current version (major.minor.patch) & build ───────────────────
# Read from frontend/package.json (single source of truth for the bump).
VERSION := $(shell node -p "require('./frontend/package.json').version")
BUILD := $(shell node -p "require('./frontend/package.json').build || 1")

# ── Version bumping ──────────────────────────────────────────────
# Bump the patch (bugfix): 0.8.6 -> 0.8.7
patch-version:
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('./frontend/package.json','utf8'));const v=p.version.split('.').map(Number);v[2]++;p.version=v.join('.');p.build=(p.build||0)+1;fs.writeFileSync('./frontend/package.json',JSON.stringify(p,null,2)+'\n');console.log('patched version ->',p.version,'(build',p.build+')')"

# Bump the minor (feature): 0.8.6 -> 0.9.0
minor-version:
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('./frontend/package.json','utf8'));const v=p.version.split('.').map(Number);v[1]++;v[2]=0;p.version=v.join('.');p.build=(p.build||0)+1;fs.writeFileSync('./frontend/package.json',JSON.stringify(p,null,2)+'\n');console.log('minored version ->',p.version,'(build',p.build+')')"

# Bump the major (breaking): 0.8.6 -> 1.0.0
major-version:
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('./frontend/package.json','utf8'));const v=p.version.split('.').map(Number);v[0]++;v[1]=0;v[2]=0;p.version=v.join('.');p.build=(p.build||0)+1;fs.writeFileSync('./frontend/package.json',JSON.stringify(p,null,2)+'\n');console.log('majored version ->',p.version,'(build',p.build+')')"

# Bump the build number alone: 31 -> 32
bump-build:
	@node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('./frontend/package.json','utf8'));p.build=(p.build||0)+1;fs.writeFileSync('./frontend/package.json',JSON.stringify(p,null,2)+'\n');console.log('build ->',p.build)"

# After bumping package.json, the version is read from it directly
# (build/config.yml `info.version` is the packaging source of truth).
version:
	@node -p "require('./frontend/package.json').version + ' (build ' + (require('./frontend/package.json').build || 1) + ')'"

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
