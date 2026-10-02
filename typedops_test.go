package ixgo_test

import (
	"testing"

	"github.com/goplus/ixgo"
)

// Typed operations (typedops.go) behave as the reflect ones they stand in
// for: zero values, comma-ok, nil maps and pointers, globals, and a named
// type beside an unnamed one.
func TestTypedOps(t *testing.T) {
	ctx := ixgo.NewContext(0)
	_, err := ctx.RunFile("main.go", `package main

import (
	"fmt"
	"strings"
)

type M map[string]int

var (
	gs  = "global"
	gi  = 300
	gss = []string{"a", "b"}
)

func panics(f func()) (msg string) {
	defer func() { msg = fmt.Sprint(recover()) }()
	f()
	return
}

func check(what string, got, want any) {
	if fmt.Sprint(got) != fmt.Sprint(want) {
		panic(fmt.Sprintf("%s = %v, want %v", what, got, want))
	}
}

func main() {
	count := map[string]int{}
	for _, w := range strings.Fields("a b a c a b") {
		count[w]++
	}
	check("count", count, map[string]int{"a": 3, "b": 2, "c": 1})
	v, ok := count["z"]
	check("count[z]", []any{v, ok}, []any{0, false})
	v, ok = count["a"]
	check("count[a]", []any{v, ok}, []any{3, true})

	named := M{}
	named["x"] += 1000
	check("named", named["x"], 1000)

	var nilMap map[string]string
	check("nil map read", nilMap["k"] == "", true)
	check("nil map write", strings.Contains(panics(func() { nilMap["k"] = "v" }), "assignment to entry in nil map"), true)

	anys := map[string]any{}
	anys["k"] = nil
	av, aok := anys["k"]
	check("anys", []any{len(anys), av == nil, aok}, []any{1, true, true})

	// Through reflect, as before, and without deleting the key.
	errs := map[int]error{}
	errs[1] = nil
	keys := map[any]int{}
	keys[nil] = 1
	keys[nil]++
	check("nil values and keys", []any{len(errs), len(keys), keys[nil]}, []any{1, 1, 2})

	ints := map[int]int{1: 1000}
	ints[2] = ints[1] * 3
	check("ints", ints[2], 3000)
	strs := map[int]string{}
	strs[7] = "seven"
	check("strs", strs[7]+strs[8], "seven")

	words := []string{"x", "yy", "zzz"}
	n := 0
	for i := range words {
		p := &words[i]
		n += len(*p)
	}
	check("words", n, 6)

	var np *int
	check("nil deref", strings.Contains(panics(func() { _ = *np }), "nil pointer dereference"), true)
	var ns *string
	check("nil string deref", strings.Contains(panics(func() { _ = *ns }), "nil pointer dereference"), true)

	gs += "!"
	gi *= 2
	gss = append(gss, "c")
	check("globals", []any{gs, gi, gss}, []any{"global!", 600, []string{"a", "b", "c"}})

	var e any = 1
	pe := &e
	*pe = nil
	check("*any", *pe == nil, true)
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
}
