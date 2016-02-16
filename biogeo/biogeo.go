// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package biogeo

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Earth boundaries
const (
	MinLon = -180
	MaxLon = 180
	MinLat = -90
	MaxLat = 90
)

// A GeoRef is a georeferenced record.
type GeoRef struct {
	Catalog  string
	Lon, Lat float64
}

// IsValid returns true if the GeoRef is a valid geographic point.
func (g GeoRef) IsValid() bool {
	if (g.Lon <= MinLon) || (g.Lon > MaxLon) {
		return false
	}
	if (g.Lat < MinLat) || (g.Lat > MaxLat) {
		return false
	}
	return true
}

// A Taxon is a named terminal taxon with a list of georeferenced records.
type Taxon struct {
	Name string
	Recs []GeoRef
}

// A DataSet is a biogeography data set.
type DataSet struct {
	Ls    []*Taxon          // list of taxons, as found
	Names map[string]*Taxon // a map of name (in lower caps) to taxon
}

// Taxon returns a taxon of a given name.
func (d *DataSet) Taxon(name string) *Taxon {
	for _, tx := range d.Ls {
		if strings.ToLower(name) == strings.ToLower(tx.Name) {
			return tx
		}
	}
	return nil
}

// Read reads data from an input stream in csv format.
func Read(in io.Reader) (*DataSet, error) {
	d := &DataSet{Names: make(map[string]*Taxon)}
	r := csv.NewReader(in)
	r.TrimLeadingSpace = true

	// reads the file header
	h, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("header (data): %v", err)
	}
	name := -1
	lon := -1
	lat := -1
	cat := -1
	for i, v := range h {
		switch strings.ToLower(v) {
		case "name", "scientificname", "scientific name":
			name = i
		case "lon", "longitude", "long":
			lon = i
		case "lat", "latitude":
			lat = i
		case "catalog", "recordid", "record id":
			cat = i
		}
	}
	if (name < 0) || (lon < 0) || (lat < 0) {
		return nil, errors.New("header (data): incomplete header")
	}

	// read the data
	for i := 1; ; i++ {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("(data) row %d: %v", i, err)
		}
		if lr := len(row); (lr <= name) || (lr <= lon) || (lr <= lat) {
			continue
		}
		nm := strings.Join(strings.Fields(row[name]), " ")
		t, ok := d.Names[strings.ToLower(nm)]
		if !ok {
			t = &Taxon{Name: nm}
			d.Names[strings.ToLower(nm)] = t
			d.Ls = append(d.Ls, t)
		}
		lgv, err := strconv.ParseFloat(row[lon], 64)
		if err != nil {
			return nil, fmt.Errorf("(data) row %d, col %d: %v", i, lon+1, err)
		}
		ltv, err := strconv.ParseFloat(row[lat], 64)
		if err != nil {
			return nil, fmt.Errorf("(data) row %d, col %d: %v", i, lat+1, err)
		}
		var cv string
		if (cat >= 0) && (cat < len(row)) {
			cv = row[cat]
		}
		g := GeoRef{
			Lon:     lgv,
			Lat:     ltv,
			Catalog: cv,
		}
		if !g.IsValid() {
			return nil, fmt.Errorf("(data) row %d: invalid georeference", i)
		}
		t.Recs = append(t.Recs, g)
	}
	return d, nil
}
