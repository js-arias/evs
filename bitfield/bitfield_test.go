// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package bitfield

import "testing"

func TestBitCount(t *testing.T) {
	mp := map[int]int{
		0:     0,
		1:     1,
		67:    3,
		101:   4,
		38729: 8,
	}
	for i, v := range mp {
		if bitCount[i] != v {
			t.Errorf("bitCount error: expecting %d, found %d", v, bitCount[i])
		}
	}
}

func TestBitfieldOps(t *testing.T) {
	f := make(Bitfield, 2)
	b := Bitfield{
		101,
		73,
	}

	bt := []int{
		31,
		28,
		26,
		25,
		24,
		22,
		19,
		16,
	}
	for _, i := range bt {
		f.PutOn(i)
	}
	if f[1] != 38729 {
		t.Errorf("PutOn error: expecting %d, found %d", 38729, f[1])
	}
	for _, i := range bt {
		if !f.IsOn(i) {
			t.Errorf("IsOn error: expecting %d, found %d", 38729, f[1])
		}
	}
	if f.Count() != len(bt) {
		t.Errorf("Count error: expecting %d, found %d", len(bt), f.Count)
	}
	if f.Common(b) != 3 {
		t.Errorf("Common error: expecting %d, found %d", 3, f.Common(b))
	}
	f.Union(b)
	if f[0] != b[0] {
		t.Errorf("Union error: expecting %d, found %d", b[0], f[0])
	}
	if f[1] != 38729 {
		t.Errorf("Union error: expecting %d, found %d", 38729, f[1])
	}
	f.Reset()
	for _, x := range f {
		if x != 0 {
			t.Errorf("Union error: expecting %d, found %d", 0, x)
		}
	}
}
