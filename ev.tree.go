// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/raster"
	"github.com/js-arias/evs/treesvg"
)

var evTree = &cmdapp.Command{
	Run:       evTreeRun,
	UsageLine: `ev.tree [-i|--input file] [--stepX number] [--stepY number]`,
	Short:     "exports a tree reconstruction",
	Long: `
Ev.tree exports a tree reconstruction into a svg file. In that file, black
filled squares represent nodes with vicariance, white squares full sympatry,
white circles punctual sympatry (the branch with the circle is the punctual
descendant), and white triangle founder event (the branch with the triangle is
the founder descendant).

Options are:

    -i file
    --input file
      Reads from an input file instead of standard input.

    --stepX number
    --stepY number
      Sets the separation between branches of the tree.
	`,
}

var (
	stepX int
	stepY int
)

func init() {
	evTree.Flag.StringVar(&inFile, "input", "", "")
	evTree.Flag.StringVar(&inFile, "i", "", "")
	evTree.Flag.IntVar(&stepX, "stepX", 0, "")
	evTree.Flag.IntVar(&stepY, "stepY", 0, "")
}

func evTreeRun(c *cmdapp.Command, args []string) {
	d, err := loadData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	r := raster.Rasterize(d, numCols, numFill)
	ts, err := loadTrees()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	f := os.Stdin
	if len(inFile) > 0 {
		f, err = os.Open(inFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		defer f.Close()
	}
	recs, err := events.Read(f, r, ts, szExtra)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	err = treesvg.SVG(ts, recs, stepX, stepY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
}
