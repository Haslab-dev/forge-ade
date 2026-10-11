package app

import (
	"github.com/hasdev/forge-ade/internal/dbviewer"
)

// ---------------------------------------------------------------------------
// Database Viewer Operations (SQLite, DuckDB, RocksDB)
// ---------------------------------------------------------------------------

// InspectDatabase returns schema, tables, views, columns, and engine metadata.
func (a *App) InspectDatabase(path string) (*dbviewer.DatabaseInfo, error) {
	if resolved, err := ResolvePath(path); err == nil {
		path = resolved
	}
	return dbviewer.InspectDatabase(path)
}

// GetDatabaseTableRows returns a paginated slice of table rows with sorting, filtering, and schema.
func (a *App) GetDatabaseTableRows(path, table string, limit, offset int, sortCol string, sortAsc bool, filterCol, filterVal string) (*dbviewer.QueryResult, error) {
	if resolved, err := ResolvePath(path); err == nil {
		path = resolved
	}
	return dbviewer.GetTableRows(path, table, limit, offset, sortCol, sortAsc, filterCol, filterVal)
}

// ExecuteDatabaseQuery executes an arbitrary read-only SQL query against the database.
func (a *App) ExecuteDatabaseQuery(path, sqlQuery string, limit int) (*dbviewer.QueryResult, error) {
	if resolved, err := ResolvePath(path); err == nil {
		path = resolved
	}
	return dbviewer.ExecuteDatabaseQuery(path, sqlQuery, limit)
}
