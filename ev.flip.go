// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/raster"
)

var evFlip = &cmdapp.Command{
	Run: evFlipRun,
	UsageLine: `ev.flip [-b|--brlen] [-c|--columns number] [-f|--fill number]
	[-m|--random number] [--found number] [--point number] [--symp number]
	[--vic number] [-o|--output file] [-p|--procs number]
	[-r|--replicates number] [-v|--verbose] [-z|--size number]
	[-sympSize number]`,
	Short: "flip search with four events",
	Long: `
Ev.flip searches with the flipping algorithm for the most parsimonious
biogeographic history using the geographically explicit event model.

The answer will be send to the standard output with the following columns:
	Tree	Tree identifier
	Node	Node identifier
	Event	Event identifier (using a single letter)
	Set	Identifier of the assigned set

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

    -m number
    --random number
      Set the probability (as percentage) of randomly modifying a node in the
      initial OR reconstruction at the start of each replicate. Default = 10.

    --found number
    --point number
    --symp number
    --vic number
      Sets the cost of a given type of event. Event costs should be greater
      than 0. Default = 1.

    -o file
    --output file
      Set the output file, instead of the standard output.

    -p number
    --procs number
      Set the number of parallel process used for the search. By default it
      will use the double of available processors.

    -r number
    --replicates number
      Set the number of replicates for each process in the search. Default =
      100.

    -v
    --verbose
      Set verbose output.

    -z number
    --size number
      If set, the indicated the value of the ancestral_range_size / number
      will be used as extra-cost on internal nodes.

    --sympSize number
      If set, it will add to the cost of a sympatry event, the result of 
      ancestral_range_size / number.
	`,
}

// events flags
var (
	numProc   int     // -p|--proc
	numRand   int     // -m|--random
	numReps   int     // -r|--replicates
	brlen     bool    // -b|--brlen
	szExtra   float64 // -z|--size
	sympSize  float64 // --sympSize
	VicCost   float64 // --vic
	SympCost  float64 // --symp
	PointCost float64 // --point
	FoundCost float64 // --found
)

func setEventFlags(c *cmdapp.Command) {
	c.Flag.Float64Var(&szExtra, "size", 0, "")
	c.Flag.Float64Var(&szExtra, "z", 0, "")
	c.Flag.Float64Var(&sympSize, "sympSize", 0, "")
	c.Flag.Float64Var(&VicCost, "vic", 1, "")
	c.Flag.Float64Var(&SympCost, "symp", 1, "")
	c.Flag.Float64Var(&PointCost, "point", 1, "")
	c.Flag.Float64Var(&FoundCost, "found", 1, "")
	c.Flag.BoolVar(&brlen, "brlen", false, "")
	c.Flag.BoolVar(&brlen, "b", false, "")
}

func init() {
	setRasterFlags(evFlip)
	setEventFlags(evFlip)
	evFlip.Flag.StringVar(&outFile, "output", "", "")
	evFlip.Flag.StringVar(&outFile, "o", "", "")
	evFlip.Flag.IntVar(&numProc, "procs", 0, "")
	evFlip.Flag.IntVar(&numProc, "p", 0, "")
	evFlip.Flag.IntVar(&numRand, "random", 25, "")
	evFlip.Flag.IntVar(&numRand, "m", 25, "")
	evFlip.Flag.IntVar(&numReps, "replicates", 100, "")
	evFlip.Flag.IntVar(&numReps, "r", 100, "")
	evFlip.Flag.BoolVar(&verbose, "verbose", false, "")
	evFlip.Flag.BoolVar(&verbose, "v", false, "")
}

