.PHONY: dev build build-prod sign notarize clean shell-test

# ── Development ──────────────────────────────────────────────────
dev:
	wails dev

# ── Production build (with devtools for debugging) ──────────────
build:
	cd frontend && bun run build
	wails generate module
	wails build -skipbindings -s -devtools

# ── Production build (without devtools, smaller binary) ──────────
build-prod:
	cd frontend && bun run build
	wails generate module
	wails build -skipbindings -s

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
