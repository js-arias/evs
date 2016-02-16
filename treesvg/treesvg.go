// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package treesvg

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"

	"github.com/js-arias/evs/events"
	"github.com/js-arias/evs/tree"
)

type Node struct {
	Level int
	Nest  int
	X     int
	Y     int
	TopY  int
	BotY  int
}

type Tree struct {
	ID    string
	Tree  *tree.Tree
	Nodes []Node
}

func prepareTree(t *tree.Tree, stepX, stepY int) *Tree {
	tv := &Tree{
		ID:    t.ID,
		Tree:  t,
		Nodes: make([]Node, len(t.Nodes)),
	}
	y := 0
	tv.prepareNode(t.Root, &y, stepY)
	for i := range tv.Nodes {
		tv.Nodes[i].X = ((tv.Nodes[0].Level - tv.Nodes[i].Level) * stepX) + 5
	}
	return tv
}

func (tv *Tree) prepareNode(n *tree.Node, y *int, stepY int) {
	if n.Anc != nil {
		tv.Nodes[n.Index].Nest = tv.Nodes[n.Anc.Index].Nest + 1
	}
	if n.First == nil {
		tv.Nodes[n.Index].Y = (*y * stepY) + 5
		*y += 1
		return
	}
	topY := 640000 * stepY
	botY := -1
	for d := n.First; d != nil; d = d.Sister {
		tv.prepareNode(d, y, stepY)
		if tv.Nodes[d.Index].Level >= tv.Nodes[n.Index].Level {
			tv.Nodes[n.Index].Level = tv.Nodes[d.Index].Level + 1
		}
		if tv.Nodes[d.Index].Y < topY {
			topY = tv.Nodes[d.Index].Y
		}
		if tv.Nodes[d.Index].Y > botY {
			botY = tv.Nodes[d.Index].Y
		}
	}
	tv.Nodes[n.Index].TopY = topY
	tv.Nodes[n.Index].BotY = botY
	tv.Nodes[n.Index].Y = topY + ((botY - topY) / 2)
}

func arrow(x int, y int, top bool) string {
	y2 := y + 5
	if top {
		y2 = y - 5
	}
	return fmt.Sprintf("%d,%d %d,%d %d,%d", x-2, y, x+2, y, x, y2)
}

