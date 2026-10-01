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
	"reflect"
	"testing"

	"github.com/goplus/ixgo"
)

// A panic unwinding a recursion runs each frame's defers, the frames a
// function's call of itself made, directly or through another, included.
// The call comes before the defer's recover block, so it's compiled before
// the function is known to recover.
func TestRecursionRunsDefers(t *testing.T) {
	for name, src := range map[string]string{
		"self": `func down(n int, log *[]int) {
	if n > 0 {
		defer func() { *log = append(*log, n) }()
		down(n-1, log)
		return
	}
	panic("bottom")
}`,
		"mutual": `func down(n int, log *[]int) {
	if n > 0 {
		defer func() { *log = append(*log, n) }()
		up(n-1, log)
		return
	}
	panic("bottom")
}

func up(n int, log *[]int) {
	if n > 0 {
		defer func() { *log = append(*log, n) }()
		down(n-1, log)
		return
	}
	panic("bottom")
}`,
	} {
		t.Run(name, func(t *testing.T) {
			interp, err := ixgo.NewContext(0).LoadInterp("main.go", "package main\n\n"+src+`

func Run() (log []int) {
	defer func() { recover() }()
	down(3, &log)
	return
}
`)
			if err != nil {
				t.Fatal(err)
			}
			got, err := interp.RunFunc("Run")
			if want := []int{1, 2, 3}; err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("deferred = %v, %v; want %v", got, err, want)
			}
		})
	}
}
