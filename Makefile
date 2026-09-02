.PHONY: dev build build-prod sign notarize clean shell-test version patch-version minor-version major-version

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
	wails3 dev

# ── Production build (with devtools for debugging) ──────────────
build:
	wails3 build -devtools

# ── Production build (without devtools, smaller binary) ──────────
build-prod:
	wails3 build

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
	cd shell_test && go build -o ../shell-test .
