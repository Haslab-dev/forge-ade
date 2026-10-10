package app

import (
	"github.com/hasdev/forge-ade/internal/editor"
)

// Editor language tooling bindings. The implementation lives in
// internal/editor; these thin bindings resolve the workspace root per call
// (an app launched from Finder has an arbitrary working directory, so the
// root must come from the open workspace, not os.Getwd).

// editorTooling builds a Tooling rooted at the first workspace folder.
func (a *App) editorTooling() *editor.Tooling {
	root := ""
	if ws := a.workspaceMgr.Current(); ws != nil {
		if folders := ws.GetFolders(); len(folders) > 0 && !isRemotePath(folders[0]) {
			root = folders[0]
		}
	}
	return editor.NewTooling(root)
}

// isRemotePath reports whether the path addresses a remote (SSH) location.
func isRemotePath(p string) bool {
	return len(p) >= 6 && p[:6] == "ssh://"
}

// CheckSyntax runs esbuild's parser over a JS/TS source string and returns
// syntax diagnostics (missing braces, invalid tokens, etc.). It does not do
// type checking — that requires a language server.
func (a *App) CheckSyntax(path, content string) ([]editor.SyntaxDiagnostic, error) {
	return a.editorTooling().CheckSyntax(a.ctx, path, content)
}

// FormatCode formats a JS/TS source string using the workspace's prettier
// (with its .prettierrc config when present). Returns the formatted source,
// or the original content if prettier is unavailable.
func (a *App) FormatCode(path, content string) (string, error) {
	return a.editorTooling().FormatCode(a.ctx, path, content)
}
