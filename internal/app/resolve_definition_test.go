package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDefinition_ImportedBy(t *testing.T) {
	tmpDir := t.TempDir()

	// File a.ts exports foo and a default class
	aContent := `export const foo = "hello from a";
export default class MyDefaultA {
    constructor() {}
}
`
	aPath := filepath.Join(tmpDir, "a.ts")
	if err := os.WriteFile(aPath, []byte(aContent), 0644); err != nil {
		t.Fatalf("failed to write a.ts: %v", err)
	}

	// File b.ts imports foo and default from ./a
	bContent := `import MyDefaultA, { foo } from './a';

console.log(foo);
const instance = new MyDefaultA();
`
	bPath := filepath.Join(tmpDir, "b.ts")
	if err := os.WriteFile(bPath, []byte(bContent), 0644); err != nil {
		t.Fatalf("failed to write b.ts: %v", err)
	}

	app := &App{}

	// Test 1: Querying "foo" from b.ts should resolve to a.ts at line 1
	resFoo := app.ResolveDefinition(bPath, "foo")
	if resFoo == nil {
		t.Fatalf("expected foo to resolve, got nil")
	}
	if resFoo.File != aPath {
		t.Errorf("expected file %s, got %s", aPath, resFoo.File)
	}
	if resFoo.Line != 1 {
		t.Errorf("expected line 1, got %d", resFoo.Line)
	}

	// Test 2: Querying "MyDefaultA" from b.ts should resolve to a.ts at line 2
	resDefault := app.ResolveDefinition(bPath, "MyDefaultA")
	if resDefault == nil {
		t.Fatalf("expected MyDefaultA to resolve, got nil")
	}
	if resDefault.File != aPath {
		t.Errorf("expected file %s, got %s", aPath, resDefault.File)
	}
	if resDefault.Line != 2 {
		t.Errorf("expected line 2, got %d", resDefault.Line)
	}

	// Test 3: Querying the import path './a' from b.ts should resolve to a.ts
	resPath := app.ResolveDefinition(bPath, "./a")
	if resPath == nil {
		t.Fatalf("expected './a' to resolve, got nil")
	}
	if resPath.File != aPath {
		t.Errorf("expected file %s, got %s", aPath, resPath.File)
	}
}

func TestResolveDefinition_GoSibling(t *testing.T) {
	tmpDir := t.TempDir()

	// File manager.go defines NewManager
	mgrContent := `package mypkg

func NewManager(dir string) *Manager {
	return &Manager{}
}
`
	mgrPath := filepath.Join(tmpDir, "manager.go")
	if err := os.WriteFile(mgrPath, []byte(mgrContent), 0644); err != nil {
		t.Fatalf("failed to write manager.go: %v", err)
	}

	// File manager_test.go calls NewManager
	testContent := `package mypkg

import "testing"

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	_ = m
}
`
	testPath := filepath.Join(tmpDir, "manager_test.go")
	if err := os.WriteFile(testPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write manager_test.go: %v", err)
	}

	app := &App{}

	// Querying "NewManager" from manager_test.go should resolve to manager.go
	res := app.ResolveDefinition(testPath, "NewManager")
	if res == nil {
		t.Fatalf("expected NewManager to resolve to sibling file, got nil")
	}
	if res.File != mgrPath {
		t.Errorf("expected file %s, got %s", mgrPath, res.File)
	}
	if res.Line != 3 {
		t.Errorf("expected line 3, got %d", res.Line)
	}
}

func TestResolveDefinition_ForgeReal(t *testing.T) {
	app := &App{}
	cwd, _ := os.Getwd()

	// In tests under internal/app, cwd is /data/workspace/forge-ade/internal/app
	repoRoot := filepath.Clean(filepath.Join(cwd, "../.."))
	currentFile := filepath.Join(repoRoot, "frontend/src/features/automations/automations.api.ts")

	// Test A: path '../../lib/wails'
	resPath := app.ResolveDefinition(currentFile, "../../lib/wails")
	t.Logf("resPath: %+v", resPath)
	if resPath == nil || !strings.HasSuffix(resPath.File, "lib/wails.ts") {
		t.Errorf("expected to resolve to wails.ts, got %+v", resPath)
	}

	// Test B: symbol 'WailsListAutomations'
	resSym := app.ResolveDefinition(currentFile, "WailsListAutomations")
	t.Logf("resSym: %+v", resSym)
	if resSym == nil || !strings.HasSuffix(resSym.File, "lib/wails.ts") {
		t.Errorf("expected to resolve to wails.ts, got %+v", resSym)
	}
	if resSym.Line <= 0 {
		t.Errorf("expected valid line in wails.ts, got %d", resSym.Line)
	}

	// Test C: symbol 'ListAutomations'
	resOrig := app.ResolveDefinition(currentFile, "ListAutomations")
	t.Logf("resOrig: %+v", resOrig)
	if resOrig == nil || !strings.HasSuffix(resOrig.File, "lib/wails.ts") {
		t.Errorf("expected to resolve to wails.ts, got %+v", resOrig)
	}
}

