//go:build !llgo
// +build !llgo

package ixgo

import (
	"path/filepath"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/visualfc/funcval"
)

const (
	Compiler = runtime.Compiler
	IsLLGo   = false
)

func init() {
	if funcval.IsSupport {
		RegisterExternal("(reflect.Value).Pointer", func(fr *frame, v reflect.Value) uintptr {
			if v.Kind() == reflect.Func {
				if c := fr.interp.getMakeFuncVal(v.Interface()); c != nil && c.interp == fr.interp {
					return uintptr(c.pfn.base)
				}
			}
			return v.Pointer()
		})
		RegisterExternal("(reflect.Value).UnsafePointer", func(fr *frame, v reflect.Value) unsafe.Pointer {
			if v.Kind() == reflect.Func {
				if c := fr.interp.getMakeFuncVal(v.Interface()); c != nil && c.interp == fr.interp {
					return unsafe.Pointer(uintptr(c.pfn.base))
				}
			}
			return v.UnsafePointer()
		})
	}
}

//go:linkname runtimePanic runtime.gopanic
func runtimePanic(e interface{})

type funcinl struct {
	ones  uint32  // set to ^0 to distinguish from _func
	entry uintptr // entry of the real (the "outermost") frame
	name  string
	file  string
	line  int
}

func inlineFunc(entry uintptr) *funcinl {
	return &funcinl{ones: ^uint32(0), entry: entry}
}

func isInlineFunc(f *runtime.Func) bool {
	return (*funcinl)(unsafe.Pointer(f)).ones == ^uint32(0)
}

func runtimeFuncFileLine(fr *frame, f *runtime.Func, pc uintptr) (file string, line int) {
	entry := f.Entry()
	if isInlineFunc(f) && pc > entry {
		interp := fr.interp
		if pfn := findFuncByEntry(interp, int(entry)); pfn != nil {
			// pc-1 : fn.instr.pos
			pos := pfn.PosForPC(int(pc - entry - 1))
			if !pos.IsValid() {
				return "?", 0
			}
			fpos := interp.ctx.FileSet.Position(pos)
			if fpos.Filename == "" {
				return "??", fpos.Line
			}
			file, line = filepath.ToSlash(fpos.Filename), fpos.Line
			return
		}
	}
	return f.FileLine(pc)
}

const supportFuncVal = funcval.IsSupport

func dynamicFunCall(interp *Interp, iv register, ir register, ia []register) func(fr *frame) {
	return func(fr *frame) {
		fn := fr.reg(iv)
		if c := interp.getMakeFuncVal(fn); c != nil && c.interp == interp {
			if c.pfn.Recover == nil {
				interp.callFunctionByStackNoRecoverWithEnv(fr, c.pfn, ir, ia, c.env)
			} else {
				interp.callFunctionByStackWithEnv(fr, c.pfn, ir, ia, c.env)
			}
			return
		}
		v := reflect.ValueOf(fn)
		interp.callExternalByStack(fr, v, ir, ia)
	}
}

func (pfn *function) makeFunction(typ reflect.Type, env []value) reflect.Value {
	c := makeFuncVal{interp: pfn.Interp, pfn: pfn, typ: typ, env: env}
	if supportFuncVal {
		switch typ {
		case callbackVoidType:
			return reflect.ValueOf(c.callVoid)
		case callbackBoolType:
			return reflect.ValueOf(c.callBool)
		case callbackLessType:
			return reflect.ValueOf(c.callLess)
		case callbackIntBoolType:
			return reflect.ValueOf(c.callIntBool)
		case callbackRuneBoolType:
			return reflect.ValueOf(c.callRuneBool)
		case callbackRuneRuneType:
			return reflect.ValueOf(c.callRuneRune)
		}
	}
	return reflect.MakeFunc(typ, c.callReflect)
}

type makeFuncVal struct {
	interp *Interp
	pfn    *function
	typ    reflect.Type
	env    []value
}

