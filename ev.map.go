// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/js-arias/evs/biogeo"
	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/raster"
	"github.com/js-arias/evs/treesvg"
)

var evMap = &cmdapp.Command{
	Run:       evMapRun,
	UsageLine: `ev.map [-i|--input file] [-s|--size number] [<imagemap>]`,
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
      
    -s number
    --size number
      Sets the size of each record in the ouput map. Default = 2

    <imagemap>
      A map in an image format (e.g. jpg, png) with an equirectangular
      projection. If no map is defined, a white background image will be used.
	`,
}

var recSize int

func init() {
	evMap.Flag.StringVar(&inFile, "input", "", "")
	evMap.Flag.StringVar(&inFile, "i", "", "")
	evMap.Flag.IntVar(&recSize, "size", 2, "")
	evMap.Flag.IntVar(&recSize, "s", 2, "")
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
	recs, err := events.Read(f, r, ts, szExtra, sympSize, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	if recSize < 1 {
		recSize = 2
	}

	err = treesvg.SVG(ts, nil, 0, 0, false)

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
						for x := c - recSize - 1; x <= c+recSize+1; x++ {
							for y := r - recSize - 1; y <= r+recSize+1; y++ {
								dest.Set(x, y, black)
							}
						}
						for x := c - recSize; x <= c+recSize; x++ {
							for y := r - recSize; y <= r+recSize; y++ {
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
