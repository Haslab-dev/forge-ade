package index

import (
	"os"
	"testing"
)

// Two languages both declare `User` + a `User.` binding. Completion and
// members must not leak across languages.
func TestLangIsolation(t *testing.T) {
	s := New(t.TempDir())
	ts := `class User { name: string; age: number }
const u = new User();
export function tsOnly() {}`
	goSrc := `type User struct { Name string }
func (u *User) Greet() string { return "" }
func GoOnly() {}`
	dir := t.TempDir()
	for _, f := range []struct{ path, src string }{
		{"a.ts", ts}, {"b.go", goSrc},
	} {
		if err := os.WriteFile(dir+"/"+f.path, []byte(f.src), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.Update(dir + "/" + f.path); err != nil {
			t.Fatal(err)
		}
	}
	// completion: `u.` in .ts sees name/age, in .go sees nothing
	mts := s.Members("u", "typescript")
	if len(mts) != 2 {
		t.Fatalf("ts members = %v, want name/age", symbolNames(mts))
	}
	if mg := s.Members("u", "go"); len(mg) != 0 {
		t.Fatalf("go members of ts binding = %v, want none", symbolNames(mg))
	}
	// completion prefix: tsOnly only in typescript, GoOnly only in go
	if c := s.Completion("tsOn", "typescript"); len(c) != 1 || c[0].Name != "tsOnly" {
		t.Fatalf("ts completion = %v", symbolNames(c))
	}
	if c := s.Completion("tsOn", "go"); len(c) != 0 {
		t.Fatalf("go completion of ts sym = %v, want none", symbolNames(c))
	}
	// type-name direct access gated too
	if c := s.Completion("GoOn", "go"); len(c) != 1 {
		t.Fatalf("go completion = %v", symbolNames(c))
	}
	if m := s.Members("u", ""); len(m) != 2 {
		t.Fatalf("empty lang should still resolve: %v", symbolNames(m))
	}
}