var (
	callbackVoidType  = reflect.TypeFor[func()]()
	callbackBoolType  = reflect.TypeFor[func() bool]()
	callbackVoidPC    = reflect.ValueOf(makeFuncVal{}.callVoid).Pointer()
	callbackBoolPC    = reflect.ValueOf(makeFuncVal{}.callBool).Pointer()
	callbackReflectPC = reflect.ValueOf(makeFuncVal{}.callReflect).Pointer()

	// The commonest callbacks native code makes besides: sort.Slice's
	// less, sort.Search's, and strings' per-rune functions (FieldsFunc,
	// IndexFunc, TrimFunc, Map). Through reflect each call built slices
	// of reflect.Values for its arguments and results.
	callbackLessType     = reflect.TypeFor[func(int, int) bool]()
	callbackIntBoolType  = reflect.TypeFor[func(int) bool]()
	callbackRuneBoolType = reflect.TypeFor[func(rune) bool]()
	callbackRuneRuneType = reflect.TypeFor[func(rune) rune]()
	callbackTypedPCs     = map[uintptr]bool{
		callbackVoidPC: true,
		callbackBoolPC: true,
		reflect.ValueOf(makeFuncVal{}.callLess).Pointer():     true,
		reflect.ValueOf(makeFuncVal{}.callIntBool).Pointer():  true,
		reflect.ValueOf(makeFuncVal{}.callRuneBool).Pointer(): true,
		reflect.ValueOf(makeFuncVal{}.callRuneRune).Pointer(): true,
	}
)

func (c makeFuncVal) callVoid() {
	caller := c.interp.tryDeferFrame()
	if c.interp.maxDepth != 0 {
		defer c.interp.leaveCallback(c.interp.enterCallback(caller))
	}
	c.interp.callFunctionDiscardsResult(caller, c.pfn, nil, c.env)
}

func (c makeFuncVal) callBool() bool {
	caller := c.interp.tryDeferFrame()
	if c.interp.maxDepth != 0 {
		defer c.interp.leaveCallback(c.interp.enterCallback(caller))
	}
	result := c.interp.callFunction(caller, c.pfn, nil, c.env)
	return result != nil && result.(bool)
}

// call calls c with args, its single result, nil if an abort stopped it.
func (c makeFuncVal) call(args ...value) value {
	caller := c.interp.tryDeferFrame()
	if c.interp.maxDepth != 0 {
		defer c.interp.leaveCallback(c.interp.enterCallback(caller))
	}
	return c.interp.callFunction(caller, c.pfn, args, c.env)
}

func (c makeFuncVal) callLess(i, j int) bool {
	r, _ := c.call(i, j).(bool)
	return r
}

func (c makeFuncVal) callIntBool(i int) bool {
	r, _ := c.call(i).(bool)
	return r
}

func (c makeFuncVal) callRuneBool(r rune) bool {
	ok, _ := c.call(r).(bool)
	return ok
}

func (c makeFuncVal) callRuneRune(r rune) rune {
	m, _ := c.call(r).(rune)
	return m
}

func (c makeFuncVal) callReflect(args []reflect.Value) []reflect.Value {
	caller := c.interp.tryDeferFrame()
	if c.interp.maxDepth != 0 {
		defer c.interp.leaveCallback(c.interp.enterCallback(caller))
	}
	return c.interp.callFunctionByReflect(caller, c.pfn, c.typ, args, c.env)
}

type interpExt struct{}

// Callers must check interpreter ownership.
func (*interpExt) getMakeFuncVal(fn interface{}) *makeFuncVal {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func || v.IsNil() {
		return nil
	}
	fv, n := funcval.Get(fn)
	switch {
	case n == 0 && callbackTypedPCs[fv.Fn]:
		// Direct method value.
	case n == 1 && fv.Fn == callbackReflectPC:
		// One reflect.MakeFunc bridge.
	default:
		return nil
	}
	// gc ABI: FuncVal is one word, followed by the receiver.
	// The call methods must retain value receivers.
	return &(*struct {
		funcval.FuncVal
		receiver makeFuncVal
	})(unsafe.Pointer(fv)).receiver
}
