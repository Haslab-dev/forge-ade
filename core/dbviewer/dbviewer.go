package dbviewer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DatabaseInfo represents the high-level metadata of an inspected database.
type DatabaseInfo struct {
	Path        string      `json:"path"`
	FileName    string      `json:"fileName"`
	Type        string      `json:"type"` // "sqlite", "duckdb", "rocksdb", "unknown"
	Size        int64       `json:"size"`
	Driver      string      `json:"driver"`
	IsReadOnly  bool        `json:"isReadOnly"`
	Tables      []TableInfo `json:"tables"`
	Views       []TableInfo `json:"views"`
	Notes       string      `json:"notes,omitempty"`
	InstallHint string      `json:"installHint,omitempty"`
}

// TableInfo represents schema metadata for a table or view.
type TableInfo struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"` // "table" or "view"
	RowCount int64        `json:"rowCount"`
	Columns  []ColumnInfo `json:"columns"`
	SQL      string       `json:"sql,omitempty"`
}

// ColumnInfo describes a single column.
type ColumnInfo struct {
	CID          int    `json:"cid"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	NotNull      bool   `json:"notNull"`
	DefaultValue string `json:"defaultValue"`
	IsPK         bool   `json:"isPk"`
}

// QueryResult represents the outcome of a table fetch or custom query.
type QueryResult struct {
	Columns     []string                 `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	TotalRows   int64                    `json:"totalRows"`
	ExecutionMs int64                    `json:"executionMs"`
	Error       string                   `json:"error,omitempty"`
}

// DetectType inspects magic bytes and extension to identify the database format.
func DetectType(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		// Check for LevelDB / RocksDB directory signatures (CURRENT, MANIFEST-*, *.sst, *.ldb)
		entries, err := os.ReadDir(path)
		if err == nil {
			hasCurrent := false
			hasManifest := false
			hasSST := false
			for _, entry := range entries {
				name := entry.Name()
				if name == "CURRENT" {
					hasCurrent = true
				}
				if strings.HasPrefix(name, "MANIFEST") {
					hasManifest = true
				}
				if strings.HasSuffix(name, ".sst") || strings.HasSuffix(name, ".ldb") {
					hasSST = true
				}
			}
			if (hasCurrent && hasManifest) || hasSST {
				return "rocksdb", nil
			}
		}
		return "unknown", nil
	}

	ext := strings.ToLower(filepath.Ext(path))

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	header := make([]byte, 32)
	n, _ := f.Read(header)

	// SQLite magic header: "SQLite format 3\x00"
	if n >= 16 && string(header[:16]) == "SQLite format 3\x00" {
		return "sqlite", nil
	}

	// DuckDB magic bytes: "DUCK" at offset 0 or offset 8, or .duckdb extension
	if n >= 4 && string(header[:4]) == "DUCK" {
		return "duckdb", nil
	}
	if n >= 12 && string(header[8:12]) == "DUCK" {
		return "duckdb", nil
	}
	if ext == ".duckdb" {
		return "duckdb", nil
	}

	// RocksDB / LevelDB files (.ldb, .sst)
	if ext == ".ldb" || ext == ".sst" || ext == ".rocksdb" {
		return "rocksdb", nil
	}

	// Fallback to SQLite extensions if empty or newly initialized
	if ext == ".sqlite" || ext == ".sqlite3" || ext == ".db" || ext == ".db3" {
		return "sqlite", nil
	}

	return "unknown", nil
}

