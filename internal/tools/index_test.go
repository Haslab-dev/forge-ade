package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexGoFile(t *testing.T) {
	src := `package sample

import (
	"fmt"
	"strings"
)

type User struct {
	Name string
	Age  int
}

type Greeter interface {
	Greet() string
}

const DefaultMax = 100

func NewUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

func (u *User) Greet() string {
	return fmt.Sprintf("Hello %s", u.Name)
}
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "user.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tool := indexTool()
	res, err := tool.Handler(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("indexTool failed: %v", err)
	}

	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}

	skeleton, ok := m["skeleton"].(string)
	if !ok {
		t.Fatalf("expected skeleton string, got %T", m["skeleton"])
	}

	for _, expected := range []string{
		"package sample",
		"type User struct [8-11]",
		"type Greeter interface [13-15]",
		"func NewUser(name string, age int) *User [19-21]",
		"func (u *User) Greet() string [23-25]",
	} {
		if !strings.Contains(skeleton, expected) {
			t.Errorf("expected skeleton to contain %q, but got:\n%s", expected, skeleton)
		}
	}
}

func TestIndexTypeScriptFile(t *testing.T) {
	src := `import { useState } from 'react';
import type { User } from './types';

export interface Props {
  name: string;
}

export class Button {
  render() {}
}

export function greet(name: string): string {
  return "hello " + name;
}
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "comp.tsx")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tool := indexTool()
	res, err := tool.Handler(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("indexTool failed: %v", err)
	}

	m := res.(map[string]any)
	skeleton := m["skeleton"].(string)

	for _, expected := range []string{
		"interface Props",
		"class Button",
		"greet",
	} {
		if !strings.Contains(skeleton, expected) {
			t.Errorf("expected skeleton to contain %q, but got:\n%s", expected, skeleton)
		}
	}
}

func TestIndexMarkdownFile(t *testing.T) {
	src := `# Title

Some intro text

## Section 1
Content 1

### Subsection A
More content

## Section 2
Final thoughts
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "doc.md")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tool := indexTool()
	res, err := tool.Handler(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("indexTool failed: %v", err)
	}

	m := res.(map[string]any)
	skeleton := m["skeleton"].(string)

	for _, expected := range []string{
		"# Title [1]",
		"## Section 1 [5]",
		"### Subsection A [8]",
		"## Section 2 [11]",
	} {
		if !strings.Contains(skeleton, expected) {
			t.Errorf("expected skeleton to contain %q, but got:\n%s", expected, skeleton)
		}
	}
}
