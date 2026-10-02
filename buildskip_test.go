package ixgo_test

import (
	"testing"

	"github.com/goplus/ixgo"
	_ "github.com/goplus/ixgo/pkg/cmp"
	_ "github.com/goplus/ixgo/pkg/iter"
	_ "github.com/goplus/ixgo/pkg/slices"
	_ "github.com/goplus/ixgo/pkg/sort"
)

func runF(t *testing.T, ctx *ixgo.Context, src string) (ixgo.Value, *ixgo.Interp) {
	t.Helper()
	pkg, err := ctx.LoadFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}
	it, err := ctx.NewInterp(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := it.RunInit(); err != nil {
		t.Fatal(err)
	}
	v, err := it.RunFunc("F")
	if err != nil {
		t.Fatal(err)
	}
	return v, it
}

// fmt imports slices, from source, but never runs it: a program that
// only imports fmt doesn't build it.
func TestNativeImportsNotBuilt(t *testing.T) {
	v, it := runF(t, ixgo.NewContext(0), `package main

import "fmt"

func F() string { return fmt.Sprint([]int{3, 1, 2}) }
`)
	if v != "[3 1 2]" {
		t.Fatalf("F() = %v", v)
	}
	if p := it.MainPkg().Prog.ImportedPackage("slices"); p != nil {
		t.Fatal("slices built for a program that only imports fmt")
	}
}

// A package a native one's imports reach is built once source imports
// it, whichever comes first.
func TestSourceImportAfterNative(t *testing.T) {
	c, err := ixgo.NewTypesCache(0)
	if err != nil {
		t.Fatal(err)
	}
	for name, ctx := range map[string]*ixgo.Context{"fresh": ixgo.NewContext(0), "cached": c.NewContext()} {
		v, _ := runF(t, ctx, `package main

import (
	"fmt"
	"slices"
	"sort"
)

func F() string {
	a := []int{3, 1, 2}
	slices.Sort(a)
	b := []string{"b", "a"}
	sort.Strings(b)
	return fmt.Sprint(a, b, slices.Index(a, 2))
}
`)
		if v != "[1 2 3] [a b] 1" {
			t.Errorf("%s: F() = %v", name, v)
		}
	}
}