// SVG creates an svg version of a list of trees.
func SVG(ts []*tree.Tree, recs []*events.Recons, stepX, stepY int) error {
	if stepX <= 0 {
		stepX = 10
	}
	if stepY <= 0 {
		stepY = 10
	}

	// draws the trees (with node ids)
	tvmap := make(map[string]*Tree)
	for _, t := range ts {
		tv := prepareTree(t, stepX, stepY)
		tvmap[t.ID] = tv
		f, err := os.Create(t.ID + ".svg")
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "%s", xml.Header)
		e := xml.NewEncoder(f)
		svg := xml.StartElement{
			Name: xml.Name{Local: "svg"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"},
			},
		}
		e.EncodeToken(svg)
		desc := xml.StartElement{Name: xml.Name{Local: "desc"}}
		e.EncodeToken(desc)
		e.EncodeToken(xml.CharData(t.ID))
		e.EncodeToken(desc.End())
		g := xml.StartElement{
			Name: xml.Name{Local: "g"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "stroke-width"}, Value: "2"},
				{Name: xml.Name{Local: "stroke"}, Value: "black"},
				{Name: xml.Name{Local: "stroke-linecap"}, Value: "round"},
				{Name: xml.Name{Local: "font-family"}, Value: "Verdana"},
				{Name: xml.Name{Local: "font-size"}, Value: "10"},
			},
		}
		e.EncodeToken(g)

		// draws the tree
		for i := range tv.Nodes {
			e.EncodeToken(xml.Comment("Node: " + t.Nodes[i].ID))
			// horizontal line
			ln := xml.StartElement{
				Name: xml.Name{Local: "line"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "x1"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X-5), 10)},
					{Name: xml.Name{Local: "y1"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y), 10)},
					{Name: xml.Name{Local: "x2"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X), 10)},
					{Name: xml.Name{Local: "y2"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y), 10)},
				},
			}
			if t.Nodes[i].Anc != nil {
				ln.Attr[0].Value = strconv.FormatInt(int64(tv.Nodes[t.Nodes[i].Anc.Index].X), 10)
			}
			e.EncodeToken(ln)
			e.EncodeToken(ln.End())
			// term name
			if len(t.Nodes[i].Term) > 0 {
				tx := xml.StartElement{
					Name: xml.Name{Local: "text"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "x"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X+5), 10)},
						{Name: xml.Name{Local: "y"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y+5), 10)},
						{Name: xml.Name{Local: "stroke-width"}, Value: "0"},
					},
				}
				e.EncodeToken(tx)
				e.EncodeToken(xml.CharData(t.Nodes[i].Term))
				e.EncodeToken(tx.End())
				continue
			} else if t.Nodes[i].First == nil {
				continue
			}
			// draws vertical line
			ln.Attr[0].Value = ln.Attr[2].Value
			ln.Attr[1].Value = strconv.FormatInt(int64(tv.Nodes[i].TopY), 10)
			ln.Attr[3].Value = strconv.FormatInt(int64(tv.Nodes[i].BotY), 10)
			e.EncodeToken(ln)
			e.EncodeToken(ln.End())
		}

		// draws a label for the internal nodes
		for i := range tv.Nodes {
			if t.Nodes[i].First == nil {
				continue
			}
			// draws a cricle at the node
			circ := xml.StartElement{
				Name: xml.Name{Local: "circle"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "cx"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X), 10)},
					{Name: xml.Name{Local: "cy"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y), 10)},
					{Name: xml.Name{Local: "r"}, Value: "7"},
					{Name: xml.Name{Local: "fill"}, Value: "white"},
					{Name: xml.Name{Local: "stroke"}, Value: "black"},
					{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
				},
			}
			e.EncodeToken(circ)
			e.EncodeToken(circ.End())
			// put node ID
			tx := xml.StartElement{
				Name: xml.Name{Local: "text"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "x"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X-5), 10)},
					{Name: xml.Name{Local: "y"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y+2), 10)},
					{Name: xml.Name{Local: "stroke-width"}, Value: "0"},
					{Name: xml.Name{Local: "font-size"}, Value: "6"},
				},
			}
			e.EncodeToken(tx)
			e.EncodeToken(xml.CharData(t.Nodes[i].ID))
			e.EncodeToken(tx.End())
		}
		e.EncodeToken(g.End())
		e.EncodeToken(svg.End())
		e.Flush()
		f.Close()
	}
	for _, rc := range recs {
		t := rc.Tree
		tv := tvmap[t.ID]
		f, err := os.Create(t.ID + "-r" + rc.ID + ".svg")
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "%s", xml.Header)
		e := xml.NewEncoder(f)
		svg := xml.StartElement{
			Name: xml.Name{Local: "svg"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"},
			},
		}
		e.EncodeToken(svg)
		desc := xml.StartElement{Name: xml.Name{Local: "desc"}}
		e.EncodeToken(desc)
		e.EncodeToken(xml.CharData(t.ID + "-" + rc.ID))
		e.EncodeToken(desc.End())
		g := xml.StartElement{
			Name: xml.Name{Local: "g"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "stroke-width"}, Value: "2"},
				{Name: xml.Name{Local: "stroke"}, Value: "black"},
				{Name: xml.Name{Local: "stroke-linecap"}, Value: "round"},
				{Name: xml.Name{Local: "font-family"}, Value: "Verdana"},
				{Name: xml.Name{Local: "font-size"}, Value: "10"},
			},
		}
		e.EncodeToken(g)
		for i := range tv.Nodes {
			e.EncodeToken(xml.Comment("Node: " + t.Nodes[i].ID))
			// horizontal line
			ln := xml.StartElement{
				Name: xml.Name{Local: "line"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "x1"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X-5), 10)},
					{Name: xml.Name{Local: "y1"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y), 10)},
					{Name: xml.Name{Local: "x2"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X), 10)},
					{Name: xml.Name{Local: "y2"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y), 10)},
				},
			}
			if t.Nodes[i].Anc != nil {
				ln.Attr[0].Value = strconv.FormatInt(int64(tv.Nodes[t.Nodes[i].Anc.Index].X), 10)
			}
			e.EncodeToken(ln)
			e.EncodeToken(ln.End())
			// term name
			if len(t.Nodes[i].Term) > 0 {
				tx := xml.StartElement{
					Name: xml.Name{Local: "text"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "x"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X+5), 10)},
						{Name: xml.Name{Local: "y"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y+5), 10)},
						{Name: xml.Name{Local: "stroke-width"}, Value: "0"},
					},
				}
				e.EncodeToken(tx)
				e.EncodeToken(xml.CharData(t.Nodes[i].Term))
				e.EncodeToken(tx.End())
				continue
			} else if t.Nodes[i].First == nil {
				continue
			}
			// draws vertical line
			ln.Attr[0].Value = ln.Attr[2].Value
			ln.Attr[1].Value = strconv.FormatInt(int64(tv.Nodes[i].TopY), 10)
			ln.Attr[3].Value = strconv.FormatInt(int64(tv.Nodes[i].BotY), 10)
			e.EncodeToken(ln)
			e.EncodeToken(ln.End())

			// draws the event
			switch rc.Rec[i].Flag {
			case events.Vic:
				rect := xml.StartElement{
					Name: xml.Name{Local: "rect"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "x"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X-2), 10)},
						{Name: xml.Name{Local: "y"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y-2), 10)},
						{Name: xml.Name{Local: "width"}, Value: "5"},
						{Name: xml.Name{Local: "height"}, Value: "5"},
						{Name: xml.Name{Local: "fill"}, Value: "black"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
					},
				}
				e.EncodeToken(rect)
				e.EncodeToken(rect.End())
			case events.SympL, events.SympR, events.SympU:
				rect := xml.StartElement{
					Name: xml.Name{Local: "rect"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "x"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X-2), 10)},
						{Name: xml.Name{Local: "y"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y-2), 10)},
						{Name: xml.Name{Local: "width"}, Value: "5"},
						{Name: xml.Name{Local: "height"}, Value: "5"},
						{Name: xml.Name{Local: "fill"}, Value: "white"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
						{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
					},
				}
				e.EncodeToken(rect)
				e.EncodeToken(rect.End())
			case events.PointL:
				circ := xml.StartElement{
					Name: xml.Name{Local: "circle"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "cx"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X), 10)},
						{Name: xml.Name{Local: "cy"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y-4), 10)},
						{Name: xml.Name{Local: "r"}, Value: "2"},
						{Name: xml.Name{Local: "fill"}, Value: "white"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
						{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
					},
				}
				p := rc.Rec[i].SetL
				if tv.Nodes[p].Y > tv.Nodes[i].Y {
					circ.Attr[1].Value = strconv.FormatInt(int64(tv.Nodes[i].Y+4), 10)
				}
				e.EncodeToken(circ)
				e.EncodeToken(circ.End())
			case events.PointR:
				circ := xml.StartElement{
					Name: xml.Name{Local: "circle"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "cx"}, Value: strconv.FormatInt(int64(tv.Nodes[i].X), 10)},
						{Name: xml.Name{Local: "cy"}, Value: strconv.FormatInt(int64(tv.Nodes[i].Y-4), 10)},
						{Name: xml.Name{Local: "r"}, Value: "2"},
						{Name: xml.Name{Local: "fill"}, Value: "white"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
						{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
					},
				}
				p := rc.Rec[i].SetR
				if tv.Nodes[p].Y > tv.Nodes[i].Y {
					circ.Attr[1].Value = strconv.FormatInt(int64(tv.Nodes[i].Y+4), 10)
				}
				e.EncodeToken(circ)
				e.EncodeToken(circ.End())
			case events.FoundL:
				poly := xml.StartElement{
					Name: xml.Name{Local: "polygon"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "points"}, Value: ""},
						{Name: xml.Name{Local: "fill"}, Value: "white"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
						{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
					},
				}
				f := rc.Rec[i].SetL
				if tv.Nodes[f].Y > tv.Nodes[i].Y {
					poly.Attr[0].Value = arrow(tv.Nodes[i].X, tv.Nodes[i].Y+2, false)
				} else {
					poly.Attr[0].Value = arrow(tv.Nodes[i].X, tv.Nodes[i].Y-2, true)
				}
				e.EncodeToken(poly)
				e.EncodeToken(poly.End())
			case events.FoundR:
				poly := xml.StartElement{
					Name: xml.Name{Local: "polygon"},
					Attr: []xml.Attr{
						{Name: xml.Name{Local: "points"}, Value: ""},
						{Name: xml.Name{Local: "fill"}, Value: "white"},
						{Name: xml.Name{Local: "stroke"}, Value: "black"},
						{Name: xml.Name{Local: "stroke-width"}, Value: "1"},
					},
				}
				f := rc.Rec[i].SetR
				if tv.Nodes[f].Y > tv.Nodes[i].Y {
					poly.Attr[0].Value = arrow(tv.Nodes[i].X, tv.Nodes[i].Y+2, false)
				} else {
					poly.Attr[0].Value = arrow(tv.Nodes[i].X, tv.Nodes[i].Y-2, true)
				}
				e.EncodeToken(poly)
				e.EncodeToken(poly.End())
			}
		}
		e.EncodeToken(g.End())
		e.EncodeToken(svg.End())
		e.Flush()
		f.Close()
	}
	return nil
}
