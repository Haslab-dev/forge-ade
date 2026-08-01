package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
)

func setupAnnotateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "T")
	for _, p := range []string{"root.txt", "sub/inner.txt", "sub/deep/far.txt"} {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0755)
		os.WriteFile(full, []byte("v1\n"), 0644)
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return dir
}

func buildTree(dir string, depth int) *explorer.FileInfo {
	exp := explorer.New(nil)
	exp.SetRoots([]string{dir})
	tree, err := exp.GetTree(depth)
	if err != nil || len(tree) == 0 {
		return nil
	}
	return tree[0]
}

func TestAnnotateNestedGitStatus(t *testing.T) {
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("MODIFIED\n"), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("ROOTMOD\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	root := buildTree(dir, 2)
	if root == nil {
		t.Fatal("failed to build tree")
	}
	annotateNodeGitStatus(root, dir, statusMap)

	if root.GitStatus == "" {
		t.Error("root dir should be marked dirty")
	}
	if root.Children == nil {
		t.Fatal("expected children")
	}
	find := func(name string) *explorer.FileInfo {
		for _, c := range root.Children {
			if c.Name == name {
				return c
			}
		}
		return nil
	}
	rootTxt := find("root.txt")
	if rootTxt == nil || rootTxt.GitStatus != "M" {
		t.Errorf("root.txt should be M, got %+v", rootTxt)
	}
	sub := find("sub")
	if sub == nil || sub.GitStatus == "" {
		t.Errorf("sub dir should be dirty, got %+v", sub)
	} else {
		inner := findChild(sub, "inner.txt")
		if inner == nil || inner.GitStatus != "M" {
			t.Errorf("sub/inner.txt should be M, got %+v", inner)
		}
	}
}

func findChild(parent *explorer.FileInfo, name string) *explorer.FileInfo {
	for _, c := range parent.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAnnotateFlatListing(t *testing.T) {
	// Simulates ExpandPath/ListDirectory output: a flat []*FileInfo of sibling
	// files AND directories (not a root folder). Top-level files must get
	// annotated too.
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("MODIFIED\n"), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("ROOTMOD\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	// Build a flat listing of the root dir (mirrors readDir output).
	exp := explorer.New(nil)
	listing, err := exp.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}

	for _, n := range listing {
		annotateNodeGitStatus(n, dir, statusMap)
	}

	for _, n := range listing {
		switch n.Name {
		case "root.txt":
			if n.GitStatus != "M" {
				t.Errorf("flat root.txt should be M, got %q", n.GitStatus)
			}
		case "sub":
			if n.GitStatus == "" {
				t.Errorf("flat sub dir should be dirty")
			}
		}
	}
}

func TestAnnotateUntrackedDeep(t *testing.T) {
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "deep", "brandnew.txt"), []byte("new\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	root := buildTree(dir, 3)
	annotateNodeGitStatus(root, dir, statusMap)

	sub := findChild(root, "sub")
	if sub == nil || sub.GitStatus == "" {
		t.Errorf("sub should be dirty, got %+v", sub)
	}
	deep := findChild(sub, "deep")
	if deep == nil || deep.GitStatus == "" {
		t.Errorf("sub/deep should be dirty, got %+v", deep)
	}
	brand := findChild(deep, "brandnew.txt")
	if brand == nil || brand.GitStatus != "U" {
		t.Errorf("brandnew.txt should be U, got %+v", brand)
	}
}
