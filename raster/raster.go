// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package raster

import (
	"strings"

	"github.com/js-arias/evs/biogeo"
	"github.com/js-arias/evs/bitfield"
)

// A Taxon is a named terminal taxon with a defined rasterized data.
type Taxon struct {
	Name string
	// rasterized data
	Obs  bitfield.Bitfield
	Fill bitfield.Bitfield
}

// A Raster is a rasterized data set.
type Raster struct {
	Names  map[string]*Taxon // a map of name (in lower caps) to taxon
	Fields int               // number of fields in the raster bitfield
	Pixel  map[int]int       // map of pixel:bit
	Cols   int               // number of columns
	Fill   int               // fill of the raster
	Resol  float64           // resolution of the raster
}

// Rasterize creates a new raster from a given dataset.
func Rasterize(d *biogeo.DataSet, cols, fill int) *Raster {
	ras := &Raster{
		Names: make(map[string]*Taxon),
		Pixel: make(map[int]int),
		Cols:  cols,
		Fill:  fill,
		Resol: 360 / float64(cols),
	}
	cells := 0
	for _, t := range d.Ls {
		for _, g := range t.Recs {
			c := int((180 + g.Lon) / ras.Resol)
			r := int((90 - g.Lat) / ras.Resol)
			px := (r * ras.Cols) + c
			if _, ok := ras.Pixel[px]; ok {
				continue
			}
			ras.Pixel[px] = cells
			cells++
		}
	}
	ras.Fields = cells / bitfield.BitsPerField
	if (ras.Fields * bitfield.BitsPerField) != cells {
		ras.Fields++
	}
	tc := make(chan *Taxon)
	for _, t := range d.Ls {
		go ras.rasterize(t, tc)
	}
	for _ = range d.Ls {
		t := <-tc
		ras.Names[strings.ToLower(t.Name)] = t
	}
	return ras
}

// rasterize creates the raster of a given taxon.
func (ras *Raster) rasterize(tx *biogeo.Taxon, tc chan *Taxon) {
	t := &Taxon{
		Name: tx.Name,
		Obs:  make(bitfield.Bitfield, ras.Fields),
		Fill: make(bitfield.Bitfield, ras.Fields),
	}
	for _, g := range tx.Recs {
		c := int((180 + g.Lon) / ras.Resol)
		r := int((90 - g.Lat) / ras.Resol)
		px := (r * ras.Cols) + c
		b := ras.Pixel[px]
		if t.Obs.IsOn(b) {
			continue
		}
		t.Obs.PutOn(b)
		for i := -ras.Fill; i <= ras.Fill; i++ {
			x := c - i
			if x < 0 {
				x += ras.Cols
			}
			if x >= ras.Cols {
				x -= ras.Cols
			}
			for j := -ras.Fill; j <= ras.Fill; j++ {
				y := r - j
				if y < 0 {
					continue
				}
				fp := (y * ras.Cols) + x
				fb, ok := ras.Pixel[fp]
				if !ok {
					continue
				}
				t.Fill.PutOn(fb)
			}
		}
	}
	tc <- t
}

// Taxon returns a taxon for a given name.
func (r *Raster) Taxon(name string) *Taxon {
	return r.Names[strings.ToLower(name)]
}
