package ixgo_test

import (
	"testing"

	"github.com/goplus/ixgo"
	_ "github.com/goplus/ixgo/pkg/strings"
	_ "github.com/goplus/ixgo/pkg/unicode"
)

// Interpreted functions native code calls back with the typed callbacks'
// signatures (runtime_func.go): sort's less and search, strings' per-rune
// functions; a panic in one reaches the program, and one handed back to
// interpreted code calls as itself.
func TestTypedCallbackShapes(t *testing.T) {
	_, err := ixgo.NewContext(0).RunFile("main.go", `package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func check(what string, got, want any) {
	if fmt.Sprint(got) != fmt.Sprint(want) {
		panic(fmt.Sprintf("%s = %v, want %v", what, got, want))
	}
}

func main() {
	words := []string{"pear", "fig", "apple", "kiwi", "banana"}
	sort.Slice(words, func(i, j int) bool { return len(words[i]) < len(words[j]) || len(words[i]) == len(words[j]) && words[i] < words[j] })
	check("sort.Slice", words, []string{"fig", "kiwi", "pear", "apple", "banana"})

	nums := []int{1, 3, 5, 700, 900}
	check("sort.Search", sort.Search(len(nums), func(i int) bool { return nums[i] >= 600 }), 3)

	check("FieldsFunc", strings.FieldsFunc("a1b22c", unicode.IsDigit), []string{"a", "b", "c"})
	check("FieldsFunc closure", strings.FieldsFunc("a,b;c", func(r rune) bool { return r == ',' || r == ';' }), []string{"a", "b", "c"})
	check("Map", strings.Map(func(r rune) rune {
		if r == 'a' {
			return -1
		}
		return unicode.ToUpper(r)
	}, "banana"), "BNN")

	msg := func() (msg any) {
		defer func() { msg = recover() }()
		sort.Slice(nums, func(i, j int) bool { panic("in less") })
		return nil
	}()
	check("panic in less", msg, "in less")

	var less func(i, j int) bool = func(i, j int) bool { return i > j }
	keep := []func(int, int) bool{less}
	check("called back", keep[0](2, 1), true)
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
}
