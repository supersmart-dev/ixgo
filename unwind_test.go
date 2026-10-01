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
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"testing"

	"github.com/goplus/ixgo"
	_ "github.com/goplus/ixgo/pkg/runtime/debug"
	_ "github.com/goplus/ixgo/pkg/sync"
)

// unwindDefers are the defers of b, the frames a panic unwinds: none,
// an interpreted call, and only a native one.
var unwindDefers = map[string][2]string{
	"none":        {"_ = n", "_ = n"},
	"interpreted": {"_ = n", "defer func() {}()"},
	"native":      {"var mu sync.Mutex; mu.Lock()", "defer mu.Unlock()"},
}

// The stack a deferred call sees while a panic unwinds names, innermost
// first, the frames that panicked below it, then its own and those above,
// each once.
func TestUnwindStack(t *testing.T) {
	const src = `package main

import (
	"runtime/debug"
	"sync"
)

var (
	trace string
	_     sync.Mutex
)

func a(n int) {
	defer func() {
		if n == 3 {
			trace = string(debug.Stack())
		}
	}()
	if n == 0 {
		panic("bottom")
	}
	b(n - 1)
}

func b(n int) {
	%s
	%s
	c(n)
}

func c(n int) { a(n) }

func Run() (s string) {
	defer func() {
		recover()
		s = trace
	}()
	a(5)
	return
}
`
	// The deferred call (16), a(0) panicking (20), then c, b, a up to a(5),
	// and Run.
	want := []string{"16", "20"}
	for range 5 {
		want = append(want, "31", "28", "22")
	}
	want = append(want, "38")
	for name, defers := range unwindDefers {
		t.Run(name, func(t *testing.T) {
			interp, err := ixgo.NewContext(0).LoadInterp("main.go", fmt.Sprintf(src, defers[0], defers[1]))
			if err != nil {
				t.Fatal(err)
			}
			v, err := interp.RunFunc("Run")
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, m := range regexp.MustCompile(`main\.go:(\d+)`).FindAllStringSubmatch(v.(string), -1) {
				lines = append(lines, m[1])
			}
			if !reflect.DeepEqual(lines, want) {
				t.Errorf("stack lines %v, want %v:\n%s", lines, want, v)
			}
		})
	}
}

// Unwinding a panic costs memory in proportion to its depth: 4 times as
// deep allocates about 4 times as much, not 16 (each frame copying the
// record of those below) or more (native defers).
func TestUnwindLinear(t *testing.T) {
	const src = `package main

import "sync"

var _ sync.Mutex

func down(n int) {
	%s
	%s
	if n == 0 {
		panic("bottom")
	}
	down(n - 1)
}

func Run(n int) {
	defer func() { recover() }()
	down(n)
}
`
	for name, defers := range unwindDefers {
		t.Run(name, func(t *testing.T) {
			interp, err := ixgo.NewContext(0).LoadInterp("main.go", fmt.Sprintf(src, defers[0], defers[1]))
			if err != nil {
				t.Fatal(err)
			}
			alloc := func(n int) uint64 {
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				if _, err := interp.RunFunc("Run", n); err != nil {
					t.Fatal(err)
				}
				runtime.ReadMemStats(&after)
				return after.TotalAlloc - before.TotalAlloc
			}
			alloc(100)
			small, large := alloc(1000), alloc(4000)
			if large > 8*small {
				t.Errorf("4,000 deep allocated %d bytes, 1,000 deep %d: %.1f times, want about 4", large, small, float64(large)/float64(small))
			}
		})
	}
}
