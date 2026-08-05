package index

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDepTree(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Dependency index: only exports are recorded, only the export graph is
// followed — no full .d.ts parse.
func TestDepIndex(t *testing.T) {
	dir := writeDepTree(t, map[string]string{
		"node_modules/react/package.json": `{"name":"react","types":"index.d.ts"}`,
		"node_modules/react/index.d.ts": `import { FC } from "./types";
export function useState<S>(i: S): [S, (s: S) => void];
export function useEffect(e: () => void): void;
export interface Component { render(): void }
export const version = "18.0.0";
export * from "./hooks";
declare namespace React { const Fragment: symbol }
`,
		"node_modules/react/hooks.d.ts": `export function useReducer(r: any, s: any): any;
export { useState as useLegacyState } from "./index";
`,
		"node_modules/react/types.d.ts": `export interface FC { (): any }
`,
		"src/app.tsx": `import React, { useState, useEffect } from "react";
export function App() { return null }
`,
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	mods := map[string]string{}
	for _, sym := range s.symbols {
		if sym.Module == "react" {
			got[sym.Name] = sym.Kind
			mods[sym.Name] = sym.File
		}
	}
	want := map[string]SymbolKind{
		"useState": Function, "useEffect": Function, "Component": Interface,
		"version": Constant, "useReducer": Function,
		"useLegacyState": Variable, "React": Package,
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("dep %s = %v, want %v (got: %v)", name, got[name], kind, got)
		}
	}
	// go-to-def: dep symbols point at the real .d.ts line
	if !filepath.HasPrefix(mods["useState"], dir+"/node_modules/react/") {
		t.Errorf("useState file = %q", mods["useState"])
	}

}

// Completion tiering: current file → workspace → dependency.
func TestDepCompletionTiers(t *testing.T) {
	dir := writeDepTree(t, map[string]string{
		"node_modules/react/package.json": `{"types":"index.d.ts"}`,
		"node_modules/react/index.d.ts":   `export function useState(): any;`,
		"src/a.ts": `import { useState } from "react";
export function useLocal() {}
export const useUser = 1;
`,
		"src/b.ts": `export const useOther = 2;`,
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	c := s.Completion("use", "typescript", dir+"/src/a.ts")
	names := symbolNames(c)
	// order: local first, then workspace, then dependency
	if len(names) < 3 {
		t.Fatalf("completion = %v, want >= 3", names)
	}
	if names[0] != "useLocal" && names[0] != "useUser" {
		t.Errorf("tier1/2 expected first, got %v", names)
	}
	if names[len(names)-1] != "useState" {
		t.Errorf("dep should be last, got %v", names)
	}
}

// Dependency symbols must not break language isolation or cross into Go.
func TestDepNoCrossLang(t *testing.T) {
	dir := writeDepTree(t, map[string]string{
		"node_modules/react/package.json": `{"types":"index.d.ts"}`,
		"node_modules/react/index.d.ts":   `export function useState(): any;`,
		"src/a.ts":                        `import { useState } from "react";`,
		"src/b.go":                        `package main`,
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if c := s.Completion("use", "go", dir+"/src/b.go"); len(c) != 0 {
		t.Fatalf("go completion leaked dep: %v", symbolNames(c))
	}
	if c := s.Completion("use", "typescript", dir+"/src/a.ts"); len(c) != 1 {
		t.Fatalf("ts completion = %v, want useState", symbolNames(c))
	}
}

// React-style: `declare namespace React { function useState... }` + @types
// fallback when the package has no types field.
func TestDepNamespaceAndTypesFallback(t *testing.T) {
	dir := writeDepTree(t, map[string]string{
		"node_modules/react/package.json":        `{"name":"react","main":"index.js"}`,
		"node_modules/react/index.js":            `module.exports = {};`,
		"node_modules/@types/react/index.d.ts": `export = React;
export as namespace React;
declare namespace React {
    function useState<S>(i: S): [S, (s: S) => void];
    function useEffect(e: () => void): void;
    interface FC { children?: any }
    const version: string;
}
`,
		"src/a.ts": `import * as React from "react";`,
	})
	s := New(dir)
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	for _, sym := range s.symbols {
		if sym.Module == "react" {
			got[sym.Name] = sym.Kind
		}
	}
	for name, kind := range map[string]SymbolKind{
		"useState": Function, "useEffect": Function, "FC": Interface,
		"version": Constant, "React": Package,
	} {
		if got[name] != kind {
			t.Errorf("dep %s = %v, want %v (got %v)", name, got[name], kind, got)
		}
	}
}
