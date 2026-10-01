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
