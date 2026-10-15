package ignore

import "testing"

func TestName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Canonical entries.
		{"node_modules", true},
		{".git", true},
		{"dist", true},
		{"target", true},
		{"__pycache__", true},
		// Matching is case-insensitive (map stores lowercase keys).
		{"NODE_MODULES", true},
		{"Dist", true},
		{"TARGET", true},
		{"__PYCACHE__", true},
		{"Pods", true},
		{"pods", true},
		{"PODS", true},
		// Non-ignored names.
		{"src", false},
		{"my-lib", false},
		{"main.go", false},
		{"", false},
		// A name merely containing an ignored name is not ignored.
		{"mynode_modules", false},
		// Ignored name with an extension is a different component.
		{"build.go", false},
	}
	for _, tc := range tests {
		if got := Name(tc.name); got != tc.want {
			t.Errorf("Name(%q) = %v; want %v", tc.name, got, tc.want)
		}
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"", false},
		{"src/main.go", false},
		{"my-lib/pkg/util.go", false},
		// Each component is checked.
		{"foo/node_modules/bar", true},
		{"foo/dist/bar", true},
		{"vendor", true},
		{"a/b/target/c", true},
		// Windows separators are normalized.
		{`foo\node_modules\bar`, true},
		{`proj\dist\out.js`, true},
		// Leading/trailing slashes are tolerated.
		{"/node_modules/", true},
		{"node_modules/", true},
		{"/tmp/build/", true},
		// Component must match exactly, not contain.
		{"mynode_modules", false},
		{"foo/mynode_modules/bar", false},
		{"build.go", false},
		{"foo/build.go", false},
		// Case-insensitive per component.
		{"foo/NODE_MODULES/bar", true},
	}
	for _, tc := range tests {
		if got := Path(tc.path); got != tc.want {
			t.Errorf("Path(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

// TestDirCanonicalEntriesPresent guards against accidental clobbering of the
// shared Dir map (e.g. a bad merge dropping entries that watcher/search/index
// all rely on).
func TestDirCanonicalEntriesPresent(t *testing.T) {
	// "Pods" is stored lowercased in the map; check via Name to cover the
	// case-insensitivity contract too.
	entries := []string{
		".git", ".svn", "node_modules", "dist", "build", "target", "vendor",
		"__pycache__", ".venv", "Pods", ".gradle", "obj", "bin",
	}
	for _, e := range entries {
		if !Name(e) {
			t.Errorf("Dir is missing documented entry %q", e)
		}
	}
}
