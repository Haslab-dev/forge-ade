package database

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/workspace"
	"github.com/hasdev/forge-ade/internal/dbviewer"
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

// NewDatabaseManager creates and initializes the DatabaseManager.
func NewDatabaseManager(ws *workspace.WorkspaceState) *DatabaseManager {
	dm := &DatabaseManager{
		Workspace: ws,
		SQLQuery:  "SELECT * FROM sqlite_master LIMIT 25;",
	}
	dm.ScanDatabases()
	return dm
}

// ScanDatabases finds database files inside the active workspace.
func (dm *DatabaseManager) ScanDatabases() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

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

	// Auto-open first discovered database
	if len(dbs) > 0 && dm.ActiveDBPath == "" {
		dm.ActiveDBPath = dbs[0]
		go dm.Inspect(dbs[0])
	}
}

// Inspect loads database metadata, tables, and column schemas.
func (dm *DatabaseManager) Inspect(path string) {
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
		dm.SQLQuery = fmt.Sprintf("SELECT * FROM %s LIMIT 25;", firstTable)
		go dm.ExecuteQuery()
	} else if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}
}

// SelectTable loads table rows and updates the SQL console.
func (dm *DatabaseManager) SelectTable(tableName string) {
	dm.mu.Lock()
	dm.ActiveTable = tableName
	dm.SQLQuery = fmt.Sprintf("SELECT * FROM %s LIMIT 25;", tableName)
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
	} else {
		dm.QueryResult = res
		dm.StatusMsg = fmt.Sprintf("%d rows returned in %d ms", len(res.Rows), res.ExecutionMs)
	}

	if dm.OnNeedRedraw != nil {
		dm.OnNeedRedraw()
	}
}