// InspectDatabase returns complete schema and metadata for the given database file or folder.
func InspectDatabase(path string) (*DatabaseInfo, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("stat database %s: %w", cleanPath, err)
	}

	dbType, err := DetectType(cleanPath)
	if err != nil {
		return nil, err
	}

	dbInfo := &DatabaseInfo{
		Path:       cleanPath,
		FileName:   filepath.Base(cleanPath),
		Type:       dbType,
		Size:       info.Size(),
		IsReadOnly: true,
	}

	switch dbType {
	case "sqlite":
		return inspectSQLite(cleanPath, dbInfo)
	case "duckdb":
		return inspectDuckDB(cleanPath, dbInfo)
	case "rocksdb":
		return inspectRocksDB(cleanPath, dbInfo)
	default:
		dbInfo.Notes = "Unrecognized database format. Supported formats: SQLite (.sqlite, .db), DuckDB (.duckdb), RocksDB/LevelDB (.ldb, .sst)."
		return dbInfo, nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SQLite Implementation
// ─────────────────────────────────────────────────────────────────────────────

func inspectSQLite(path string, dbInfo *DatabaseInfo) (*DatabaseInfo, error) {
	dbInfo.Driver = resolveSQLiteDriver()
	if dbInfo.Driver == "" {
		dbInfo.Notes = "No SQLite engine found. Please install sqlite3 CLI or python3."
		dbInfo.InstallHint = "Run 'brew install sqlite' or install python3."
		return dbInfo, nil
	}

	// 1. Fetch tables and views from sqlite_master
	masterSQL := `SELECT type, name, sql FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY type, name;`
	res, err := runSQLiteRawJSON(path, masterSQL)
	if err != nil {
		return nil, fmt.Errorf("query sqlite_master: %w", err)
	}

	tables := make([]TableInfo, 0)
	views := make([]TableInfo, 0)

	for _, row := range res {
		rowType, _ := row["type"].(string)
		name, _ := row["name"].(string)
		sqlStr, _ := row["sql"].(string)
		if name == "" {
			continue
		}

		tableInfo := TableInfo{
			Name: name,
			Type: rowType,
			SQL:  sqlStr,
		}

		// Get column schema via PRAGMA table_info
		cols, _ := getSQLiteTableColumns(path, name)
		tableInfo.Columns = cols

		// Get row count
		countSQL := fmt.Sprintf(`SELECT COUNT(*) AS count FROM "%s";`, escapeIdent(name))
		countRes, countErr := runSQLiteRawJSON(path, countSQL)
		if countErr == nil && len(countRes) > 0 {
			if countVal, ok := countRes[0]["count"]; ok {
				switch v := countVal.(type) {
				case float64:
					tableInfo.RowCount = int64(v)
				case int64:
					tableInfo.RowCount = v
				}
			}
		}

		if rowType == "table" {
			tables = append(tables, tableInfo)
		} else {
			views = append(views, tableInfo)
		}
	}

	dbInfo.Tables = tables
	dbInfo.Views = views
	return dbInfo, nil
}

func getSQLiteTableColumns(path, tableName string) ([]ColumnInfo, error) {
	pragmaSQL := fmt.Sprintf(`PRAGMA table_info("%s");`, escapeIdent(tableName))
	rows, err := runSQLiteRawJSON(path, pragmaSQL)
	if err != nil {
		return nil, err
	}

	cols := make([]ColumnInfo, 0, len(rows))
	for _, r := range rows {
		cid := 0
		if c, ok := r["cid"].(float64); ok {
			cid = int(c)
		}
		name, _ := r["name"].(string)
		colType, _ := r["type"].(string)
		notnull := false
		if nn, ok := r["notnull"].(float64); ok && nn != 0 {
			notnull = true
		}
		dflt := ""
		if d, ok := r["dflt_value"].(string); ok {
			dflt = d
		}
		isPk := false
		if pk, ok := r["pk"].(float64); ok && pk != 0 {
			isPk = true
		}

		cols = append(cols, ColumnInfo{
			CID:          cid,
			Name:         name,
			Type:         colType,
			NotNull:      notnull,
			DefaultValue: dflt,
			IsPK:         isPk,
		})
	}
	return cols, nil
}

// GetTableRows returns a paginated slice of table rows with sorting, filtering, and schema.
func GetTableRows(path, table string, limit, offset int, sortCol string, sortAsc bool, filterCol, filterVal string) (*QueryResult, error) {
	cleanPath := filepath.Clean(path)
	dbType, err := DetectType(cleanPath)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	if dbType == "duckdb" {
		return getDuckDBTableRows(cleanPath, table, limit, offset, sortCol, sortAsc, filterCol, filterVal)
	}

	// SQLite implementation:
	columns, err := getSQLiteTableColumns(cleanPath, table)
	if err != nil {
		return nil, err
	}

	colNames := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = c.Name
	}

	whereClause := ""
	if filterCol != "" && filterVal != "" {
		escapedVal := strings.ReplaceAll(filterVal, "'", "''")
		whereClause = fmt.Sprintf(`WHERE CAST("%s" AS TEXT) LIKE '%%%s%%'`, escapeIdent(filterCol), escapedVal)
	}

	// Row count
	countSQL := fmt.Sprintf(`SELECT COUNT(*) AS count FROM "%s" %s;`, escapeIdent(table), whereClause)
	countRes, err := runSQLiteRawJSON(cleanPath, countSQL)
	var totalRows int64
	if err == nil && len(countRes) > 0 {
		if c, ok := countRes[0]["count"]; ok {
			if f, ok := c.(float64); ok {
				totalRows = int64(f)
			}
		}
	}

	// Data query
	orderClause := ""
	if sortCol != "" {
		dir := "ASC"
		if !sortAsc {
			dir = "DESC"
		}
		orderClause = fmt.Sprintf(`ORDER BY "%s" %s`, escapeIdent(sortCol), dir)
	}

	selectSQL := fmt.Sprintf(`SELECT * FROM "%s" %s %s LIMIT %d OFFSET %d;`,
		escapeIdent(table), whereClause, orderClause, limit, offset)

	start := time.Now()
	rows, err := runSQLiteRawJSON(cleanPath, selectSQL)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &QueryResult{
			Columns:     colNames,
			Rows:        []map[string]interface{}{},
			TotalRows:   totalRows,
			ExecutionMs: elapsed,
			Error:       err.Error(),
		}, nil
	}

	return &QueryResult{
		Columns:     colNames,
		Rows:        rows,
		TotalRows:   totalRows,
		ExecutionMs: elapsed,
	}, nil
}

