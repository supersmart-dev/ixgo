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
	"testing"
	"time"

	"github.com/goplus/ixgo"
)

// Abort stops an endless loop whichever way its function was called: from
// the interpreter's entry, by a direct call (a function without defers, a
// method, a closure), by a dynamic call of a func value, and by a function's
// call of itself, with or without defers, directly or through another.
func TestAbortStopsLoops(t *testing.T) {
	for name, src := range map[string]string{
		"entry": `func Run() { for {} }`,
		"direct": `func spin() { for {} }

func Run() { spin() }`,
		"result": `func spin() int {
	n := 0
	for {
		n++
	}
}

func Run() { _ = spin() }`,
		"results": `func spin() (int, int) { for {} }

func Run() { _, _ = spin() }`,
		"method": `type T struct{}

func (T) spin() { for {} }

func Run() { T{}.spin() }`,
		"closure": `func Run() {
	n := 0
	f := func() {
		for {
			n++
		}
	}
	f()
}`,
		"funcval": `var f func()

func Run() {
	f = func() { for {} }
	f()
}`,
		"self": `func down(n int) {
	if n == 0 {
		for {}
	}
	down(n - 1)
}

func Run() { down(100) }`,
		"selfdefer": `func down(n int) {
	if n > 0 {
		defer func() {}()
		down(n - 1)
		return
	}
	for {}
}

func Run() { down(100) }`,
		"mutual": `func a(n int) {
	if n == 0 {
		for {}
	}
	b(n - 1)
}

func b(n int) { a(n) }

func Run() { a(100) }`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ixgo.NewContext(0)
			interp, err := ctx.LoadInterp("main.go", "package main\n\n"+src+"\n")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := interp.RunFunc("Run")
				done <- err
			}()
			time.Sleep(50 * time.Millisecond)
			interp.Abort()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("aborted run: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Errorf("still running 2s after Abort")
			}
		})
	}
}

// What looking for an abort costs on the fast call path: calls (fib) and
// a loop's instructions in a function called directly.
func BenchmarkFastPathCalls(b *testing.B) {
	benchmarkRun(b, `func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func Run() int { return fib(20) }`)
}

func BenchmarkFastPathLoop(b *testing.B) {
	benchmarkRun(b, `func sum(n int) int {
	s := 0
	for i := 0; i < n; i++ {
		s += i
	}
	return s
}

func Run() int { return sum(100000) }`)
}

func benchmarkRun(b *testing.B, src string) {
	interp, err := ixgo.NewContext(0).LoadInterp("main.go", "package main\n\n"+src+"\n")
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := interp.RunFunc("Run"); err != nil {
			b.Fatal(err)
		}
	}
}
