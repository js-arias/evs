// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/raster"
)

var rBay = &cmdapp.Command{
	Run:       rBayRun,
	UsageLine: `r.bay [-c|--columns number] <name>`,
	Short:     "export to BayArea",
	Long: `
R.bay export the current dataset to files called <name>.areas.txt that
contains a matrix present-absent matrix of taxon vs pixel, and <name>.geo.txt
that contains the geographic locations of the center of each pixel.

Options are:

    -c number
    --column number
      Set the number of columns in the raster. Default = 360 (i.e. a pixel
      grid with 1x1 degrees.

    <name>
      Name used to set output files.
	`,
}

func init() {
	rBay.Flag.IntVar(&numCols, "columns", 360, "")
	rBay.Flag.IntVar(&numCols, "c", 360, "")
}

func rBayRun(c *cmdapp.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: expecting output file\n", c.Name())
		os.Exit(1)
	}
	d, err := loadData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	ras := raster.Rasterize(d, numCols, 0)

	geo, err := os.Create(args[0] + ".geo.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	defer geo.Close()
	fmt.Fprintf(geo, "# 0.0\n")
	var pxls []int
	for y := 0; y < ((numCols / 2) + 1); y++ {
		lat := 90 - ((float64(y) * ras.Resol) + (ras.Resol / 2))
		for x := 0; x < numCols; x++ {
			px := (y * numCols) + x
			if _, ok := ras.Pixel[px]; ok {
				pxls = append(pxls, px)
				lon := ((float64(x) * ras.Resol) + (ras.Resol / 2)) - 180
				fmt.Fprintf(geo, "%.4f %.4f\n", lat, lon)
			}
		}
	}

	areas, err := os.Create(args[0] + ".areas.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	defer areas.Close()
	fmt.Fprintf(areas, "%d %d\n", len(ras.Names), len(pxls))
	for _, tx := range ras.Names {
		fmt.Fprintf(areas, "%s\t", tx.Name)
		for _, v := range pxls {
			px := ras.Pixel[v]
			if tx.Obs.IsOn(px) {
				fmt.Fprintf(areas, "1")
			} else {
				fmt.Fprintf(areas, "0")
			}
		}
		fmt.Fprintf(areas, "\n")
	}
}