func evFlipRun(c *cmdapp.Command, args []string) {
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
	best := make(chan []*events.Recons)
	if numReps <= 0 {
		numReps = 100
	}
	if numProc <= 0 {
		numProc = runtime.NumCPU() * 2
	}
	for _, t := range ts {
		out := make(chan []*events.Recons)
		or := events.OR(r, t, szExtra, sympSize, brlen)
		or.SetVicCost(VicCost)
		or.SetSympCost(SympCost)
		or.SetFoundCost(FoundCost)
		or.SetPointCost(PointCost)
		go doFlip(or, out)
		go getBestFlip(or, out, best)
	}
	head := true
	for _ = range ts {
		recs := <-best
		if verbose {
			fmt.Printf("Tree %s best: %.3f recs found: %d\n", recs[0].Tree.ID, recs[0].Cost(), len(recs))
		}
		for _, b := range recs {
			err := b.Write(o, head)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
				os.Exit(1)
			}
			head = false
		}
	}
}

// getBestFlip selects the best reconstruction of a flip.
func getBestFlip(or *events.Recons, in, out chan []*events.Recons) {
	best := <-in
	for p := 1; p < numProc; p++ {
		b := <-in
		if b[0].Cost() < best[0].Cost() {
			best = b
		} else if b[0].Cost() == best[0].Cost() {
			for _, r1 := range b {
				diff := true
				for _, r2 := range best {
					if !r1.IsDiff(r2) {
						diff = false
						break
					}
				}
				if diff {
					best = append(best, r1)
				}
			}
		}
	}
	out <- best
}

// doFlip setups the flipping algorithm.
func doFlip(or *events.Recons, out chan []*events.Recons) {
	for p := 0; p < numProc; p++ {
		go func(px int) {
			hits := 1
			var best []*events.Recons
			r := or.MakeCopy()
			best = append(best, or.MakeCopy())
			var nodes []int
			for i := range r.Rec {
				if r.Rec[i].SetL != -1 {
					nodes = append(nodes, i)
				}
			}
			evs := events.Events()
			for i := 0; i < numReps; i++ {
				if i > 0 {
					r.Copy(or)
				}
				r.Randomize(numRand, evs)
				flipRecons(r, nodes, evs)
				if r.Cost() < best[0].Cost() {
					if verbose {
						fmt.Printf("Replicate %s.%d.%d: %.3f [best so far]\n", r.Tree.ID, px, i, r.Cost())
					}
					hits = 1
					cp := r.MakeCopy()
					cp.ID = fmt.Sprintf("%d.%d", px, i)
					best = append([]*events.Recons{}, cp)
				} else if r.Cost() == best[0].Cost() {
					if verbose {
						fmt.Printf("Replicate %s.%d.%d: %.3f [hit best]\n", r.Tree.ID, px, i, r.Cost())
					}
					hits++
					diff := true
					for _, b := range best {
						if !r.IsDiff(b) {
							diff = false
							break
						}
					}
					if diff {
						cp := r.MakeCopy()
						cp.ID = fmt.Sprintf("%d.%d", px, i)
						best = append(best, cp)
					}
				} else if verbose {
					fmt.Printf("Replicate %s.%d.%d: %.3f\n", r.Tree.ID, px, i, r.Cost())
				}
			}
			if verbose {
				fmt.Printf("Process: %s.%d hits: %d (of %d) best: %.3f stored: %d\n", or.Tree.ID, px, hits, numReps, best[0].Cost(), len(best))
			}
			out <- best
		}(p)
	}
}

// flipRecons permorms the flip algorithm on list of nodes an events.
func flipRecons(r *events.Recons, nodes, evs []int) float64 {
	best := r.Cost()
	for doAgain := true; doAgain; {
		doAgain = false
		shuffle(nodes)
		for _, n := range nodes {
			prev := r.Rec[n].Flag
			shuffle(evs)
			for _, e := range evs {
				if e == prev {
					continue
				}
				r.Rec[n].Flag = e
				if r.DownPass(n) < best {
					break
				}
			}
			if r.Cost() < best {
				best = r.Cost()
				doAgain = true
				break
			}
			r.Rec[n].Flag = prev
			r.DownPass(n)
		}
	}
	return r.Cost()
}

//
