package index

import "testing"

func TestGoParse(t *testing.T) {
	src := `package main
type User struct{ Name string }
type Repo interface{ Get(id int) User }
const MaxRetries = 3
var version = "1.0"
func main() {}
func (u *User) Greet() string { return "" }`
	p, err := Parse([]byte(src), &File{Path: "x.go", Language: LangGo})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	for _, s := range p.Symbols {
		got[s.Name] = s.Kind
	}
	for name, kind := range map[string]SymbolKind{
		"User": Struct, "Repo": Interface, "MaxRetries": Constant,
		"version": Variable, "main": Function, "Greet": Function,
	} {
		if got[name] != kind {
			t.Errorf("go: %s = %v, want %v", name, got[name], kind)
		}
	}
}

func TestJavaParse(t *testing.T) {
	src := `package com.x;
public class App {
  private String name;
  public static final int MAX = 10;
  public void run() {}
}
interface Service {
  void serve();
}
enum Color { RED, GREEN }
record Point(int x, int y) {}`
	p, err := Parse([]byte(src), &File{Path: "x.java", Language: LangJava})
	if err != nil {
		t.Fatal(err)
	}
	var appName, runScope, maxKind, colorKind string
	maxKind = "none"
	for _, s := range p.Symbols {
		switch s.Name {
		case "App":
			appName = s.Kind.String()
		case "run":
			runScope = s.Scope
		case "MAX":
			maxKind = s.Kind.String()
		case "Color":
			colorKind = s.Kind.String()
		}
	}
	if appName != "class" || runScope != "App" || maxKind != "constant" || colorKind != "enum" {
		t.Errorf("java: App=%s run.scope=%s MAX=%s Color=%s", appName, runScope, maxKind, colorKind)
	}
}

func TestKotlinParse(t *testing.T) {
	src := `package com.x
class User(val name: String)
data class Pair(val a: Int, val b: Int)
enum class Color { RED, GREEN }
interface Repo {
  fun get(id: Int): User
}
typealias Callback = (Int) -> Unit
const val LIMIT = 5
fun main() {}
fun User.greet(): String = ""`
	p, err := Parse([]byte(src), &File{Path: "x.kt", Language: LangKotlin})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	scopes := map[string]string{}
	for _, s := range p.Symbols {
		got[s.Name] = s.Kind
		scopes[s.Name] = s.Scope
	}
	for name, kind := range map[string]SymbolKind{
		"User": Class, "Pair": Class, "Color": Enum, "Repo": Interface,
		"Callback": TypeAlias, "LIMIT": Constant, "main": Function, "greet": Function,
	} {
		if got[name] != kind {
			t.Errorf("kotlin: %s = %v, want %v", name, got[name], kind)
		}
	}
	if scopes["get"] != "Repo" {
		t.Errorf("kotlin: get scope = %q, want Repo", scopes["get"])
	}
}

func TestSwiftParse(t *testing.T) {
	src := `import Foundation
class App {
  let name: String
  var count = 0
  func run() {}
}
struct Point {
  var x: Int
  var y: Int
}
protocol Drawable {
  func draw()
}
let MAX = 100
func main() {}`
	p, err := Parse([]byte(src), &File{Path: "x.swift", Language: LangSwift})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	scopes := map[string]string{}
	for _, s := range p.Symbols {
		got[s.Name] = s.Kind
		scopes[s.Name] = s.Scope
	}
	for name, kind := range map[string]SymbolKind{
		"App": Class, "Point": Struct, "Drawable": Interface,
		"MAX": Constant, "main": Function, "x": Variable,
	} {
		if got[name] != kind {
			t.Errorf("swift: %s = %v, want %v", name, got[name], kind)
		}
	}
	if scopes["run"] != "App" || scopes["draw"] != "Drawable" {
		t.Errorf("swift: run=%q draw=%q", scopes["run"], scopes["draw"])
	}
}

func TestDartParse(t *testing.T) {
	src := `import 'dart:io';
class App {
  String name;
  void run() {}
}
typedef IntCallback = void Function(int);
const MAX = 100;
void main() {}`
	p, err := Parse([]byte(src), &File{Path: "x.dart", Language: LangDart})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	for _, s := range p.Symbols {
		got[s.Name] = s.Kind
	}
	for name, kind := range map[string]SymbolKind{
		"App": Class, "IntCallback": TypeAlias, "MAX": Constant, "main": Function,
	} {
		if got[name] != kind {
			t.Errorf("dart: %s = %v, want %v", name, got[name], kind)
		}
	}
	if _, ok := got["Function"]; ok {
		t.Errorf("dart: spurious Function symbol from typedef")
	}
}

func TestPythonParse(t *testing.T) {
	src := `import os
CONST_MAX = 100
app_name = "x"

class Database:
    def __init__(self, host):
        pass

    def connect(self):
        pass

def make_user(name, age=30):
    return {"name": name}

async def fetch(url):
    pass`
	p, err := Parse([]byte(src), &File{Path: "x.py", Language: LangPython})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SymbolKind{}
	scopes := map[string]string{}
	for _, s := range p.Symbols {
		got[s.Name] = s.Kind
		scopes[s.Name] = s.Scope
	}
	for name, kind := range map[string]SymbolKind{
		"CONST_MAX": Constant, "app_name": Variable, "Database": Class,
		"make_user": Function, "fetch": Function,
	} {
		if got[name] != kind {
			t.Errorf("python: %s = %v, want %v", name, got[name], kind)
		}
	}
	if scopes["connect"] != "Database" || scopes["__init__"] != "Database" {
		t.Errorf("python: connect=%q __init__=%q", scopes["connect"], scopes["__init__"])
	}
}
