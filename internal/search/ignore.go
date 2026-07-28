package search

import (
	"path/filepath"
	"strings"
)

// IsIndexed checks whether a file path should be indexed.
func IsIndexed(path string) bool {
	// Skip directories/files with these names
	base := filepath.Base(path)
	name := strings.ToLower(base)

	skipNames := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true,
		"vendor": true, ".next": true, ".cache": true,
		"dist": true, "build": true, "coverage": true,
		"__pycache__": true, ".DS_Store": true,
	}

	if skipNames[name] {
		return false
	}

	// Skip common binary/archive extensions
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".bin": true, ".dll": true, ".so": true,
		".dylib": true, ".png": true, ".jpg": true, ".jpeg": true,
		".gif": true, ".ico": true, ".svg": true, ".woff": true,
		".woff2": true, ".ttf": true, ".eot": true, ".mp4": true,
		".mp3": true, ".zip": true, ".tar": true, ".gz": true,
		".o": true, ".a": true, ".class": true, ".pyc": true,
		".jar": true, ".war": true, ".dmg": true, ".pkg": true,
		".deb": true, ".rpm": true, ".wasm": true,
	}

	return !binaryExts[ext]
}

// IsDirSkipped checks if a directory should be skipped entirely.
func IsDirSkipped(dirName string) bool {
	name := strings.ToLower(dirName)
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, ".svn": true,
		"vendor": true, ".next": true, ".cache": true,
		"dist": true, "build": true, "coverage": true,
		"__pycache__": true, ".hg": true, ".bzr": true,
	}
	return skipDirs[name]
}
