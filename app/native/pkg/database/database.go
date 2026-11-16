package database

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
	"github.com/hasdev/forge-ade/core/dbviewer"
)

// DatabaseManager coordinates database schema inspection, data browsing, and query execution.
type DatabaseManager struct {
	mu           sync.RWMutex
	Workspace    *workspace.WorkspaceState
	ActiveDBPath string
	Databases    []string
	Info         *dbviewer.DatabaseInfo
	ActiveTable  string
	SQLQuery     string
	QueryResult  *dbviewer.QueryResult
	StatusMsg    string
	IsExecuting  bool
	OnNeedRedraw func()
}

// DatabaseSnapshot is a consistent point-in-time copy of DatabaseManager
// state for render paths. Background goroutines (ScanDatabases, Inspect,
// SelectTable, ExecuteQuery) write the exported fields under mu, so reading
// fields directly from the render thread races — ScanDatabases can nil out
// Info or QueryResult between a nil-check and its use, which crashes the
// frame. Always render from a Snapshot.
type DatabaseSnapshot struct {
	ActiveDBPath string
	Databases    []string
	Info         *dbviewer.DatabaseInfo
	ActiveTable  string
	SQLQuery     string
	StatusMsg    string
	IsExecuting  bool
	QueryResult  *dbviewer.QueryResult
}

// Snapshot returns the render-relevant state under RLock.
//
// Info is returned as the live *dbviewer.DatabaseInfo pointer: writers only
// ever replace it wholesale (InspectDatabase builds a fresh value; nothing
// mutates the pointee in place), so a pointer read under lock stays stable.
// Databases is copied. QueryResult is returned as a copy with its Columns
// slice and Rows (including each row map) shallow-copied: the render iterates
// every column of every row, so the slice and map headers it reads must not
// be able to change under it. The cell values themselves are shared — query
// results are built fresh by dbviewer and never mutated after publication.
func (dm *DatabaseManager) Snapshot() DatabaseSnapshot {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	snap := DatabaseSnapshot{
		ActiveDBPath: dm.ActiveDBPath,
		Info:         dm.Info,
		ActiveTable:  dm.ActiveTable,
		SQLQuery:     dm.SQLQuery,
		StatusMsg:    dm.StatusMsg,
		IsExecuting:  dm.IsExecuting,
	}
	if len(dm.Databases) > 0 {
		snap.Databases = make([]string, len(dm.Databases))
		copy(snap.Databases, dm.Databases)
	}
	if qr := dm.QueryResult; qr != nil {
		c := &dbviewer.QueryResult{
			Columns:     append([]string(nil), qr.Columns...),
			TotalRows:   qr.TotalRows,
			ExecutionMs: qr.ExecutionMs,
			Error:       qr.Error,
		}
		if len(qr.Rows) > 0 {
			c.Rows = make([]map[string]interface{}, len(qr.Rows))
			for i, row := range qr.Rows {
				if row == nil {
					continue
				}
				rowCopy := make(map[string]interface{}, len(row))
				for k, v := range row {
					rowCopy[k] = v
				}
				c.Rows[i] = rowCopy
			}
		}
		snap.QueryResult = c
	}
	return snap
}

// SetSQLQuery stores the query text under lock. The SQL console binds to a
// frame-local copy and syncs edits back through this setter, keeping its
// UI-thread writes synchronized with ExecuteQuery's locked read.
func (dm *DatabaseManager) SetSQLQuery(q string) {
	dm.mu.Lock()
	dm.SQLQuery = q
	dm.mu.Unlock()
}

// NewDatabaseManager creates and initializes the DatabaseManager.
func NewDatabaseManager(ws *workspace.WorkspaceState) *DatabaseManager {
	dm := &DatabaseManager{
		Workspace: ws,
		SQLQuery:  "SELECT * FROM sqlite_master LIMIT 25;",
	}
	dm.ScanDatabases()
	return dm
}

// SetWorkspaceAndScan atomically updates the workspace reference and rescans its files.
func (dm *DatabaseManager) SetWorkspaceAndScan(ws *workspace.WorkspaceState) {
	dm.mu.Lock()
	dm.Workspace = ws
	dm.mu.Unlock()
	dm.ScanDatabases()
}