// ExecuteDatabaseQuery executes an arbitrary read-only SQL query against the database.
func ExecuteDatabaseQuery(path, sqlQuery string, limit int) (*QueryResult, error) {
	cleanPath := filepath.Clean(path)
	dbType, err := DetectType(cleanPath)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	// Guard against data modification queries
	trimmed := strings.TrimSpace(sqlQuery)
	upper := strings.ToUpper(trimmed)
	forbiddenWords := []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "CREATE ",
		"VACUUM", "ATTACH ", "DETACH ", "REPLACE ", "TRUNCATE ", "REINDEX ",
	}
	for _, word := range forbiddenWords {
		if strings.HasPrefix(upper, word) || strings.Contains(upper, "; "+word) || strings.Contains(upper, ";\n"+word) {
			return &QueryResult{
				Error: fmt.Sprintf("Write operation forbidden in read-only mode: %s", strings.TrimSpace(word)),
			}, nil
		}
	}

	if dbType == "duckdb" {
		return executeDuckDBQuery(cleanPath, sqlQuery, limit)
	}

	// SQLite query runner: Prefer Python if available for exact column ordering and safe BLOB handling
	if py := resolvePython(); py != "" {
		return runSQLiteViaPython(cleanPath, sqlQuery, limit)
	}

	// Fallback to sqlite3 CLI
	start := time.Now()
	rows, err := runSQLiteRawJSON(cleanPath, sqlQuery)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &QueryResult{
			ExecutionMs: elapsed,
			Error:       err.Error(),
		}, nil
	}

	// Extract unique columns from rows
	var columns []string
	if len(rows) > 0 {
		seen := make(map[string]bool)
		for k := range rows[0] {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}

	return &QueryResult{
		Columns:     columns,
		Rows:        rows,
		TotalRows:   int64(len(rows)),
		ExecutionMs: elapsed,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SQLite CLI & Python execution helpers
// ─────────────────────────────────────────────────────────────────────────────

func resolveSQLiteDriver() string {
	if _, err := exec.LookPath("sqlite3"); err == nil {
		return "sqlite3 (CLI)"
	}
	for _, p := range []string{"/usr/bin/sqlite3", "/usr/local/bin/sqlite3", "/opt/homebrew/bin/sqlite3"} {
		if _, err := os.Stat(p); err == nil {
			return "sqlite3 (CLI)"
		}
	}
	if py := resolvePython(); py != "" {
		return "python3 (sqlite3)"
	}
	return ""
}

func resolvePython() string {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	for _, p := range []string{"/usr/bin/python3", "/opt/homebrew/bin/python3", "/usr/local/bin/python3"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func runSQLiteRawJSON(dbPath, sql string) ([]map[string]interface{}, error) {
	sqliteBin := "sqlite3"
	for _, candidate := range []string{"/usr/bin/sqlite3", "/usr/local/bin/sqlite3", "/opt/homebrew/bin/sqlite3"} {
		if _, err := os.Stat(candidate); err == nil {
			sqliteBin = candidate
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, sqliteBin, "-readonly", "-json", dbPath, sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If sqlite3 binary failed, try python fallback
		if py := resolvePython(); py != "" {
			res, pyErr := runSQLiteViaPython(dbPath, sql, 1000)
			if pyErr == nil && res.Error == "" {
				return res.Rows, nil
			}
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, errors.New(errMsg)
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return []map[string]interface{}{}, nil
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, fmt.Errorf("parse json output: %w", err)
	}
	return rows, nil
}

func runSQLiteViaPython(dbPath, sql string, limit int) (*QueryResult, error) {
	py := resolvePython()
	if py == "" {
		return nil, errors.New("python3 is not available")
	}

	pyScript := `
import sqlite3, json, sys, time

path = sys.argv[1]
sql = sys.argv[2]
limit = int(sys.argv[3]) if len(sys.argv) > 3 else 500

t0 = time.time()
try:
    con = sqlite3.connect(f'file:{path}?mode=ro', uri=True)
    cur = con.cursor()
    cur.execute(sql)
    cols = [d[0] for d in cur.description] if cur.description else []
    raw_rows = cur.fetchmany(limit) if limit > 0 else cur.fetchall()
    elapsed = int((time.time() - t0) * 1000)
    
    rows = []
    for r in raw_rows:
        row_dict = {}
        for i, col in enumerate(cols):
            val = r[i]
            if isinstance(val, bytes):
                try:
                    val = val.decode('utf-8')
                except:
                    val = f"<BLOB {len(val)} bytes>"
            row_dict[col] = val
        rows.append(row_dict)
    
    print(json.dumps({'columns': cols, 'rows': rows, 'totalRows': len(rows), 'executionMs': elapsed}))
except Exception as e:
    print(json.dumps({'error': str(e), 'columns': [], 'rows': [], 'totalRows': 0, 'executionMs': 0}))
`

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, py, "-c", pyScript, dbPath, sql, fmt.Sprintf("%d", limit))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, errors.New(errMsg)
	}

	var result QueryResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse python output: %w", err)
	}
	return &result, nil
}

func escapeIdent(name string) string {
	return strings.ReplaceAll(name, `"`, `""`)
}

// ─────────────────────────────────────────────────────────────────────────────
// DuckDB Support
// ─────────────────────────────────────────────────────────────────────────────

func inspectDuckDB(path string, dbInfo *DatabaseInfo) (*DatabaseInfo, error) {
	duckBin, err := exec.LookPath("duckdb")
	hasDuckCLI := err == nil
	hasDuckPy := checkDuckDBPython()

	if !hasDuckCLI && !hasDuckPy {
		dbInfo.Driver = "none"
		dbInfo.Notes = "DuckDB database file detected."
		dbInfo.InstallHint = "DuckDB CLI or python package is required to browse tables. Install via:\n• macOS: brew install duckdb\n• Python: pip install duckdb"
		return dbInfo, nil
	}

	dbInfo.Driver = "duckdb"
	if hasDuckCLI {
		dbInfo.Driver = fmt.Sprintf("duckdb (%s)", duckBin)
	} else {
		dbInfo.Driver = "python3 (duckdb)"
	}

	// Query information_schema.tables
	sql := `SELECT table_name as name, table_type as type FROM information_schema.tables WHERE table_schema = 'main' ORDER BY table_name;`
	res, err := executeDuckDBQuery(path, sql, 200)
	if err != nil || res.Error != "" {
		if res != nil && res.Error != "" {
			dbInfo.Notes = "DuckDB query error: " + res.Error
		}
		return dbInfo, nil
	}

	var tables []TableInfo
	var views []TableInfo
	for _, row := range res.Rows {
		name, _ := row["name"].(string)
		tType, _ := row["type"].(string)
		tInfo := TableInfo{
			Name: name,
			Type: strings.ToLower(tType),
		}
		// Count rows
		cntRes, cntErr := executeDuckDBQuery(path, fmt.Sprintf(`SELECT COUNT(*) as count FROM "%s";`, escapeIdent(name)), 1)
		if cntErr == nil && len(cntRes.Rows) > 0 {
			if c, ok := cntRes.Rows[0]["count"].(float64); ok {
				tInfo.RowCount = int64(c)
			}
		}

		if strings.Contains(strings.ToLower(tType), "view") {
			views = append(views, tInfo)
		} else {
			tables = append(tables, tInfo)
		}
	}

	dbInfo.Tables = tables
	dbInfo.Views = views
	return dbInfo, nil
}

func checkDuckDBPython() bool {
	py := resolvePython()
	if py == "" {
		return false
	}
	cmd := exec.Command(py, "-c", "import duckdb")
	return cmd.Run() == nil
}

func executeDuckDBQuery(path, sql string, limit int) (*QueryResult, error) {
	duckBin, err := exec.LookPath("duckdb")
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, duckBin, "-readonly", "-json", path, sql)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		start := time.Now()
		runErr := cmd.Run()
		elapsed := time.Since(start).Milliseconds()
		if runErr != nil {
			return &QueryResult{
				ExecutionMs: elapsed,
				Error:       strings.TrimSpace(stderr.String()),
			}, nil
		}
		var rows []map[string]interface{}
		_ = json.Unmarshal(stdout.Bytes(), &rows)
		var cols []string
		if len(rows) > 0 {
			for k := range rows[0] {
				cols = append(cols, k)
			}
		}
		return &QueryResult{
			Columns:     cols,
			Rows:        rows,
			TotalRows:   int64(len(rows)),
			ExecutionMs: elapsed,
		}, nil
	}

	if checkDuckDBPython() {
		py := resolvePython()
		script := `
import duckdb, json, sys, time

path = sys.argv[1]
sql = sys.argv[2]
t0 = time.time()
try:
    con = duckdb.connect(path, read_only=True)
    df = con.execute(sql).df()
    elapsed = int((time.time() - t0) * 1000)
    cols = list(df.columns)
    rows = df.to_dict(orient='records')
    print(json.dumps({'columns': cols, 'rows': rows, 'totalRows': len(rows), 'executionMs': elapsed}))
except Exception as e:
    print(json.dumps({'error': str(e), 'columns': [], 'rows': [], 'totalRows': 0, 'executionMs': 0}))
`
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, py, "-c", script, path, sql)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return &QueryResult{Error: strings.TrimSpace(stderr.String())}, nil
		}
		var res QueryResult
		_ = json.Unmarshal(stdout.Bytes(), &res)
		return &res, nil
	}

	return &QueryResult{
		Error: "DuckDB is not installed on this system. Install DuckDB CLI or python package to query DuckDB files.",
	}, nil
}

