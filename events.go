// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"strconv"
	"sync"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/js-arias/evs/biogeo"
	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/raster"
	"github.com/js-arias/evs/treesvg"
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

var evEval = &cmdapp.Command{
	Run: evEvalRun,
	UsageLine: `ev.eval [-c|--columns number] [-f|--fill number]
	[-i|--input file] [-z|--size number]`,
	Short: "evaluate four event reconstructions",
	Long: `
Ev.eval reads a reconstruction in csv from the standard input, and for a given
data and tree, will print the cost of that reconstruction.

Options are:

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

    -z number
    --size number
      If set, the indicated the value of the ancestral_range / number will be
      used as extra-cost on each internal nodes.
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
	recs, err := events.Read(f, r, ts, szExtra)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	w := csv.NewWriter(o)
	defer w.Flush()
	err = w.Write([]string{"Tree", "RecID", "Cost", "Vics", "Symps", "Point", "Found"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	for _, rc := range recs {
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

var evMap = &cmdapp.Command{
	Run:       evMapRun,
	UsageLine: `ev.map [-i|--input file] [<imagemap>]`,
	Short:     "print reconstructions in a map",
	Long: `
Ev.map reads a reconstruction and export it as png files. Each node is printed
as a single image, with the name referring to the tree-ID, node-ID and
reconstruction-ID. An svg file containing the tree with all node IDs is also
produced to aid the node identification.

The image will be cropped to match the geography of the dataset.

Options are:

    -i file
    --input file
      Reads from an input file instead of standard input.

    <imagemap>
      A map in an image format (e.g. jpg, png) with an equirectangular
      projection. If no map is defined, a white background image will be used.
	`,
}

func init() {
	evMap.Flag.StringVar(&inFile, "input", "", "")
	evMap.Flag.StringVar(&inFile, "i", "", "")
}

func evMapRun(c *cmdapp.Command, args []string) {
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

	err = treesvg.SVG(ts, nil, 0, 0)

	// determines the boudaries of the geography
	minLat := float64(biogeo.MaxLat)
	maxLat := float64(biogeo.MinLat)
	minLon := float64(biogeo.MaxLon)
	maxLon := float64(biogeo.MinLon)
	for _, tx := range d.Ls {
		for _, g := range tx.Recs {
			if g.Lat < minLat {
				minLat = g.Lat
			}
			if g.Lat > maxLat {
				maxLat = g.Lat
			}
			if g.Lon < minLon {
				minLon = g.Lon
			}
			if g.Lon > maxLon {
				maxLon = g.Lon
			}
		}
	}
	maxLat += 10
	if maxLat > biogeo.MaxLat {
		maxLat = biogeo.MaxLat
	}
	minLat -= 10
	if minLat < biogeo.MinLat {
		minLat = biogeo.MinLat
	}
	maxLon += 10
	if maxLon > biogeo.MaxLon {
		maxLon = biogeo.MaxLon
	}
	minLon -= 10
	if minLon < biogeo.MinLon {
		minLon = biogeo.MinLon
	}

	// loads the map
	var imgmap image.Image
	if len(args) == 0 {
		imgmap = image.NewRGBA(image.Rect(0, 0, 360, 180))
	} else {
		im, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		imgmap, _, err = image.Decode(im)
		im.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
	}
	sizeX := imgmap.Bounds().Max.X
	sizeY := imgmap.Bounds().Max.Y
	scaleX := float64(sizeX) / 360
	scaleY := float64(sizeY) / 180

	szX := (maxLon - minLon) * scaleX
	szY := (maxLat - minLat) * scaleY
	originX := int((180 + minLon) * scaleX)
	originY := int((90 - maxLat) * scaleY)

	imgPump := make(chan *image.RGBA64, 10)
	go func() {
		ln := 0
		for _, rc := range recs {
			for i := range rc.Rec {
				if rc.Rec[i].Node.First == nil {
					continue
				}
				ln++
			}
		}
		for i := 0; i < ln; i++ {
			dest := image.NewRGBA64(image.Rect(0, 0, int(szX), int(szY)))
			for x := 0; x < int(szX); x++ {
				for y := 0; y < int(szY); y++ {
					dest.Set(x, y, imgmap.At(x+originX, y+originY))
				}
			}
			imgPump <- dest
		}
	}()

	black := color.RGBA64{0, 0, 0, 0xFFFF}

	var done sync.WaitGroup
	var errVal error
	for _, rv := range recs {
		done.Add(1)
		go func(rc *events.Recons) {
			defer done.Done()
			for i := range rc.Rec {
				if rc.Rec[i].Node.First == nil {
					continue
				}
				dest := <-imgPump
				e := "noev"
				switch rc.Rec[i].Flag {
				case events.Vic:
					e = "vic"
				case events.SympU, events.SympL, events.SympR:
					e = "symp"
				case events.PointL, events.PointR:
					e = "point"
				case events.FoundL, events.FoundR:
					e = "found"
				}
				for j := range rc.Rec {
					if rc.Rec[j].Node.First != nil {
						continue
					}
					if len(rc.Rec[j].Node.Term) == 0 {
						continue
					}
					tx := d.Taxon(rc.Rec[j].Node.Term)
					if tx == nil {
						continue
					}
					cr, ok := eventColor(rc, i, j)
					if !ok {
						continue
					}
					for _, g := range tx.Recs {
						c := int((180+g.Lon)*scaleX) - originX
						r := int((90-g.Lat)*scaleY) - originY
						for x := c - 3; x <= c+3; x++ {
							for y := r - 3; y <= r+3; y++ {
								dest.Set(x, y, black)
							}
						}
						for x := c - 2; x <= c+2; x++ {
							for y := r - 2; y <= r+2; y++ {
								dest.Set(x, y, cr)
							}
						}
					}
				}
				im, err := os.Create(rc.Tree.ID + "-n" + rc.Rec[i].Node.ID + "-r" + rc.ID + "-" + e + ".png")
				if err != nil {
					errVal = err
					return
				}
				err = png.Encode(im, dest)
				im.Close()
				if err != nil {
					errVal = err
					return
				}
			}
		}(rv)
	}
	done.Wait()
	if errVal != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), errVal)
		os.Exit(1)
	}
}

func eventColor(rc *events.Recons, i, j int) (color.RGBA64, bool) {
	n := rc.Rec[j].Node
	for anc := n.Anc; anc != nil; anc = anc.Anc {
		j = anc.Index
		switch rc.Rec[j].Flag {
		case events.Vic:
			if j == i {
				if rc.Rec[i].SetL == n.Index {
					return color.RGBA64{0xFFFF, 0, 0, 0xFFFF}, true
				} else if rc.Rec[i].SetR == n.Index {
					return color.RGBA64{0, 0, 0xFFFF, 0xFFFF}, true
				} else {
					return color.RGBA64{}, false
				}
			}
		case events.SympL:
			if rc.Rec[j].SetL != n.Index {
				return color.RGBA64{}, false
			}
		case events.PointR, events.FoundR:
			if rc.Rec[j].SetL != n.Index {
				if j == i {
					return color.RGBA64{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF}, true
				}
				return color.RGBA64{}, false
			}
		case events.SympR:
			if rc.Rec[j].SetR != n.Index {
				return color.RGBA64{}, false
			}
		case events.PointL, events.FoundL:
			if rc.Rec[j].SetR != n.Index {
				if j == i {
					return color.RGBA64{0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF}, true
				}
				return color.RGBA64{}, false
			}
		}
		if j == i {
			return color.RGBA64{0, 0xFFFF, 0, 0xFFFF}, true
		}
		n = anc
	}
	return color.RGBA64{}, false
}
