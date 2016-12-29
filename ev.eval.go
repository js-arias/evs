// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/raster"
)

var evEval = &cmdapp.Command{
	Run: evEvalRun,
	UsageLine: `ev.eval [-b|--brlen] [-c|--columns number] [-f|--fill number]
	[-i|--input file] [--found number] [--point number] [--symp number]
	[--vic number] [-z|--size number] [-sympSize number]`,
	Short: "evaluate four event reconstructions",
	Long: `
Ev.eval reads a reconstruction in tsv from the standard input, and for a given
data and tree, will print the cost of that reconstruction.

Options are:

    -b
    --brlen
      If set, branch lengths will be will be used to downweight pixel changes
      in a branch (i.e. cost = changes / len), so changes in long branches
      will be cheaper, and, if -z, --size is used, branch lenghts will be
      upweight the cost of the size (i.e. cost = (range-size / sizeParam) *
      len), so having a large size will be costly on longer branches.

    -c number
    --column number
      Set the number of columns in the raster. Default = 360 (i.e. a pixel
      grid with 1x1 degrees.

    -f number
    --fill number
      Set the number of pixels to fill around an observed pixel. Default = 2.

    -i file
    --input file
      Reads from an input file instead of standard input.

    -o file
    --output file
      Set the output file, instead of the standard output.

    --found number
    --point number
    --symp number
    --vic number
      Sets the cost of a given type of event. Event costs should be greather
      than 0. Default = 1.

    -z number
    --size number
      If set, the indicated the value of the ancestral_range_size / number
      will be used as extra-cost on internal nodes.

    --sympSize number
      If set, it will add to the cost of a sympatry event, the result of 
      ancestral_range_size / number.
	`,
}

func init() {
	setRasterFlags(evEval)
	setEventFlags(evEval)
	evEval.Flag.StringVar(&inFile, "input", "", "")
	evEval.Flag.StringVar(&inFile, "i", "", "")
	evEval.Flag.StringVar(&outFile, "output", "", "")
	evEval.Flag.StringVar(&outFile, "o", "", "")
}

func evEvalRun(c *cmdapp.Command, args []string) {
	if (VicCost <= 0) || (SympCost <= 0) || (PointCost <= 0) || (FoundCost <= 0) {
		fmt.Fprintf(os.Stderr, "%s: event costs should be greater than 0\n", c.Name())
		os.Exit(1)
	}
	o := os.Stdout
	if len(outFile) > 0 {
		var err error
		o, err = os.Create(outFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		defer o.Close()
	}
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
	recs, err := events.Read(f, r, ts, szExtra, sympSize, brlen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	w := csv.NewWriter(o)
	w.Comma = '\t'
	w.UseCRLF = true
	defer w.Flush()
	err = w.Write([]string{"Tree", "RecID", "Cost", "Vics", "Symps", "Point", "Found"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	for _, rc := range recs {
		rc.SetVicCost(VicCost)
		rc.SetSympCost(SympCost)
		rc.SetFoundCost(FoundCost)
		rc.SetPointCost(PointCost)
		ev := rc.Evaluate()
		row := []string{
			rc.Tree.ID,
			rc.ID,
			strconv.FormatFloat(rc.Cost(), 'f', 3, 64),
			strconv.FormatInt(int64(ev.Vics), 10),
			strconv.FormatInt(int64(ev.Symp), 10),
			strconv.FormatInt(int64(ev.Point), 10),
			strconv.FormatInt(int64(ev.Found), 10),
		}
		err := w.Write(row)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
	}
}
