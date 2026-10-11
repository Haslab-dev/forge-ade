package dbviewer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTestSQLiteDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	sqliteBin := "sqlite3"
	if p, err := exec.LookPath("sqlite3"); err == nil {
		sqliteBin = p
	} else if _, err := os.Stat("/usr/bin/sqlite3"); err == nil {
		sqliteBin = "/usr/bin/sqlite3"
	}

	cmd := exec.Command(sqliteBin, dbPath)
	cmd.Stdin = strings.NewReader(`
CREATE TABLE users (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT,
	role TEXT DEFAULT 'developer',
	active INTEGER
);
INSERT INTO users (name, email, role, active) VALUES ('Alice Johnson', 'alice@test.local', 'admin', 1);
INSERT INTO users (name, email, role, active) VALUES ('Bob Smith', 'bob@test.local', 'developer', 1);
INSERT INTO users (name, email, role, active) VALUES ('Charlie Doe', 'charlie@test.local', 'viewer', 0);
CREATE VIEW active_users AS SELECT id, name, email FROM users WHERE active = 1;
`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("create test db failed: %v, out: %s", err, string(out))
	}
	return dbPath
}

func TestDetectType(t *testing.T) {
	dbPath := createTestSQLiteDB(t)

	dbType, err := DetectType(dbPath)
	if err != nil {
		t.Fatalf("DetectType error: %v", err)
	}
	if dbType != "sqlite" {
		t.Errorf("expected sqlite, got %s", dbType)
	}

	// Test duckdb extension
	tmpDuck := filepath.Join(t.TempDir(), "analytics.duckdb")
	_ = os.WriteFile(tmpDuck, []byte("DUCKdummydata"), 0644)
	duckType, _ := DetectType(tmpDuck)
	if duckType != "duckdb" {
		t.Errorf("expected duckdb, got %s", duckType)
	}

	// Test rocksdb directory
	tmpRocks := filepath.Join(t.TempDir(), "store.rocksdb")
	_ = os.MkdirAll(tmpRocks, 0755)
	_ = os.WriteFile(filepath.Join(tmpRocks, "CURRENT"), []byte("MANIFEST-000001\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpRocks, "MANIFEST-000001"), []byte("manifest"), 0644)
	_ = os.WriteFile(filepath.Join(tmpRocks, "000002.sst"), []byte("sst data"), 0644)
	rocksType, _ := DetectType(tmpRocks)
	if rocksType != "rocksdb" {
		t.Errorf("expected rocksdb, got %s", rocksType)
	}
}

func TestInspectDatabase(t *testing.T) {
	dbPath := createTestSQLiteDB(t)

	info, err := InspectDatabase(dbPath)
	if err != nil {
		t.Fatalf("InspectDatabase failed: %v", err)
	}

	if info.Type != "sqlite" {
		t.Errorf("expected sqlite, got %s", info.Type)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(info.Tables))
	}
	if info.Tables[0].Name != "users" {
		t.Errorf("expected table 'users', got %s", info.Tables[0].Name)
	}
	if info.Tables[0].RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", info.Tables[0].RowCount)
	}
	if len(info.Views) != 1 || info.Views[0].Name != "active_users" {
		t.Errorf("expected 1 view 'active_users', got %+v", info.Views)
	}

	// Verify columns
	cols := info.Tables[0].Columns
	if len(cols) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || !cols[0].IsPK {
		t.Errorf("expected primary key 'id', got %+v", cols[0])
	}
	if cols[1].Name != "name" || !cols[1].NotNull {
		t.Errorf("expected not null 'name', got %+v", cols[1])
	}
}

func TestGetTableRows(t *testing.T) {
	dbPath := createTestSQLiteDB(t)

	// Fetch page 1 (limit 2)
	res, err := GetTableRows(dbPath, "users", 2, 0, "id", true, "", "")
	if err != nil {
		t.Fatalf("GetTableRows failed: %v", err)
	}
	if res.TotalRows != 3 {
		t.Errorf("expected total rows 3, got %d", res.TotalRows)
	}
	if len(res.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(res.Rows))
	}
	if res.Rows[0]["name"] != "Alice Johnson" {
		t.Errorf("expected first user Alice Johnson, got %v", res.Rows[0]["name"])
	}

	// Filter by name
	filterRes, err := GetTableRows(dbPath, "users", 10, 0, "", false, "name", "Charlie")
	if err != nil {
		t.Fatalf("filtered GetTableRows failed: %v", err)
	}
	if filterRes.TotalRows != 1 || len(filterRes.Rows) != 1 {
		t.Fatalf("expected 1 filtered row, got total %d, rows %d", filterRes.TotalRows, len(filterRes.Rows))
	}
	if filterRes.Rows[0]["name"] != "Charlie Doe" {
		t.Errorf("expected Charlie Doe, got %v", filterRes.Rows[0]["name"])
	}
}

func TestExecuteDatabaseQuery(t *testing.T) {
	dbPath := createTestSQLiteDB(t)

	// SELECT query
	res, err := ExecuteDatabaseQuery(dbPath, "SELECT id, name FROM users WHERE role = 'admin';", 10)
	if err != nil {
		t.Fatalf("ExecuteDatabaseQuery failed: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("query returned error: %s", res.Error)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Rows))
	}

	// Reject write query
	writeRes, err := ExecuteDatabaseQuery(dbPath, "DROP TABLE users;", 10)
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if !strings.Contains(writeRes.Error, "forbidden in read-only mode") {
		t.Errorf("expected forbidden error, got: %s", writeRes.Error)
	}
}