// ScanDatabases finds database files inside the active workspace.
func (dm *DatabaseManager) ScanDatabases() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.Workspace == nil || dm.Workspace.IsRemote() {
		dm.Databases = nil
		dm.ActiveDBPath = ""
		dm.Info = nil
		dm.ActiveTable = ""
		dm.QueryResult = nil
		return
	}

	var dbs []string
	if dm.Workspace != nil {
		for _, f := range dm.Workspace.Files {
			lower := strings.ToLower(f.RelativePath)
			if strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") ||
				strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".db3") ||
				strings.HasSuffix(lower, ".duckdb") {
				dbs = append(dbs, f.FullPath)
			}
		}
	}
	dm.Databases = dbs

	// Reset active DB state if switching workspaces or active DB disappeared.
	stillPresent := false
	for _, p := range dbs {
		if p == dm.ActiveDBPath {
			stillPresent = true
			break
		}
	}
	if !stillPresent {
		dm.ActiveDBPath = ""
		dm.Info = nil
		dm.ActiveTable = ""
		dm.QueryResult = nil
	}

	// Auto-open first discovered database
	if len(dbs) > 0 && dm.ActiveDBPath == "" {
		dm.ActiveDBPath = dbs[0]
		go dm.Inspect(dbs[0])
	}
}

func quoteSQLIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Inspect loads database metadata, tables, and column schemas.
func (dm *DatabaseManager) Inspect(path string) {
	if strings.HasPrefix(path, "ssh://") {
		return
	}
	dm.mu.Lock()
	dm.ActiveDBPath = path
	dm.StatusMsg = "Inspecting schema..."
	dm.mu.Unlock()

	if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}

	info, err := dbviewer.InspectDatabase(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if err != nil {
		dm.StatusMsg = fmt.Sprintf("Inspection error: %v", err)
		dm.Info = nil
		if dm.OnNeedRedraw != nil {
			dm.OnNeedRedraw()
		}
		return
	}

	dm.Info = info
	dm.StatusMsg = fmt.Sprintf("Loaded %s (%s, %d tables)", info.FileName, info.Type, len(info.Tables))

	if len(info.Tables) > 0 {
		firstTable := info.Tables[0].Name
		dm.ActiveTable = firstTable
		dm.SQLQuery = fmt.Sprintf("SELECT * FROM %s LIMIT 25;", quoteSQLIdent(firstTable))
		go dm.ExecuteQuery()
	} else if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}
}

// SelectTable loads table rows and updates the SQL console.
func (dm *DatabaseManager) SelectTable(tableName string) {
	dm.mu.Lock()
	dm.ActiveTable = tableName
	dm.SQLQuery = fmt.Sprintf("SELECT * FROM %s LIMIT 25;", quoteSQLIdent(tableName))
	dm.mu.Unlock()

	go dm.ExecuteQuery()
}

// ExecuteQuery runs the current read-only SQL query against the inspected database.
func (dm *DatabaseManager) ExecuteQuery() {
	dm.mu.Lock()
	path := dm.ActiveDBPath
	query := strings.TrimSpace(dm.SQLQuery)
	dm.IsExecuting = true
	dm.StatusMsg = "Executing query..."
	dm.mu.Unlock()

	if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}

	if path == "" || query == "" {
		dm.mu.Lock()
		dm.IsExecuting = false
		dm.mu.Unlock()
		if dm.OnNeedRedraw != nil {
			dm.OnNeedRedraw()
		}
		return
	}

	start := time.Now()
	res, err := dbviewer.ExecuteDatabaseQuery(path, query, 50)
	elapsed := time.Since(start).Milliseconds()

	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.IsExecuting = false

	if err != nil {
		dm.StatusMsg = fmt.Sprintf("Query error: %v (%d ms)", err, elapsed)
		dm.QueryResult = nil
	} else if res != nil && res.Error != "" {
		dm.StatusMsg = fmt.Sprintf("Query error: %s", res.Error)
		dm.QueryResult = res
	} else {
		dm.QueryResult = res
		dm.StatusMsg = fmt.Sprintf("%d rows returned in %d ms", len(res.Rows), res.ExecutionMs)
	}

	if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}
}
