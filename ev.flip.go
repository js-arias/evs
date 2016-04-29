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
	UsageLine: `ev.flip [-c|--columns number] [-f|--fill number]
	[-m|--random number] [--nofound] [--nopoint] [--nosymp] [--novic]
	[-o|--output file] [-p|--procs number] [-r|--replicates number]
	[-v|--verbose] [-z|--size number]`,
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

    --nofound
    --nopoint
    --nosymp
    --novic
      Prohibits a given type of event for the searches. At least one type of
      event should be allowed.

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
      If set, the indicated the value of the ancestral_range / number will be
      used as extra-cost on internal nodes.
	`,
}

// events flags
var (
	numProc int  // -p|--proc
	numRand int  // -m|--random
	numReps int  // -r|--replicates
	szExtra int  // -z|--size
	noVic   bool // --novic
	noSymp  bool // --nosymp
	noPoint bool // -nopoint
	noFound bool // -nofound
)

func setEventFlags(c *cmdapp.Command) {
	c.Flag.IntVar(&szExtra, "size", 0, "")
	c.Flag.IntVar(&szExtra, "z", 0, "")
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
	evFlip.Flag.BoolVar(&noVic, "novic", false, "")
	evFlip.Flag.BoolVar(&noSymp, "nosymp", false, "")
	evFlip.Flag.BoolVar(&noPoint, "nopoint", false, "")
	evFlip.Flag.BoolVar(&noFound, "nofound", false, "")
}

func evFlipRun(c *cmdapp.Command, args []string) {
	if noVic && noSymp && noPoint && noFound {
		fmt.Fprintf(os.Stderr, "%s: at least one event must be allowed\n", c.Name())
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
		or := events.OR(r, t, szExtra)

		// modify or to take into account the prohibited events
		if (noVic) && (noSymp) {
			for i := range or.Rec {
				if (or.Rec[i].Flag == events.Vic) || (or.Rec[i].Flag == events.SympU) {
					if noFound {
						or.Rec[i].Flag = events.PointR
					} else {
						or.Rec[i].Flag = events.FoundR
					}
					or.DownPass(i)
				}
			}
		} else if noVic {
			for i := range or.Rec {
				if or.Rec[i].Flag == events.Vic {
					or.Rec[i].Flag = events.SympU
					or.DownPass(i)
				}
			}
		} else if noSymp {
			for i := range or.Rec {
				if or.Rec[i].Flag == events.SympU {
					or.Rec[i].Flag = events.Vic
					or.DownPass(i)
				}
			}
		}
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
			evs := makeEvents()
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

// make events slice.
func makeEvents() []int {
	var evs []int
	if !noVic {
		evs = events.AddVic(evs)
	}
	if !noSymp {
		evs = events.AddSymp(evs)
	}
	if !noPoint {
		evs = events.AddPoint(evs)
	}
	if !noFound {
		evs = events.AddFounder(evs)
	}
	return evs
}
