package ixgo

import "unsafe"

// Registers hold values as interfaces, so storing an int-sized result
// boxes it: the runtime allocates eight bytes for each value past 255, two
// or three times per turn of a loop counting past it. A goroutine's boxes
// come instead from chunks of its own, one allocation per boxChunk values.
// A box is written once and only read after, so registers and anything
// else may share it as they share the runtime's. A chunk lives while any
// box in it does: at worst boxChunk times the memory of the boxes kept.

// boxChunk is the most values a chunk boxes. A goroutine's first chunk
// boxes firstChunk, each after twice the one before: a native call back
// into the interpreter (a sort's less) starts a goroutine's frames afresh,
// and most box a value or two.
const (
	firstChunk = 4
	boxChunk   = 128
)

// boxes is a goroutine's chunk of boxes; allocFrame hands a caller's to
// its callee, so each goroutine's frames share one.
type boxes struct {
	free  []uint64
	nfree int // the last chunk's size
}

// grow is the size of a chunk after one of n, at most max.
func grow(n, max int) int {
	if n == 0 {
		return firstChunk
	}
	return min(2*n, max)
}

// eface is an empty interface's layout.
type eface struct {
	typ, data unsafe.Pointer
}

// typeWord is the type word of an interface holding v.
func typeWord(v any) unsafe.Pointer { return (*eface)(unsafe.Pointer(&v)).typ }

var (
	intWord     = typeWord(int(0))
	int64Word   = typeWord(int64(0))
	uintWord    = typeWord(uint(0))
	uint64Word  = typeWord(uint64(0))
	uintptrWord = typeWord(uintptr(0))
	float64Word = typeWord(float64(0))
)

// box is bits in a box of b's, typed by typ, the type word of an 8-byte
// type without pointers.
func (b *boxes) box(typ unsafe.Pointer, bits uint64) value {
	if len(b.free) == 0 {
		b.nfree = grow(b.nfree, boxChunk)
		b.free = make([]uint64, b.nfree)
	}
	p := &b.free[0]
	b.free = b.free[1:]
	*p = bits
	var v value
	*(*eface)(unsafe.Pointer(&v)) = eface{typ, unsafe.Pointer(p)}
	return v
}

// The set functions store v in register ir as setReg does. Values under
// 256 the runtime boxes without allocating; so does a frame with no boxes
// of its own. Strings aren't boxed here: a chunk of them would keep each
// one's bytes alive as long as any other box in it, which a program can
// see (a finalizer that doesn't run) and pay for (output dropped but
// kept).

func (fr *frame) setInt(ir register, v int) {
	if uint(v) < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(intWord, uint64(v))
}

func (fr *frame) setInt64(ir register, v int64) {
	if uint64(v) < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(int64Word, uint64(v))
}

func (fr *frame) setUint(ir register, v uint) {
	if v < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(uintWord, uint64(v))
}

func (fr *frame) setUint64(ir register, v uint64) {
	if v < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(uint64Word, v)
}

func (fr *frame) setUintptr(ir register, v uintptr) {
	if v < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(uintptrWord, uint64(v))
}

func (fr *frame) setFloat64(ir register, v float64) {
	bits := *(*uint64)(unsafe.Pointer(&v))
	if bits < 256 || fr.boxes == nil {
		fr.stack[ir] = v
		return
	}
	fr.stack[ir] = fr.boxes.box(float64Word, bits)
}
