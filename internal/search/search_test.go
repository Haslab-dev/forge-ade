package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilenameSearch(t *testing.T) {
	sm := NewSearchManager()

	tmpDir, err := os.MkdirTemp("", "search_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "App.tsx")
	file2 := filepath.Join(tmpDir, "components", "Sidebar.tsx")
	file3 := filepath.Join(tmpDir, "utils", "search.go")

	_ = os.MkdirAll(filepath.Dir(file2), 0755)
	_ = os.MkdirAll(filepath.Dir(file3), 0755)

	_ = os.WriteFile(file1, []byte("import React from 'react';"), 0644)
	_ = os.WriteFile(file2, []byte("export function Sidebar() { return null; }"), 0644)
	_ = os.WriteFile(file3, []byte("package search\nfunc Search() {}"), 0644)

	sm.SetDirectories([]string{tmpDir})

	res := sm.SearchFilename("Sidebar", 10)
	if len(res) == 0 {
		t.Fatalf("expected search result for 'Sidebar', got 0")
	}
	if res[0].Filename != "Sidebar.tsx" {
		t.Errorf("expected Sidebar.tsx, got %s", res[0].Filename)
	}

	resPath := sm.SearchFilename("components/Sidebar", 10)
	if len(resPath) == 0 {
		t.Fatalf("expected search result for 'components/Sidebar', got 0")
	}
}

func TestContentSearch(t *testing.T) {
	sm := NewSearchManager()

	tmpDir, err := os.MkdirTemp("", "search_content_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(file1, []byte("hello world\nthis is a secret token 12345\nend of file"), 0644)

	sm.SetDirectories([]string{tmpDir})

	results, err := sm.SearchContent("secret token", 10)
	if err != nil {
		t.Fatalf("search content failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected content match for 'secret token', got 0")
	}
	if results[0].Line != 2 {
		t.Errorf("expected line 2 match, got line %d", results[0].Line)
	}
}

func TestMethodAndTypeSearch(t *testing.T) {
	sm := NewSearchManager()

	tmpDir, err := os.MkdirTemp("", "search_type_method_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	code := `package main

type UserProfile struct {
	ID   int
	Name string
}

func (u *UserProfile) GetName() string {
	return u.Name
}

interface ComponentProps {
	title: string
}
`
	filePath := filepath.Join(tmpDir, "types.go")
	_ = os.WriteFile(filePath, []byte(code), 0644)

	sm.SetDirectories([]string{tmpDir})

	// 1. Search for type definition
	typeRes, err := sm.SearchContent("type UserProfile struct", 10)
	if err != nil || len(typeRes) == 0 {
		t.Fatalf("expected type definition match, got 0 (err: %v)", err)
	}
	if !strings.Contains(typeRes[0].Content, "UserProfile") {
		t.Errorf("expected UserProfile in result content, got: %s", typeRes[0].Content)
	}

	// 2. Search for method definition
	methodRes, err := sm.SearchContent("func (u *UserProfile) GetName", 10)
	if err != nil || len(methodRes) == 0 {
		t.Fatalf("expected method definition match, got 0 (err: %v)", err)
	}
	if methodRes[0].Line != 8 {
		t.Errorf("expected line 8 for GetName method, got line %d", methodRes[0].Line)
	}

	// 3. Search for symbols specifically via SearchSymbols
	symRes, err := sm.SearchSymbols("UserProfile", 10)
	if err != nil || len(symRes) == 0 {
		t.Fatalf("expected SearchSymbols match for 'UserProfile', got 0 (err: %v)", err)
	}
}

func TestSearchOptions(t *testing.T) {
	sm := NewSearchManager()

	tmpDir, err := os.MkdirTemp("", "search_opts_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	code := `func ProcessUser() {}
func processuser() {}
var user = "john"
var user_name = "john_doe"
`
	filePath := filepath.Join(tmpDir, "opts.go")
	_ = os.WriteFile(filePath, []byte(code), 0644)

	sm.SetDirectories([]string{tmpDir})

	// 1. Match Case Test
	csRes, _ := sm.SearchContentWithOptions(SearchOptions{
		Query:     "ProcessUser",
		MatchCase: true,
		Limit:     10,
	})
	if len(csRes) != 1 {
		t.Errorf("expected 1 case-sensitive match for 'ProcessUser', got %d", len(csRes))
	}

	// 2. Whole Word Test
	wwRes, _ := sm.SearchContentWithOptions(SearchOptions{
		Query:          "user",
		MatchWholeWord: true,
		Limit:          10,
	})
	if len(wwRes) != 1 {
		t.Errorf("expected 1 whole word match for 'user', got %d", len(wwRes))
	}

	// 3. Regex Test
	rxRes, _ := sm.SearchContentWithOptions(SearchOptions{
		Query:    `user_\w+`,
		UseRegex: true,
		Limit:    10,
	})
	if len(rxRes) != 1 {
		t.Errorf("expected 1 regex match for 'user_\\w+', got %d", len(rxRes))
	}
}