func TestResolveDefinition_BarrelAndDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// sub.ts defines helper
	subContent := `export function helper() { return 42; }
`
	subPath := filepath.Join(tmpDir, "sub.ts")
	_ = os.WriteFile(subPath, []byte(subContent), 0644)

	// index.ts re-exports helper from ./sub
	indexContent := `export { helper } from './sub';
`
	indexPath := filepath.Join(tmpDir, "index.ts")
	_ = os.WriteFile(indexPath, []byte(indexContent), 0644)

	// main.ts imports helper from ./index
	mainContent := `import { helper } from './index';
console.log(helper());
`
	mainPath := filepath.Join(tmpDir, "main.ts")
	_ = os.WriteFile(mainPath, []byte(mainContent), 0644)

	app := &App{}

	// Querying "helper" from main.ts should resolve through index.ts directly to sub.ts!
	res := app.ResolveDefinition(mainPath, "helper")
	if res == nil {
		t.Fatalf("expected helper to resolve, got nil")
	}
	if res.File != subPath {
		t.Errorf("expected barrel resolution to %s, got %s", subPath, res.File)
	}
	if res.Line != 1 {
		t.Errorf("expected line 1 in sub.ts, got %d", res.Line)
	}
}

func TestResolveDefinition_PythonImport(t *testing.T) {
	tmpDir := t.TempDir()

	// a.py defines calculate
	aContent := `def calculate(x):
    return x * 2
`
	aPath := filepath.Join(tmpDir, "a.py")
	_ = os.WriteFile(aPath, []byte(aContent), 0644)

	// b.py imports calculate from a
	bContent := `from a import calculate as calc

print(calc(10))
`
	bPath := filepath.Join(tmpDir, "b.py")
	_ = os.WriteFile(bPath, []byte(bContent), 0644)

	app := &App{}

	// Querying "calc" from b.py should resolve to a.py line 1
	res := app.ResolveDefinition(bPath, "calc")
	if res == nil {
		t.Fatalf("expected calc to resolve, got nil")
	}
	if res.File != aPath {
		t.Errorf("expected %s, got %s", aPath, res.File)
	}
	if res.Line != 1 {
		t.Errorf("expected line 1 in a.py, got %d", res.Line)
	}
}

func TestResolveDefinition_SamplesAB(t *testing.T) {
	tmpDir := t.TempDir()
	aPath := filepath.Join(tmpDir, "a.ts")
	bPath := filepath.Join(tmpDir, "b.ts")

	aContent := `export const greetA = "Hello from A";

export function computeA(x: number): number {
  return x * 10;
}

export default class ServiceA {
  run() {
    console.log("ServiceA is running");
  }
}
`
	bContent := `import ServiceA, { greetA, computeA } from './a';

console.log(greetA);
const res = computeA(5);
const s = new ServiceA();
`
	_ = os.WriteFile(aPath, []byte(aContent), 0644)
	_ = os.WriteFile(bPath, []byte(bContent), 0644)

	app := &App{}

	// 1. greetA -> a.ts line 1
	resGreet := app.ResolveDefinition(bPath, "greetA")
	if resGreet == nil || resGreet.File != aPath || resGreet.Line != 1 {
		t.Fatalf("expected greetA to resolve to a.ts:1, got %+v", resGreet)
	}

	// 2. computeA -> a.ts line 3
	resCompute := app.ResolveDefinition(bPath, "computeA")
	if resCompute == nil || resCompute.File != aPath || resCompute.Line != 3 {
		t.Fatalf("expected computeA to resolve to a.ts:3, got %+v", resCompute)
	}

	// 3. ServiceA -> a.ts line 7
	resService := app.ResolveDefinition(bPath, "ServiceA")
	if resService == nil || resService.File != aPath || resService.Line != 7 {
		t.Fatalf("expected ServiceA to resolve to a.ts:7, got %+v", resService)
	}

	// 4. './a' path literal -> a.ts line 1
	resPath := app.ResolveDefinition(bPath, "./a")
	if resPath == nil || resPath.File != aPath || resPath.Line != 1 {
		t.Fatalf("expected './a' to resolve to a.ts:1, got %+v", resPath)
	}
}