func getDuckDBTableRows(path, table string, limit, offset int, sortCol string, sortAsc bool, filterCol, filterVal string) (*QueryResult, error) {
	whereClause := ""
	if filterCol != "" && filterVal != "" {
		escapedVal := strings.ReplaceAll(filterVal, "'", "''")
		whereClause = fmt.Sprintf(`WHERE CAST("%s" AS VARCHAR) ILIKE '%%%s%%'`, escapeIdent(filterCol), escapedVal)
	}
	orderClause := ""
	if sortCol != "" {
		dir := "ASC"
		if !sortAsc {
			dir = "DESC"
		}
		orderClause = fmt.Sprintf(`ORDER BY "%s" %s`, escapeIdent(sortCol), dir)
	}
	sql := fmt.Sprintf(`SELECT * FROM "%s" %s %s LIMIT %d OFFSET %d;`, escapeIdent(table), whereClause, orderClause, limit, offset)
	return executeDuckDBQuery(path, sql, limit)
}

// ─────────────────────────────────────────────────────────────────────────────
// RocksDB / LevelDB Support
// ─────────────────────────────────────────────────────────────────────────────

func inspectRocksDB(path string, dbInfo *DatabaseInfo) (*DatabaseInfo, error) {
	dbInfo.Driver = "LSM-Tree Key-Value Store"
	dbInfo.Notes = "RocksDB / LevelDB LSM-Tree Key-Value Store detected. This database engine organizes structured key-value pairs sorted in SSTables on disk rather than tabular SQL rows."
	dbInfo.InstallHint = "To inspect raw keys and values:\n• RocksDB tool: ldb --db=" + path + " scan\n• Python library: pip install plyvel / rocksdict"

	// If it's a directory, tally SSTables and search for CURRENT and MANIFEST
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		entries, _ := os.ReadDir(path)
		var totalBytes int64
		var sstCount int
		var ldbCount int
		var manifestName string

		for _, e := range entries {
			fi, err := e.Info()
			if err == nil {
				totalBytes += fi.Size()
			}
			if strings.HasSuffix(e.Name(), ".sst") {
				sstCount++
			} else if strings.HasSuffix(e.Name(), ".ldb") {
				ldbCount++
			} else if strings.HasPrefix(e.Name(), "MANIFEST") {
				manifestName = e.Name()
			}
		}

		dbInfo.Size = totalBytes
		dbInfo.Tables = []TableInfo{
			{
				Name:     "SSTables & Data Files",
				Type:     "lsm-tree",
				RowCount: int64(sstCount + ldbCount),
				Columns: []ColumnInfo{
					{Name: "SSTable Files (.sst)", Type: fmt.Sprintf("%d files", sstCount)},
					{Name: "Log Data Files (.ldb)", Type: fmt.Sprintf("%d files", ldbCount)},
					{Name: "Active Manifest", Type: manifestName},
					{Name: "Total Directory Size", Type: fmt.Sprintf("%.2f MB", float64(totalBytes)/(1024*1024))},
				},
			},
		}
	}

	return dbInfo, nil
}
