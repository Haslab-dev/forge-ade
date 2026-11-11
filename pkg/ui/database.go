package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/dbviewer"
)

// DatabaseManager coordinates database schema inspection, data browsing, and query execution.
type DatabaseManager struct {
	mu           sync.RWMutex
	engine       *Engine
	Workspace    *WorkspaceState
	ActiveDBPath string
	Databases    []string
	Info         *dbviewer.DatabaseInfo
	ActiveTable  string
	SQLQuery     string
	QueryResult  *dbviewer.QueryResult
	StatusMsg    string
	IsExecuting  bool
}

// NewDatabaseManager creates and initializes the DatabaseManager.
func NewDatabaseManager(engine *Engine, ws *WorkspaceState) *DatabaseManager {
	dm := &DatabaseManager{
		engine:    engine,
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

	info, err := dbviewer.InspectDatabase(path)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if err != nil {
		dm.StatusMsg = fmt.Sprintf("Inspection error: %v", err)
		dm.Info = nil
		return
	}

	dm.Info = info
	dm.StatusMsg = fmt.Sprintf("Loaded %s (%s, %d tables)", info.FileName, info.Type, len(info.Tables))

	if len(info.Tables) > 0 {
		firstTable := info.Tables[0].Name
		dm.ActiveTable = firstTable
		dm.SQLQuery = fmt.Sprintf("SELECT * FROM %s LIMIT 25;", firstTable)
		go dm.ExecuteQuery()
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

	if path == "" || query == "" {
		dm.mu.Lock()
		dm.IsExecuting = false
		dm.mu.Unlock()
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
		return
	}

	dm.QueryResult = res
	dm.StatusMsg = fmt.Sprintf("%d rows returned in %d ms", len(res.Rows), res.ExecutionMs)
}

// RenderDatabase draws the interactive database browser and SQL query runner.
func (dm *DatabaseManager) RenderDatabase(c *ui.Context) {
	t := c.Theme()

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	ui.Column(c).Fill().Children(func() {
		// Top Toolbar with Tabler Icons
		ui.Row(c).Gap(12).Padding(8, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerDatabase, 15)
				ui.Text(c, "Database Viewer").Bold().FontSize(14)
			})
			if dm.Info != nil {
				ui.Text(c, fmt.Sprintf("%s (%s)", dm.Info.FileName, dm.Info.Type)).FontSize(12).TextColor(t.Accent)
			}
			ui.Box(c).Grow(1)
			rescanBtn := ui.Button(c, "")
			rescanBtn.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerRefresh, 13)
					ui.Text(c, "Rescan").FontSize(11)
				})
			})
			if rescanBtn.Clicked() {
				go dm.ScanDatabases()
			}
		})

		if len(dm.Databases) == 0 {
			ui.Column(c).Grow(1).Center().Children(func() {
				TablerIcon(c, TablerDatabase, 28)
				ui.Text(c, "No SQLite / DuckDB databases detected").FontSize(15).Bold().TextColor(t.TextMuted).Padding(6, 0, 0, 0)
				ui.Text(c, "Add a .sqlite, .sqlite3, .db, or .duckdb file to your workspace.").TextColor(t.TextMuted)
			})
			return
		}

		// Body: Tables Sidebar + Query Runner
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			// Left: Table list & Schema
			ui.Column(c).Width(220).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Row(c).Padding(10, 12).Children(func() {
					ui.Text(c, "TABLES & VIEWS").FontSize(11).Bold().TextColor(t.TextMuted)
				})

				if dm.Info != nil {
					ui.Scroll(c).Grow(1).Padding(4, 8).Children(func() {
						for _, tb := range dm.Info.Tables {
							tName := tb.Name
							row := ui.Row(c.Key("db-table-"+tName)).Padding(6, 8).Radius(6).Gap(6).AlignItems(ui.Center)
							if tName == dm.ActiveTable {
								row.Background(t.Accent).TextColor(t.AccentText)
							} else if row.Hovered() {
								row.Background(t.Border)
							}

							TablerIcon(c, TablerTable, 13)
							ui.Text(c, tName).FontSize(12).Grow(1)
							ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(11).TextColor(t.TextMuted)

							if row.Clicked() {
								go dm.SelectTable(tName)
							}
						}
					})
				}
			})

			// Right: SQL Console & Result Grid
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				// SQL Query Bar
				ui.Column(c).Padding(10, 16).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
					in := ui.TextInput(c, &dm.SQLQuery).Placeholder("Write read-only SQL query (e.g. SELECT * FROM users LIMIT 25)...").Grow(1)
					if in.Submitted() && !dm.IsExecuting {
						go dm.ExecuteQuery()
					}

					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						runBtn := ui.PrimaryButton(c, "")
						runBtn.Children(func() {
							ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
								TablerIcon(c, TablerPlayerPlay, 13)
								ui.Text(c, "Execute SQL (⌘Enter)").FontSize(11).Bold()
							})
						})
						if dm.IsExecuting {
							runBtn.Disabled(true)
						}
						if runBtn.Clicked() {
							go dm.ExecuteQuery()
						}
						ui.Box(c).Grow(1)
						if dm.StatusMsg != "" {
							ui.Text(c, dm.StatusMsg).FontSize(12).TextColor(t.TextMuted)
						}
					})
				})

				// Query Result Grid
				if dm.QueryResult != nil && len(dm.QueryResult.Columns) > 0 {
					ui.Scroll(c).Grow(1).Padding(8, 16).Children(func() {
						// Header Row
						ui.Row(c).Padding(6, 8).Background(t.Surface).Radius(6).Children(func() {
							for _, col := range dm.QueryResult.Columns {
								ui.Text(c, col).Bold().FontSize(12).Width(120)
							}
						})

						// Data Rows
						for i, r := range dm.QueryResult.Rows {
							ui.Row(c.Key(fmt.Sprintf("row-%d", i))).Padding(4, 8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
								for _, col := range dm.QueryResult.Columns {
									valStr := fmt.Sprintf("%v", r[col])
									if len(valStr) > 40 {
										valStr = valStr[:37] + "…"
									}
									ui.Text(c, valStr).FontSize(12).Width(120)
								}
							})
						}
					})
				} else {
					ui.Column(c).Grow(1).Center().Children(func() {
						ui.Text(c, "No query results").TextColor(t.TextMuted).FontSize(13)
					})
				}
			})
		})
	})
}
