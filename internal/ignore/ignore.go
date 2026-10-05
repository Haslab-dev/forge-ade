// Package ignore provides a shared list of dependency, build-output and
// tooling directories that are always skipped when scanning, searching or
// watching a workspace. Keeping one source of truth here (instead of four
// divergent maps in watcher/search/index) ensures new entries apply
// everywhere.
package ignore

import "strings"

// Dir is a canonical, case-insensitive set of directory names that should
// never be indexed, searched or watched.
var Dir = map[string]bool{
	// VCS & Meta
	".git": true, ".svn": true, ".hg": true, ".bzr": true, ".jj": true,

	// Temporary & System
	"tmp": true, "temp": true, ".tmp": true, ".temp": true,
	".ds_store": true, "thumbs.db": true, ".fseventsd": true, ".spotlight-v100": true,

	// JavaScript / TypeScript / Node / Web
	"node_modules": true, "bower_components": true, "jspm_packages": true,
	".next": true, ".nuxt": true, ".cache": true, ".turbo": true, ".parcel-cache": true,
	"dist": true, "build": true, ".output": true, "out": true, ".out": true,
	".yarn": true, ".pnp.cjs": true, ".pnp.loader.mjs": true, ".npm": true, ".pnpm-store": true,
	".svelte-kit": true, ".astro": true, ".docusaurus": true, ".vitepress-cache": true,
	".gatsby": true, ".remix": true, ".webpack": true, ".vite": true,
	"storybook-static": true, ".storybook": true,

	// iOS / macOS / Swift / Objective-C / React Native
	"pods": true, ".xcworkspace": true, ".xcodeproj": true, "xcbuilddata": true,
	"deriveddata": true, ".build": true, ".swiftpm": true, "carthage": true,
	"frameworks": true, "index.noindex": true, "modulecache.noindex": true,
	"symbols": true, "buildartifacts": true,

	// Android / Gradle / Java / Kotlin
	".gradle": true, "gradle": true, ".idea": true, ".vscode": true, "captures": true,
	".externalnativebuild": true, ".cxx": true, ".navigation": true,
	".kotlin": true, ".konan": true, "kotlin-caches": true, ".klib": true,
	"android-build": true,

	// Flutter / Dart
	".dart_tool": true, ".pub-cache": true, ".pub": true, "ephemeral": true,
	".flutter-plugins": true, ".flutter-plugins-dependencies": true,

	// Python
	"__pycache__": true, ".pytest_cache": true, ".venv": true, "venv": true, "env": true,
	".mypy_cache": true, ".ruff_cache": true, ".tox": true, "htmlcov": true,
	".eggs": true, ".nox": true, ".pytype": true, "pip-wheel-metadata": true,

	// Rust / Cargo
	"target": true, ".cargo": true, ".rustup": true,

	// Go
	"bin": true, "vendor": true,

	// C / C++ / CMake
	"cmake-build-debug": true, "cmake-build-release": true, ".cmake": true,
	"cmakefiles": true,

	// Elixir / Erlang
	"_build": true, "deps": true,

	// Ruby
	".bundle": true,

	// Haskell
	".stack-work": true, ".cabal-sandbox": true, "dist-newstyle": true,

	// .NET / C# / F#
	"obj": true, ".vs": true, ".nuget": true,

	// PHP / Composer
	".phpunit.cache": true,

	// General Build & Output
	"coverage": true, "artifacts": true, "output": true,
}

// Name reports whether a single directory name (no separators) is ignored.
func Name(name string) bool {
	return Dir[strings.ToLower(name)]
}

// Path reports whether any path component of p is ignored. p may use "/" or
// the platform separator.
func Path(p string) bool {
	if p == "" {
		return false
	}
	clean := strings.ReplaceAll(p, "\\", "/")
	for _, part := range strings.Split(clean, "/") {
		if part != "" && Name(part) {
			return true
		}
	}
	return false
}
