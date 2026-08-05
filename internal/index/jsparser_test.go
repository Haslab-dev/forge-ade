package index

import (
	"reflect"
	"testing"
)

func parseJS(t *testing.T, src string) *ParseResult {
	t.Helper()
	f := &File{ID: 1, Path: "test.ts", Language: LangTypeScript}
	res, err := Parse([]byte(src), f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}

func names(syms []Symbol) []string {
	var out []string
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

func TestParseFunctions(t *testing.T) {
	res := parseJS(t, `
function foo() {}
export function bar(a, b) { return a }
export default function baz() {}
async function qux() {}
function* gen() {}
`)
	got := names(res.Symbols)
	want := []string{"foo", "bar", "baz", "qux", "gen"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// exported flags
	byName := map[string]Symbol{}
	for _, s := range res.Symbols {
		byName[s.Name] = s
	}
	if !byName["bar"].Exported {
		t.Error("bar should be exported")
	}
	if byName["foo"].Exported {
		t.Error("foo should not be exported")
	}
}

func TestParseClassAndMethods(t *testing.T) {
	res := parseJS(t, `
export class Foo extends Bar {
  constructor() {}
  greet(name) {}
  static make() {}
  get value() { return 1 }
  #private() {}
  prop = 5;
  arrow = () => {};
}
`)
	var classSym *Symbol
	methods := map[string]Symbol{}
	for i := range res.Symbols {
		s := &res.Symbols[i]
		if s.Kind == Class {
			classSym = s
		} else if s.Kind == Method || s.Kind == Function {
			methods[s.Name] = *s
		}
	}
	if classSym == nil || classSym.Name != "Foo" || !classSym.Exported {
		t.Fatalf("class Foo not found/exported: %+v", classSym)
	}
	for _, m := range []string{"constructor", "greet", "make", "value"} {
		if _, ok := methods[m]; !ok {
			t.Errorf("method %s not found", m)
		}
	}
	if methods["greet"].Scope != "Foo" {
		t.Errorf("greet scope = %q, want Foo", methods["greet"].Scope)
	}
	if _, ok := methods["arrow"]; ok {
		t.Error("arrow property should not be a method")
	}
}

func TestParseInterfacesAndTypes(t *testing.T) {
	res := parseJS(t, `
interface User { id: number }
enum Color { Red, Green }
type Point = { x: number };
export interface Admin extends User {}
`)
	kinds := map[string]SymbolKind{}
	for _, s := range res.Symbols {
		kinds[s.Name] = s.Kind
	}
	if kinds["User"] != Interface || kinds["Admin"] != Interface {
		t.Errorf("interface kinds wrong: %v", kinds)
	}
	if kinds["Color"] != Enum {
		t.Error("enum not detected")
	}
	if kinds["Point"] != TypeAlias {
		t.Error("type alias not detected")
	}
}

func TestParseVars(t *testing.T) {
	res := parseJS(t, `
const PORT = 3000;
let count = 0;
var legacy = 1;
const add = (a, b) => a + b;
const inc = n => n + 1;
const obj = { x: 1 };
const { a, b } = destructure();
`)
	kinds := map[string]SymbolKind{}
	for _, s := range res.Symbols {
		kinds[s.Name] = s.Kind
	}
	if kinds["PORT"] != Constant {
		t.Errorf("PORT kind = %v, want constant", kinds["PORT"])
	}
	if kinds["count"] != Variable || kinds["legacy"] != Variable {
		t.Error("count/legacy should be variables")
	}
	if kinds["add"] != Function || kinds["inc"] != Function {
		t.Error("arrow constants should be functions")
	}
	if kinds["obj"] != Constant {
		t.Error("obj should be constant")
	}
	if _, ok := kinds["a"]; ok {
		t.Error("destructured names should not be symbols")
	}
}

func TestParseImports(t *testing.T) {
	res := parseJS(t, `
import fs from "node:fs";
import { readFile, writeFile as wf } from "./io";
import * as net from "net";
import "reflect-metadata";
const path = require("node:path");
`)
	got := res.Imports
	if len(got) != 5 {
		t.Fatalf("got %d imports, want 5: %+v", len(got), got)
	}
	if got[0].Path != `"node:fs"` || !reflect.DeepEqual(got[0].Names, []string{"fs"}) {
		t.Errorf("import 0 wrong: %+v", got[0])
	}
	if got[1].Path != `"./io"` || !reflect.DeepEqual(got[1].Names, []string{"readFile", "writeFile"}) {
		t.Errorf("import 1 wrong: %+v", got[1])
	}
	if got[2].Path != `"net"` || !reflect.DeepEqual(got[2].Names, []string{"net"}) {
		t.Errorf("import 2 wrong: %+v", got[2])
	}
	if got[3].Path != `"reflect-metadata"` || len(got[3].Names) != 0 {
		t.Errorf("import 3 wrong: %+v", got[3])
	}
	if got[4].Path != `"node:path"` {
		t.Errorf("import 4 wrong: %+v", got[4])
	}
}

func TestParseExports(t *testing.T) {
	res := parseJS(t, `
export { a, b as c };
export const x = 1;
`)
	var names []string
	for _, e := range res.Exports {
		names = append(names, e.Name)
	}
	if !reflect.DeepEqual(names, []string{"a", "b"}) {
		t.Fatalf("exports wrong: %v", names)
	}
}

func TestParseIgnoresCommentsAndStrings(t *testing.T) {
	res := parseJS(t, "// function fake() {}\n/* class FakeClass {} */\nconst s = \"function notReal() {}\";\nconst tpl = `class TemplateThing {}`;\nfunction real() {}\n")
	if len(res.Symbols) != 3 {
		t.Fatalf("got %d symbols, want 3: %v", len(res.Symbols), names(res.Symbols))
	}
}

func TestParseBracesInStrings(t *testing.T) {
	res := parseJS(t, "class A {\n  m() { return \"{ not balanced\"; }\n  n() { return `${JSON.stringify({a:1})}`; }\n}\nfunction after() {}\n")
	var names2 []string
	for _, s := range res.Symbols {
		if s.Scope == "" {
			names2 = append(names2, s.Name)
		}
	}
	if !reflect.DeepEqual(names2, []string{"A", "after"}) {
		t.Fatalf("got %v", names2)
	}
}

func TestParseRegex(t *testing.T) {
	res := parseJS(t, `
function re() { return /[a-z]{2,}/.test("x"); }
class B { m() { return /}/.test("}"); } }
`)
	names2 := []string{}
	for _, s := range res.Symbols {
		if s.Scope == "" {
			names2 = append(names2, s.Name)
		}
	}
	if !reflect.DeepEqual(names2, []string{"re", "B"}) {
		t.Fatalf("got %v", names2)
	}
}

func TestSymbolPositions(t *testing.T) {
	res := parseJS(t, "function foo() {}\nclass Bar {}\n")
	var foo, bar *Symbol
	for i := range res.Symbols {
		s := &res.Symbols[i]
		if s.Name == "foo" {
			foo = s
		}
		if s.Name == "Bar" {
			bar = s
		}
	}
	if foo == nil || foo.Line != 1 || foo.Column != 10 {
		t.Errorf("foo position wrong: %+v", foo)
	}
	if bar == nil || bar.Line != 2 || bar.Column != 7 {
		t.Errorf("bar position wrong: %+v", bar)
	}
}
