/*
 * Copyright (c) 2026 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ixgo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goplus/ixgo"
)

// Recursion that never ends panics past the context's call depth, with a
// runtime error naming the bound and a stack naming the call's line,
// whatever recurs: a function, a closure, a method, two functions calling
// each other, one with defers, or a function native code calls back.
func TestMaxCallDepth(t *testing.T) {
	for name, tc := range map[string]struct {
		src  string
		line string
	}{
		"self": {`func down(n int) int { return down(n+1) + 1 }

func Run() { down(0) }`, "main.go:3"},
		"closure": {`func Run() {
	var f func(int) int
	f = func(n int) int { return f(n+1) + 1 }
	f(0)
}`, "main.go:5"},
		"method": {`type T struct{}

func (t T) down(n int) int { return t.down(n+1) + 1 }

func Run() { T{}.down(0) }`, "main.go:5"},
		"mutual": {`func a(n int) int { return b(n+1) + 1 }

func b(n int) int { return a(n+1) + 1 }

func Run() { a(0) }`, "main.go:"},
		"defer": {`func down(n int) int {
	defer func() {}()
	return down(n+1) + 1
}

func Run() { down(0) }`, "main.go:5"},
		"callback": {`import "strings"

func down(s string) string { return strings.Map(func(r rune) rune { down(s); return r }, s) }

func Run() { down("a") }`, "main.go:5"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ixgo.NewContext(0)
			ctx.SetMaxCallDepth(100)
			interp, err := ctx.LoadInterp("main.go", "package main\n\n"+tc.src+"\n")
			if err != nil {
				t.Fatal(err)
			}
			_, err = interp.RunFunc("Run")
			var p ixgo.PanicError
			if !errors.As(err, &p) || !strings.Contains(err.Error(), "recursion past 100 calls deep") {
				t.Fatalf("got %v, want a panic: recursion past 100 calls deep", err)
			}
			if stack := string(p.Stack()); !strings.Contains(stack, tc.line) {
				t.Errorf("stack names no %s:\n%s", tc.line, stack)
			}
		})
	}
}

// A goroutine's calls nest as deep as the bound, its entry counting as one,
// and a call one deeper panics; each goroutine starts afresh.
func TestMaxCallDepthEdge(t *testing.T) {
	const src = `package main

func depth(n int) int {
	if n == 0 {
		return 0
	}
	return depth(n-1) + 1
}

func Run(n int) int { return depth(n) }

func Twice(n int) int {
	done := make(chan int)
	go func() { done <- depth(n) }()
	return depth(n) + <-done
}
`
	ctx := ixgo.NewContext(0)
	ctx.SetMaxCallDepth(100)
	interp, err := ctx.LoadInterp("main.go", src)
	if err != nil {
		t.Fatal(err)
	}
	// Run is 1 deep, depth(98) 2, and depth(0) 100.
	if v, err := interp.RunFunc("Run", 98); err != nil || v != 98 {
		t.Errorf("Run(98) = %v, %v; want 98 within the bound", v, err)
	}
	if _, err := interp.RunFunc("Run", 99); err == nil || !strings.Contains(err.Error(), "recursion past 100") {
		t.Errorf("Run(99) = %v; want recursion past 100", err)
	}
	if v, err := interp.RunFunc("Twice", 90); err != nil || v != 180 {
		t.Errorf("Twice(90) = %v, %v; want 180, each goroutine 90 deep", v, err)
	}
	// Less than 0 is no bound, as 0 is.
	ctx = ixgo.NewContext(0)
	ctx.SetMaxCallDepth(-1)
	if interp, err = ctx.LoadInterp("main.go", src); err != nil {
		t.Fatal(err)
	}
	if v, err := interp.RunFunc("Run", 500); err != nil || v != 500 {
		t.Errorf("unbounded Run(500) = %v, %v; want 500", v, err)
	}
}

// Recursion past the bound in a goroutine ends the run with the goroutine's
// panic, naming its line, when the context has a RunContext.
func TestMaxCallDepthGoroutine(t *testing.T) {
	const src = `package main

func down(n int) int { return down(n+1) + 1 }

func main() {
	done := make(chan int)
	go func() { done <- down(0) }()
	<-done
}
`
	ctx := ixgo.NewContext(0)
	ctx.SetMaxCallDepth(100)
	ctx.RunContext = context.Background()
	_, err := ctx.RunFile("main.go", src, nil)
	var p ixgo.PanicError
	if !errors.As(err, &p) || !strings.Contains(err.Error(), "recursion past 100 calls deep") {
		t.Fatalf("got %v, want a panic: recursion past 100 calls deep", err)
	}
	if stack := string(p.Stack()); !strings.Contains(stack, "main.go:3") {
		t.Errorf("stack names no main.go:3:\n%s", stack)
	}
}

// Recursion through fmt meets the bound too, though fmt recovers the panic
// in a String method, as it does any, and prints it.
func TestMaxCallDepthString(t *testing.T) {
	const src = `package main

import "fmt"

type T struct{ n int }

func (t T) String() string { return fmt.Sprint(T{t.n + 1}) }

func Run() string { return T{}.String() }
`
	ctx := ixgo.NewContext(0)
	ctx.SetMaxCallDepth(100)
	interp, err := ctx.LoadInterp("main.go", src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := interp.RunFunc("Run")
	if s, _ := v.(string); err != nil || !strings.Contains(s, "recursion past 100 calls deep") {
		t.Errorf("Run() = %q, %v; want fmt's print of the panic past 100", v, err)
	}
}

// A runtime error in a goroutine, handed on through RunContext, names the
// line it happened on.
func TestGoroutineRuntimeErrorLine(t *testing.T) {
	const src = `package main

func main() {
	done := make(chan int)
	go func() {
		var counts map[string]int
		counts["x"]++
		done <- 1
	}()
	<-done
}
`
	ctx := ixgo.NewContext(0)
	ctx.RunContext = context.Background()
	_, err := ctx.RunFile("main.go", src, nil)
	var p ixgo.PanicError
	if !errors.As(err, &p) || !strings.Contains(err.Error(), "nil map") {
		t.Fatalf("got %v, want the goroutine's panic", err)
	}
	if stack := string(p.Stack()); !strings.Contains(stack, "main.go:7") {
		t.Errorf("stack names no main.go:7:\n%s", stack)
	}
}
