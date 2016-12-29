// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/js-arias/evs/biogeo"
	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/tree"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// general flags
var (
	inFile  string // -i/--input
	outFile string // -o|--output
	verbose bool   // -v|--verbose
)

// raster flags
var (
	numCols int // -c|--columns
	numFill int // -f|--fill
)

func setRasterFlags(c *cmdapp.Command) {
	c.Flag.IntVar(&numCols, "columns", 360, "")
	c.Flag.IntVar(&numCols, "c", 360, "")
	c.Flag.IntVar(&numFill, "fill", 2, "")
	c.Flag.IntVar(&numFill, "f", 2, "")
}

func main() {
	cmdapp.Short = "Evs is a tool for phylogenetic biogeography."
	cmdapp.Commands = []*cmdapp.Command{
		evEval,
		evFlip,
		evMap,
		evTree,
		rBay,
		txLs,
		trIn,
		trLs,

		// help topics,
		about,
		recordsHelp,
		treesHelp,
	}
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	cmdapp.Run()
}

const (
	treeFileName = "trees.tab"
	dataFileName = "records.tab"
)

func loadData() (*biogeo.DataSet, error) {
	f, err := os.Open(dataFileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, err := biogeo.Read(f)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func loadTrees() ([]*tree.Tree, error) {
	f, err := os.Open(treeFileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ts, err := tree.Read(f)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func shuffle(v []int) {
	for i, x := range v {
		j := rand.Intn(len(v))
		v[i] = v[j]
		v[j] = x
	}
}
