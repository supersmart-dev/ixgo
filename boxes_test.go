package ixgo_test

import (
	"testing"

	"github.com/goplus/ixgo"
)

// Values a frame boxes itself (boxes.go) are each their own, kept in
// interfaces, maps, and closures past the loop that made them, and in
// goroutines at once; every kind it boxes, at its edges.
func TestBoxes(t *testing.T) {
	ctx := ixgo.NewContext(0)
	_, err := ctx.RunFile("main.go", `package main

import (
	"fmt"
	"math"
	"sync"
)

func sum(n int) (kept []any, total int) {
	m := map[any]int{}
	var fs []func() int
	for i := 0; i < n; i++ {
		v := i*1000 + 7
		kept = append(kept, v)
		m[v] = i
		fs = append(fs, func() int { return v })
		total += v
	}
	for i, k := range kept {
		if k.(int) != i*1000+7 || m[k] != i || fs[i]() != k.(int) {
			panic(fmt.Sprint("box ", i, " changed: ", k, m[k], fs[i]()))
		}
	}
	return
}

func main() {
	var wg sync.WaitGroup
	totals := make([]int, 8)
	for g := range totals {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, totals[g] = sum(1000 + g)
		}()
	}
	wg.Wait()
	for g, got := range totals {
		n := 1000 + g
		if want := 1000*n*(n-1)/2 + 7*n; got != want {
			panic(fmt.Sprint("goroutine ", g, ": ", got, " != ", want))
		}
	}

	var i64 int64 = math.MaxInt64
	i64 = i64 + 0
	var mi64 int64 = math.MinInt64
	mi64 = mi64 - 0
	var u uint = math.MaxUint
	u = u | 0
	var u64 uint64 = math.MaxUint64
	u64 = u64 ^ 0
	var up uintptr = 1 << 40
	up = up + 1
	f, nz := 1.5, math.Copysign(0, -1)
	f = f * 3
	nz = nz * 1
	nan := math.NaN()
	nan = nan + 1
	inf := math.Inf(-1)
	inf = inf - 1
	n := -1
	n = n * 300
	got := fmt.Sprint(i64, mi64, u, u64, up, f, math.Signbit(nz), nz, nan, inf, n)
	want := "9223372036854775807 -9223372036854775808 18446744073709551615 18446744073709551615 1099511627777 4.5 true -0 NaN -Inf -300"
	if got != want {
		panic(got + " != " + want)
	}
}
`, nil)
	if err != nil {
		t.Fatal(err)
	}
}
