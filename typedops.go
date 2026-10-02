package ixgo

import (
	"reflect"

	"golang.org/x/tools/go/ssa"
)

// Typed operations: loads through pointers, and map reads and writes, on
// the commonest unnamed types. A register of such a type holds exactly
// it, so a type assertion and Go's own load or map access take the place
// of reflect's, whose Interface copies each value it hands back. A word
// counter's map[string]int and a loop over a []string's elements spent
// most of their time and garbage there. Any other type, a named one
// included, goes through reflect as before.

// makeDerefFast is *x for instr, x's type typ, when typ is one of the
// pointer types it knows; else nil.
func makeDerefFast(instr *ssa.UnOp, typ reflect.Type, ir, ix register, kx kind, vx value) func(fr *frame) {
	switch typ {
	case reflect.TypeFor[*int]():
		return derefInt(instr, ir, ix, kx, vx)
	case reflect.TypeFor[*string]():
		return loadPtr[string](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*bool]():
		return loadPtr[bool](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*float64]():
		return loadPtr[float64](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*byte]():
		return loadPtr[byte](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*rune]():
		return loadPtr[rune](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*any]():
		return loadPtr[any](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*[]string]():
		return loadPtr[[]string](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*[]int]():
		return loadPtr[[]int](instr, ir, ix, kx, vx)
	case reflect.TypeFor[*[]byte]():
		return loadPtr[[]byte](instr, ir, ix, kx, vx)
	}
	return nil
}

func nilDeref(fr *frame, instr ssa.Instruction) {
	panic(fr.runtimeError(instr, "invalid memory address or nil pointer dereference"))
}

func loadPtr[T any](instr *ssa.UnOp, ir, ix register, kx kind, vx value) func(fr *frame) {
	if kx == kindGlobal {
		p := vx.(*T)
		return func(fr *frame) { fr.setReg(ir, *p) }
	}
	return func(fr *frame) {
		p, _ := fr.reg(ix).(*T)
		if p == nil {
			nilDeref(fr, instr)
		}
		fr.setReg(ir, *p)
	}
}

// derefInt is loadPtr[int], boxing as setInt does.
func derefInt(instr *ssa.UnOp, ir, ix register, kx kind, vx value) func(fr *frame) {
	if kx == kindGlobal {
		p := vx.(*int)
		return func(fr *frame) { fr.setInt(ir, *p) }
	}
	return func(fr *frame) {
		p, _ := fr.reg(ix).(*int)
		if p == nil {
			nilDeref(fr, instr)
		}
		fr.setInt(ir, *p)
	}
}

// makeLookupFast is m[k] for instr, m's type typ, when typ is one of the
// map types it knows; else nil.
func makeLookupFast(instr *ssa.Lookup, typ reflect.Type, ir, ix, ii register) func(fr *frame) {
	switch typ {
	case reflect.TypeFor[map[string]int]():
		return lookupBoxed[string](instr, ir, ix, ii, (*frame).setInt)
	case reflect.TypeFor[map[string]string]():
		return lookup[string, string](instr, ir, ix, ii)
	case reflect.TypeFor[map[string]bool]():
		return lookup[string, bool](instr, ir, ix, ii)
	case reflect.TypeFor[map[string]float64]():
		return lookup[string, float64](instr, ir, ix, ii)
	case reflect.TypeFor[map[string]any]():
		return lookup[string, any](instr, ir, ix, ii)
	case reflect.TypeFor[map[string][]string]():
		return lookup[string, []string](instr, ir, ix, ii)
	case reflect.TypeFor[map[int]int]():
		return lookupBoxed[int](instr, ir, ix, ii, (*frame).setInt)
	case reflect.TypeFor[map[int]string]():
		return lookup[int, string](instr, ir, ix, ii)
	case reflect.TypeFor[map[int]bool]():
		return lookup[int, bool](instr, ir, ix, ii)
	}
	return nil
}

func lookup[K comparable, V any](instr *ssa.Lookup, ir, ix, ii register) func(fr *frame) {
	if instr.CommaOk {
		return func(fr *frame) {
			m, _ := fr.reg(ix).(map[K]V)
			v, ok := m[fr.reg(ii).(K)]
			fr.setReg(ir, tuple{v, ok})
		}
	}
	return func(fr *frame) {
		m, _ := fr.reg(ix).(map[K]V)
		fr.setReg(ir, m[fr.reg(ii).(K)])
	}
}

// lookupBoxed is lookup for a value set boxes (setInt).
func lookupBoxed[K comparable, V any](instr *ssa.Lookup, ir, ix, ii register, set func(*frame, register, V)) func(fr *frame) {
	if instr.CommaOk {
		return lookup[K, V](instr, ir, ix, ii) // a tuple's anyway
	}
	return func(fr *frame) {
		m, _ := fr.reg(ix).(map[K]V)
		set(fr, ir, m[fr.reg(ii).(K)])
	}
}

// makeMapUpdateFast is m[k] = v for instr, m's type typ, when typ is one
// of the map types makeLookupFast knows; else nil.
func makeMapUpdateFast(typ reflect.Type, im, ik, iv register, kv kind, vv value) func(fr *frame) {
	switch typ {
	case reflect.TypeFor[map[string]int]():
		return mapUpdate[string, int](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[string]string]():
		return mapUpdate[string, string](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[string]bool]():
		return mapUpdate[string, bool](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[string]float64]():
		return mapUpdate[string, float64](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[string]any]():
		return mapUpdate[string, any](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[string][]string]():
		return mapUpdate[string, []string](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[int]int]():
		return mapUpdate[int, int](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[int]string]():
		return mapUpdate[int, string](im, ik, iv, kv, vv)
	case reflect.TypeFor[map[int]bool]():
		return mapUpdate[int, bool](im, ik, iv, kv, vv)
	}
	return nil
}

func mapUpdate[K comparable, V any](im, ik, iv register, kv kind, vv value) func(fr *frame) {
	if kv.isStatic() {
		v, _ := vv.(V)
		return func(fr *frame) {
			m, _ := fr.reg(im).(map[K]V)
			m[fr.reg(ik).(K)] = v
		}
	}
	return func(fr *frame) {
		m, _ := fr.reg(im).(map[K]V)
		v, _ := fr.reg(iv).(V)
		m[fr.reg(ik).(K)] = v
	}
}

// valueOf is v as a reflect.Value of typ: a nil interface is typ's zero,
// not reflect's invalid Value, which SetMapIndex takes for a delete.
func valueOf(typ reflect.Type, v value) reflect.Value {
	if v == nil {
		return reflect.Zero(typ)
	}
	return reflect.ValueOf(v)
}
