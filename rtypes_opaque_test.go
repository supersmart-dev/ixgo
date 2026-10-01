package ixgo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/ixgo/testdata/opaquehost"
)

const opaqueHiddenPath = "github.com/goplus/ixgo/testdata/opaquehidden"

func init() {
	RegisterPackage(&Package{
		Name: "opaquehost",
		Path: "github.com/goplus/ixgo/testdata/opaquehost",
		NamedTypes: map[string]reflect.Type{
			"Host":  reflect.TypeOf((*opaquehost.Host)(nil)).Elem(),
			"Bag":   reflect.TypeOf((*opaquehost.Bag)(nil)).Elem(),
			"Sized": reflect.TypeOf((*opaquehost.Sized)(nil)).Elem(),
		},
	})
}

func TestUnexportedFieldsConvertedByDefault(t *testing.T) {
	ctx := NewContext(0)
	_, err := ctx.RunFile("main.go", `package main

import (
	"unsafe"

	"github.com/goplus/ixgo/testdata/opaquehost"
)

func main() {
	var h opaquehost.Host
	println(h.Name)
	if n := unsafe.Sizeof(opaquehost.Sized{}); n != 32 {
		panic(n)
	}
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !loaderHasPackage(ctx.Loader, opaqueHiddenPath) {
		t.Fatalf("default mode no longer converts unexported fields: %s not reached", opaqueHiddenPath)
	}
}

func TestOpaqueUnexportedFieldsKeepsComparable(t *testing.T) {
	ctx := NewContext(OpaqueUnexportedFields)
	_, err := ctx.RunFile("main.go", `package main

import "github.com/goplus/ixgo/testdata/opaquehost"

func main() {
	a := opaquehost.Host{Name: "x"}
	b := opaquehost.Host{Name: "x"}
	if a != b {
		panic("equal hosts compare unequal")
	}
	m := map[opaquehost.Host]int{a: 1}
	if m[b] != 1 {
		panic("host lookup as map key failed")
	}
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpaqueUnexportedFieldsKeepsNotComparable(t *testing.T) {
	ctx := NewContext(OpaqueUnexportedFields)
	_, err := ctx.LoadFile("main.go", `package main

import "github.com/goplus/ixgo/testdata/opaquehost"

func main() {
	var a, b opaquehost.Bag
	println(a == b)
}
`)
	if err == nil {
		t.Fatal("comparing a struct with a non-comparable unexported field type-checked")
	}
	if !strings.Contains(err.Error(), "cannot compare") && !strings.Contains(err.Error(), "invalid operation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpaqueUnexportedFieldsSkipsHiddenPackage(t *testing.T) {
	ctx := NewContext(OpaqueUnexportedFields)
	_, err := ctx.LoadFile("main.go", `package main

import "github.com/goplus/ixgo/testdata/opaquehost"

func main() {
	var h opaquehost.Host
	println(h.Name)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if loaderHasPackage(ctx.Loader, opaqueHiddenPath) {
		t.Fatalf("type checker reached %s through an unexported field", opaqueHiddenPath)
	}
}

func loaderHasPackage(l Loader, path string) bool {
	for _, p := range l.Packages() {
		if p.Path() == path {
			return true
		}
	}
	return false
}
