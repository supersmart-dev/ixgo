// Package opaquehost holds host structs whose unexported fields lead into
// the unregistered package opaquehidden.
package opaquehost

import "github.com/goplus/ixgo/testdata/opaquehidden"

// Host hides a host pointer in an unexported, comparable field.
type Host struct {
	Name   string
	secret *opaquehidden.Secret
}

// Sized is 32 bytes on every platform; its layout is all unexported.
type Sized struct {
	n [4]int64
}

// Bag hides a non-comparable unexported field, so Bag isn't comparable.
type Bag struct {
	Name  string
	items []opaquehidden.Secret
}
