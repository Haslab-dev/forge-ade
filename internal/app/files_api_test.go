package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanPathForFS(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean absolute path",
			input:    "/Users/dev/project/file.go",
			expected: "/Users/dev/project/file.go",
		},
		{
			name:     "file uri scheme",
			input:    "file:///Users/dev/project/file.go",
			expected: "/Users/dev/project/file.go",
		},
		{
			name:     "wrapped in quotes and backticks",
			input:    "`/Users/dev/project/file.go`",
			expected: "/Users/dev/project/file.go",
		},
		{
			name:     "json property path wrapper",
			input:    `Path":"/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift"`,
			expected: "/Users/hy4-mac-002/hasdev/works/inagov-ios/inagov-ios/Modules/Common/View/SwiftUIAppHeader.swift",
		},
		{
			name:     "embedded json object path wrapper",
			input:    `{"path": "/home/user/project/main.go"}`,
			expected: "/home/user/project/main.go",
		},
		{
			name:     "tilde expansion",
			input:    "~/project/test.go",
			expected: filepath.Join(home, "project/test.go"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanPathForFS(tc.input)
			if got != tc.expected {
				t.Errorf("CleanPathForFS(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
