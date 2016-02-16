// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package bitfield

// A Bitfield is a field of bits.
type Bitfield []uint16

// BitsPerField is the number of bits in each field.
const BitsPerField = 16

// bitCount stores the amount of bits in a singe field element.
var bitCount [65536]int

func init() {
	// set bitCount
	for i := range bitCount {
		c := 0
		for j := uint(0); j < BitsPerField; j++ {
			bit := 1 << j
			if (i & bit) != 0 {
				c++
			}
		}
		bitCount[i] = c
	}
}

// IsOn returns true if the indicated bit is on in the bitfield.
func (f Bitfield) IsOn(b int) bool {
	i := b / BitsPerField
	s := uint16(b) % BitsPerField
	if (f[i] & (1 << s)) == 0 {
		return false
	}
	return true
}

// PutOn sets a bit as on.
func (f Bitfield) PutOn(b int) {
	i := b / BitsPerField
	s := uint16(b) % BitsPerField
	f[i] |= 1 << s
}

// Union adds the content of bitfield b into f.
func (f Bitfield) Union(b Bitfield) {
	for i, x := range b {
		f[i] |= x
	}
}

// Common returns the number of common on bits between f and b.
func (f Bitfield) Common(b Bitfield) int {
	c := 0
	for i, x := range b {
		j := f[i] & x
		c += bitCount[j]
	}
	return c
}

// Count returns the number of on bits in a bitfield.
func (f Bitfield) Count() int {
	c := 0
	for _, x := range f {
		c += bitCount[x]
	}
	return c
}

// Reset put in zero (off) all bits of f.
func (f Bitfield) Reset() {
	for i := range f {
		f[i] = 0
	}
}
