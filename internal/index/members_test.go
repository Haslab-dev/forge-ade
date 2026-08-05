package index

import (
	"fmt"
	"os"
	"testing"
)

func TestMemberCompletion(t *testing.T) {
	src := `export class Data {
  size: number = 0
  items: string[] = []
  connect(): void {}
  disconnect(): void {}
}
export function makeConfig() {
  return { host: "x", port: 8080, retry(): void {} }
}
export const buildThing = () => ({ id: 1, name: "a", run: () => {} })
export interface Options { timeout: number; onReady(): void }
const data = new Data();
const cfg = makeConfig();
const thing = buildThing();
const opts: Options = {};
const nested = { a: { b: 1 }, plain: 2, method() {} };
`
	tmp := t.TempDir() + "/x.ts"
	os.WriteFile(tmp, []byte(src), 0644)
	s := New(t.TempDir())
	if err := s.Update(tmp); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"data", "cfg", "thing", "opts", "nested", "nope"} {
		m := s.Members(name)
		names := []string{}
		for _, x := range m {
			names = append(names, fmt.Sprintf("%s:%s", x.Name, x.Kind))
		}
		fmt.Printf("%-8s → %v\n", name, names)
	}
}
