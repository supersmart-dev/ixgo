// Package opaquehidden is never registered with ixgo; tests reach it only
// through unexported fields of host structs.
package opaquehidden

type Secret struct {
	Key string
}

func (s *Secret) Reveal() string { return s.Key }
