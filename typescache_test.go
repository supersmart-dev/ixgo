package ixgo

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// cachedSrc uses a package checked from source (log), a patched one
// (sync's OnceValue, generic), a reflected one with methods (strings), and
// fmt, which reaches many.
const cachedSrc = `package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

func F(n int) string {
	var sb strings.Builder
	l := log.New(&sb, "", 0)
	l.Print("hi")
	v := sync.OnceValue(func() int { return n * 2 })
	return fmt.Sprintf("%s %d %s", strings.TrimSpace(sb.String()), v(), strings.Repeat("x", n))
}
`

// interpMu runs one interpreter at a time: without SupportMultipleInterp,
// each shares and resets reflectx's methods, cache or none.
var interpMu sync.Mutex

func runCached(t *testing.T, ctx *Context, n int) string {
	t.Helper()
	pkg, err := ctx.LoadFile("main.go", cachedSrc)
	if err != nil {
		t.Error(err)
		return ""
	}
	interpMu.Lock()
	defer interpMu.Unlock()
	it, err := ctx.NewInterp(pkg)
	if err != nil {
		t.Error(err)
		return ""
	}
	if err := it.RunInit(); err != nil {
		t.Error(err)
		return ""
	}
	v, err := it.RunFunc("F", n)
	if err != nil {
		t.Error(err)
		return ""
	}
	return v.(string)
}

func TestTypesCache(t *testing.T) {
	c, err := NewTypesCache(0)
	if err != nil {
		t.Fatal(err)
	}
	// Contexts on one cache, loading at once.
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 3 {
				n := g + i
				want := fmt.Sprintf("hi %d %s", n*2, strings.Repeat("x", n))
				if got := runCached(t, c.NewContext(), n); got != want {
					t.Errorf("F(%d) = %q, want %q", n, got, want)
				}
			}
		}()
	}
	wg.Wait()
}

func TestTypesCacheErrorsPlace(t *testing.T) {
	c, err := NewTypesCache(0)
	if err != nil {
		t.Fatal(err)
	}
	// The program's own positions come after the cache's files.
	_, err = c.NewContext().LoadFile("main.go", "package main\n\nimport \"strings\"\n\nfunc main() {\n\tstrings.Nope()\n}\n")
	if err == nil || !strings.HasPrefix(err.Error(), "main.go:6:10:") {
		t.Fatalf("err = %v, want one at main.go:6:10", err)
	}
}

// A package registered after the cache was made is loaded by each
// context, against the cache's types.
func TestTypesCacheLateRegister(t *testing.T) {
	c, err := NewTypesCache(0)
	if err != nil {
		t.Fatal(err)
	}
	const path = "github.com/goplus/ixgo/testdata/latecache"
	RegisterPackage(&Package{
		Name: "latecache", Path: path,
		Deps:          map[string]string{"strings": "strings"},
		Interfaces:    map[string]reflect.Type{},
		NamedTypes:    map[string]reflect.Type{},
		AliasTypes:    map[string]reflect.Type{},
		Vars:          map[string]reflect.Value{},
		TypedConsts:   map[string]TypedConst{},
		UntypedConsts: map[string]UntypedConst{},
		Funcs: map[string]reflect.Value{
			"New": reflect.ValueOf(func(s string) *strings.Builder {
				var sb strings.Builder
				sb.WriteString(s)
				return &sb
			}),
		},
	})
	ctx := c.NewContext()
	pkg, err := ctx.LoadFile("main.go", `package main

import (
	"strings"

	"`+path+`"
)

func F() string {
	var sb *strings.Builder = latecache.New("late")
	return sb.String()
}
`)
	if err != nil {
		t.Fatal(err)
	}
	it, err := ctx.NewInterp(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := it.RunFunc("F"); err != nil || v != "late" {
		t.Fatalf("F() = %v, %v", v, err)
	}
}

// A package the cache made up for a type's sake, which nothing
// registered, is no more importable from a context on it than from a
// fresh one, which finds it only by Lookup or GOROOT.
func TestTypesCacheMadeUpPackages(t *testing.T) {
	c, err := NewTypesCache(0)
	if err != nil {
		t.Fatal(err)
	}
	var madeUp []string
	for path := range c.loader.packages {
		if _, ok := registerPkgs[path]; !ok && path != "unsafe" && c.pkgs[path] == nil {
			madeUp = append(madeUp, path)
		}
	}
	if len(madeUp) == 0 {
		t.Skip("the cache made up no package")
	}
	closed := func(ctx *Context) *Context {
		ctx.Lookup = func(root, path string) (string, bool) { return "", false }
		ctx.BuildContext.GOROOT, ctx.BuildContext.GOPATH = "", ""
		return ctx
	}
	for _, path := range madeUp {
		src := fmt.Sprintf("package main\n\nimport _ %q\n\nfunc main() {}\n", path)
		_, fresh := closed(NewContext(0)).LoadFile("main.go", src)
		_, cached := closed(c.NewContext()).LoadFile("main.go", src)
		if (fresh == nil) != (cached == nil) {
			t.Errorf("importing %s: fresh %v, cached %v", path, fresh, cached)
		}
	}
}

func BenchmarkLoadFile(b *testing.B) {
	c, err := NewTypesCache(0)
	if err != nil {
		b.Fatal(err)
	}
	for _, bc := range []struct {
		name string
		ctx  func() *Context
	}{
		{"fresh", func() *Context { return NewContext(0) }},
		{"cached", c.NewContext},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := bc.ctx().LoadFile("main.go", cachedSrc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
