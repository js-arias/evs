// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package events

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/js-arias/evs/bitfield"
	"github.com/js-arias/evs/raster"
	"github.com/js-arias/evs/tree"
)

const (
	Undef = iota
	Vic
	SympU
	SympL
	SympR
	PointL
	PointR
	FoundL
	FoundR
)

// Events makes an event slice.
func Events() []int {
	return []int{
		Vic,
		SympU,
		SympL,
		SympR,
		PointL,
		PointR,
		FoundL,
		FoundR,
	}
}

const infinityCost = 10000000

// A Node is a reconstruction of a node.
type Node struct {
	Node *tree.Node
	Fill bitfield.Bitfield
	Obs  bitfield.Bitfield
	SetL int
	SetR int
	Cost float64
	Flag int
}

// A Recons is a reconstruction of a given raster in a particular tree.
type Recons struct {
	ID     string
	Tree   *tree.Tree
	Raster *raster.Raster
	Rec    []Node

	UseLen bool

	// events costs
	Size     float64
	SympSize float64
	VicC     float64
	SympC    float64
	PointC   float64
	FoundC   float64
}

// OR creates an OR reconstruction based on raster and tree data. If scaled is
// true, then the cost at each node will be scaled by the ancestral
// distribution.
func OR(r *raster.Raster, t *tree.Tree, size, sympSize float64, useLen bool) *Recons {
	or := &Recons{
		ID:       "or",
		Tree:     t,
		Raster:   r,
		Rec:      make([]Node, len(t.Nodes)),
		UseLen:   useLen,
		Size:     size,
		SympSize: sympSize,
		VicC:     1,
		SympC:    1,
		PointC:   1,
		FoundC:   1,
	}
	for i := len(t.Nodes) - 1; i >= 0; i-- {
		n := t.Nodes[i]
		or.Rec[i].Node = n
		or.Rec[i].Obs = make(bitfield.Bitfield, r.Fields)
		or.Rec[i].Fill = make(bitfield.Bitfield, r.Fields)
		or.Rec[i].SetL = -1
		or.Rec[i].SetR = -1
		if len(n.Term) > 0 {
			tx := r.Taxon(n.Term)
			if tx == nil {
				continue
			}
			copy(or.Rec[i].Obs, tx.Obs)
			copy(or.Rec[i].Fill, tx.Fill)
			if or.Size > 0 {
				cost := float64(or.Rec[i].Obs.Count()-1) / or.Size
				if or.UseLen {
					cost *= n.Len
				}
				or.Rec[i].Cost = cost
			}
			continue
		}
		var setL, setR *tree.Node
		numDesc := 0
		for desc := n.First; desc != nil; desc = desc.Sister {
			or.Rec[i].Obs.Union(or.Rec[desc.Index].Obs)
			or.Rec[i].Fill.Union(or.Rec[desc.Index].Fill)
			or.Rec[i].Cost += or.Rec[desc.Index].Cost
			if or.Rec[desc.Index].Obs.Count() == 0 {
				continue
			}
			if setL == nil {
				setL = desc
			} else if setR == nil {
				setR = desc
			}
			numDesc++
		}
		if numDesc != 2 {
			continue
		}
		or.Rec[i].SetL = setL.Index
		or.Rec[i].SetR = setR.Index
		cv := or.vicariance(i)
		cs := or.sympatry(i)
		if or.Size > 0 {
			csz := (float64(or.Rec[i].Obs.Count()-1) / or.Size)
			if or.UseLen {
				csz *= n.Len
			}
			cs += csz
			cv += csz
		}
		if cv < cs {
			or.Rec[i].Flag = Vic
			or.Rec[i].Cost += cv
		} else {
			or.Rec[i].Flag = SympU
			or.Rec[i].Cost += cs
		}
	}
	return or
}

// Read reads a reconstruction from one or most trees in tsv format from an
// input stream.
func Read(in io.Reader, ras *raster.Raster, ts []*tree.Tree, size, sympSize float64, useLen bool) ([]*Recons, error) {
	var recs []*Recons
	r := csv.NewReader(in)
	r.Comma = '\t'
	r.TrimLeadingSpace = true

	// reads the file header
	h, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("header (recons): %v", err)
	}
	treeF := -1
	ID := -1
	node := -1
	eventF := -1
	set := -1
	for i, v := range h {
		switch strings.ToLower(v) {
		case "id":
			ID = i
		case "tree":
			treeF = i
		case "node", "node id":
			node = i
		case "event", "ev":
			eventF = i
		case "set":
			set = i
		}
	}
	if (ID < 0) || (treeF < 0) || (node < 0) || (eventF < 0) || (set < 0) {
		return nil, errors.New("header (recons): incomplete header")
	}
	var prev string
	var id string
	var nr *Recons
	var t *tree.Tree
	for i := 1; ; i++ {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("(recons) row %d: %v", i, err)
		}
		if lr := len(row); (lr <= treeF) || (lr <= node) || (lr <= eventF) || (lr <= set) {
			continue
		}
		if (len(row[treeF]) == 0) || (len(row[node]) == 0) || (len(row[ID]) == 0) {
			continue
		}
		if (prev != row[treeF]) || (id != row[ID]) {
			// look for a tree, if the tree is not found, ignores
			// the tree.
			for _, tv := range ts {
				if strings.ToLower(tv.ID) == strings.ToLower(row[treeF]) {
					t = tv
					break
				}
			}
			if t == nil {
				continue
			}
			prev = row[treeF]
			id = row[ID]
			nr = OR(ras, t, size, sympSize, useLen)
			nr.ID = row[ID]
			recs = append(recs, nr)
		}
		n := -1
		for j, v := range t.Nodes {
			if v.ID == row[node] {
				n = j
				break
			}
		}
		if (nr.Rec[n].SetL == -1) || (row[eventF] == "*") {
			continue
		}
		event := Undef
		setL, setR := nr.Rec[n].SetL, nr.Rec[n].SetR
		switch strings.ToLower(row[eventF]) {
		case "v":
			event = Vic
		case "s":
			event = SympU
			if row[set] == "*" {
				break
			}
			if t.Nodes[setL].ID == row[set] {
				event = SympL
			} else if t.Nodes[setR].ID == row[set] {
				event = SympR
			} else {
				continue
			}
		case "p":
			if t.Nodes[setL].ID == row[set] {
				event = PointR
			} else if t.Nodes[setR].ID == row[set] {
				event = PointL
			} else {
				return nil, fmt.Errorf("(recons) row %d: invalid set for node %s (tree %s)", i, t.Nodes[n].ID, t.ID)
			}
		case "f":
			if t.Nodes[setL].ID == row[set] {
				event = FoundR
			} else if t.Nodes[setR].ID == row[set] {
				event = FoundL
			} else {
				return nil, fmt.Errorf("(recons) row %d: invalid set for node %s (tree %s)", i, t.Nodes[n].ID, t.ID)
			}
		default:
			return nil, fmt.Errorf("(recons) row %d: unknown event %s", i, row[eventF])
		}
		nr.Rec[n].Flag = event
		nr.DownPass(n)
	}
	return recs, nil
}

// Cost returns the cost of a given reconstruction.
func (r *Recons) Cost() float64 {
	return r.Rec[0].Cost
}

// MakeCopy creates a new copy of a reconstruction.
func (r *Recons) MakeCopy() *Recons {
	cp := &Recons{
		ID:       r.ID,
		Tree:     r.Tree,
		Raster:   r.Raster,
		Rec:      make([]Node, len(r.Rec)),
		UseLen:   r.UseLen,
		Size:     r.Size,
		VicC:     r.VicC,
		SympC:    r.SympC,
		FoundC:   r.FoundC,
		PointC:   r.PointC,
		SympSize: r.SympSize,
	}
	for i := range r.Rec {
		cp.Rec[i].Node = r.Rec[i].Node
		cp.Rec[i].Obs = make(bitfield.Bitfield, cp.Raster.Fields)
		cp.Rec[i].Fill = make(bitfield.Bitfield, cp.Raster.Fields)
		copy(cp.Rec[i].Obs, r.Rec[i].Obs)
		copy(cp.Rec[i].Fill, r.Rec[i].Fill)
		cp.Rec[i].SetL = r.Rec[i].SetL
		cp.Rec[i].SetR = r.Rec[i].SetR
		cp.Rec[i].Cost = r.Rec[i].Cost
		cp.Rec[i].Flag = r.Rec[i].Flag
	}
	return cp
}

// SetVicCost sets a new vicariance cost and updates the reconstruction.
func (r *Recons) SetVicCost(c float64) {
	if r.VicC == c {
		return
	}
	r.VicC = c
	for i := range r.Rec {
		if r.Rec[i].Flag == Vic {
			r.DownPass(i)
		}
	}
}

// SetSympCost sets a new sympatry cost and updates the reconstruction.
func (r *Recons) SetSympCost(c float64) {
	if r.SympC == c {
		return
	}
	r.SympC = c
	for i := range r.Rec {
		if (r.Rec[i].Flag >= SympU) && (r.Rec[i].Flag <= SympR) {
			r.DownPass(i)
		}
	}
}

// SetPointCost sets a new point cost and updates the reconstruction.
func (r *Recons) SetPointCost(c float64) {
	if r.PointC == c {
		return
	}
	r.PointC = c
	for i := range r.Rec {
		if (r.Rec[i].Flag == PointL) || (r.Rec[i].Flag == PointR) {
			r.DownPass(i)
		}
	}
}

// SetFoundCost sets a new founder cost and updates the reconstruction.
func (r *Recons) SetFoundCost(c float64) {
	if r.FoundC == c {
		return
	}
	r.FoundC = c
	for i := range r.Rec {
		if (r.Rec[i].Flag == FoundL) || (r.Rec[i].Flag == FoundR) {
			r.DownPass(i)
		}
	}
}

// IsDiff returns true if the the reconstruction r is different from
// reconstruction cp.
func (r *Recons) IsDiff(cp *Recons) bool {
	if r.Tree != cp.Tree {
		return true
	}
	for i := range r.Rec {
		if r.Rec[i].Node.First == nil {
			continue
		}

		// All sympatry events are really the same kind of event
		// so we must test the content of the node, rather than
		// the raw event
		if (r.Rec[i].Flag >= SympU) && (r.Rec[i].Flag <= SympR) && (cp.Rec[i].Flag >= SympU) && (cp.Rec[i].Flag <= SympR) {
			if !r.Rec[i].Obs.Equal(cp.Rec[i].Obs) {
				return true
			}
			if !r.Rec[i].Fill.Equal(cp.Rec[i].Fill) {
				return true
			}
			continue
		}

		if r.Rec[i].Flag != cp.Rec[i].Flag {
			return true
		}
	}
	return false
}

// Randomize randomizes the reconstruction using the indicated probability (as
// percentage).
func (r *Recons) Randomize(prob int, evs []int) {
	if prob == 0 {
		return
	}
	if (prob < 0) || (prob > 100) {
		prob = 10
	}
	for i := range r.Rec {
		if r.Rec[i].SetL == -1 {
			continue
		}
		if rand.Intn(100) > prob {
			continue
		}
		j := rand.Intn(len(evs))
		r.Rec[i].Flag = evs[j]
		r.DownPass(i)
	}
}

// Copy copies the content of cp into reconstruction r.
func (r *Recons) Copy(cp *Recons) {
	if r.Tree != cp.Tree {
		panic("copy can only be made on a reconstruction of the same tree")
	}
	if r.Raster != cp.Raster {
		panic("copy can only be made on a reconstruction with the same raster")
	}
	r.ID = cp.ID
	r.UseLen = cp.UseLen
	r.Size = cp.Size
	r.VicC = cp.VicC
	r.SympC = cp.SympC
	r.FoundC = cp.FoundC
	r.PointC = cp.PointC
	r.SympSize = cp.SympSize

	for i := range cp.Rec {
		copy(r.Rec[i].Obs, cp.Rec[i].Obs)
		copy(r.Rec[i].Fill, cp.Rec[i].Fill)
		r.Rec[i].SetL = cp.Rec[i].SetL
		r.Rec[i].SetR = cp.Rec[i].SetR
		r.Rec[i].Cost = cp.Rec[i].Cost
		r.Rec[i].Flag = cp.Rec[i].Flag
	}
}

// Write writes a reconstruction in csv format on a given output stream. If
// header is false, no header will be printed.
func (r *Recons) Write(out io.Writer, header bool) error {
	w := csv.NewWriter(out)
	w.Comma = '\t'
	w.UseCRLF = true
	defer w.Flush()
	if header {
		err := w.Write([]string{"Tree", "ID", "Node", "Event", "Set"})
		if err != nil {
			return err
		}
	}
	for i := range r.Rec {
		if r.Rec[i].Node.First == nil {
			continue
		}
		e := "*"
		switch r.Rec[i].Flag {
		case Vic:
			e = "v"
		case SympU, SympL, SympR:
			e = "s"
		case PointL, PointR:
			e = "p"
		case FoundL, FoundR:
			e = "f"
		}
		dv := "*"
		switch r.Rec[i].Flag {
		case SympL, PointR, FoundR:
			dv = r.Rec[r.Rec[i].SetL].Node.ID
		case SympR, PointL, FoundL:
			dv = r.Rec[r.Rec[i].SetR].Node.ID
		}
		err := w.Write([]string{r.Tree.ID, r.ID, r.Rec[i].Node.ID, e, dv})
		if err != nil {
			return err
		}
	}
	return nil
}

// DownPass optimize the path from node n to root.
func (r *Recons) DownPass(n int) float64 {
	for v := r.Rec[n].Node; v != nil; v = v.Anc {
		r.optimize(v.Index)
	}
	return r.Rec[0].Cost
}

// optimize recalculates the ancestral reconstruction and the cost of a node.
func (r *Recons) optimize(n int) {
	// ignore terminals
	if r.Rec[n].Node.First == nil {
		return
	}

	// reset the node data
	r.Rec[n].Obs.Reset()
	r.Rec[n].Fill.Reset()
	r.Rec[n].Cost = 0

	// if the node is not optimizable, just add the descendants
	if (r.Rec[n].SetL == -1) || (r.Rec[n].Flag == Undef) {
		for desc := r.Rec[n].Node.First; desc != nil; desc = desc.Sister {
			r.Rec[n].Obs.Union(r.Rec[desc.Index].Obs)
			r.Rec[n].Fill.Union(r.Rec[desc.Index].Fill)
			r.Rec[n].Cost += r.Rec[desc.Index].Cost
		}
		return
	}

	// assign the distribution and the cost of the node
	setL, setR := r.Rec[n].SetL, r.Rec[n].SetR
	cost := r.Rec[setL].Cost + r.Rec[setR].Cost
	switch r.Rec[n].Flag {
	case Vic:
		copy(r.Rec[n].Obs, r.Rec[setL].Obs)
		copy(r.Rec[n].Fill, r.Rec[setL].Fill)
		r.Rec[n].Obs.Union(r.Rec[setR].Obs)
		r.Rec[n].Fill.Union(r.Rec[setR].Fill)
		cost += r.vicariance(n)
	case SympU:
		copy(r.Rec[n].Obs, r.Rec[setL].Obs)
		copy(r.Rec[n].Fill, r.Rec[setL].Fill)
		r.Rec[n].Obs.Union(r.Rec[setR].Obs)
		r.Rec[n].Fill.Union(r.Rec[setR].Fill)
		cost += r.sympatry(n)
	case SympL:
		// In left sympatry, the ancestor is equal to left descendant
		copy(r.Rec[n].Obs, r.Rec[setL].Obs)
		copy(r.Rec[n].Fill, r.Rec[setL].Fill)
		cost += r.sympatry(n)
	case SympR:
		// In right sumpatry, the ancestor is equal to rigth
		// descendant
		copy(r.Rec[n].Obs, r.Rec[setR].Obs)
		copy(r.Rec[n].Fill, r.Rec[setR].Fill)
		cost += r.sympatry(n)
	case PointL:
		// setL is a point inside a setR-exact ancestor
		copy(r.Rec[n].Obs, r.Rec[setR].Obs)
		copy(r.Rec[n].Fill, r.Rec[setR].Fill)
		cost += r.point(n, setL)
	case PointR:
		// setR is a point inside a set setL-exact ancestor
		copy(r.Rec[n].Obs, r.Rec[setL].Obs)
		copy(r.Rec[n].Fill, r.Rec[setL].Fill)
		cost += r.point(n, setR)
	case FoundL:
		// setL is a founder outside a setR-exact ancestor
		copy(r.Rec[n].Obs, r.Rec[setR].Obs)
		copy(r.Rec[n].Fill, r.Rec[setR].Fill)
		cost += r.founder(n, setL)
	case FoundR:
		// setR is a founder outside a setL-exact ancestor
		copy(r.Rec[n].Obs, r.Rec[setL].Obs)
		copy(r.Rec[n].Fill, r.Rec[setL].Fill)
		cost += r.founder(n, setR)
	}
	if r.Size > 0 {
		csz := (float64(r.Rec[n].Obs.Count()-1) / r.Size)
		if r.UseLen {
			csz *= r.Rec[n].Node.Len
		}
		cost += csz
	}
	r.Rec[n].Cost = cost
}

// founder calculates the cost of a founder event in which of the descendants
// is identical to n (its ancestor), and the other (f) starts as a poinst (a
// founder) outside the ancestral distribution.
func (r *Recons) founder(n, f int) float64 {
	comF := r.Rec[f].Obs.Common(r.Rec[n].Fill)
	cellF := r.Rec[f].Obs.Count()
	onlyF := cellF - comF

	if onlyF == 0 {
		cellF += 2
	}
	cost := float64(cellF + comF - 1)
	if r.UseLen {
		cost = cost / r.Rec[f].Node.Len
	}
	return cost + r.FoundC
}

// point calculates the cost of a point sympatry event in which one of the
// descendants is identical to n (its ancestor), and the other (p) starts as
// a point inside the ancestral distribution.
func (r *Recons) point(n, p int) float64 {
	comP := r.Rec[p].Obs.Common(r.Rec[n].Fill)
	cellP := r.Rec[p].Obs.Count()
	onlyP := cellP - comP

	if comP == 0 {
		cellP += 2
	}
	cost := float64(cellP + onlyP - 1)
	if r.UseLen {
		cost = cost / r.Rec[p].Node.Len
	}
	return cost + r.PointC
}

// sympatry calculates the cost of full sympatry.
func (r *Recons) sympatry(n int) float64 {
	setL := r.Rec[n].SetL
	setR := r.Rec[n].SetR
	cellN := r.Rec[n].Obs.Count()

	cellL := r.Rec[setL].Obs.Count()
	onlyL := cellL - r.Rec[setL].Obs.Common(r.Rec[n].Fill)
	notL := cellN - r.Rec[n].Obs.Common(r.Rec[setL].Fill)
	costL := float64(onlyL + notL)

	cellR := r.Rec[setR].Obs.Count()
	onlyR := cellR - r.Rec[setR].Obs.Common(r.Rec[n].Fill)
	notR := cellN - r.Rec[n].Obs.Common(r.Rec[setR].Fill)
	costR := float64(onlyR + notR)

	if r.UseLen {
		costL = costL / r.Rec[setL].Node.Len
		costR = costR / r.Rec[setR].Node.Len
	}

	var extra float64
	if r.SympSize > 0 {
		extra = float64(r.Rec[n].Obs.Count()) / r.SympSize
	}

	return costL + costR + r.SympC + extra
}

// vicariance calculates the cost of a disjunct set.
func (r *Recons) vicariance(n int) float64 {
	setL := r.Rec[n].SetL
	setR := r.Rec[n].SetR

	cellL := r.Rec[setL].Obs.Count()
	comL := r.Rec[setL].Obs.Common(r.Rec[setR].Fill)
	onlyL := cellL - comL

	cellR := r.Rec[setR].Obs.Count()
	comR := r.Rec[setR].Obs.Common(r.Rec[setL].Fill)
	onlyR := cellR - comR

	if (onlyL == 0) || (onlyR == 0) {
		if onlyL != 0 {
			// r is cointained in l.
			comR += 1
		} else if onlyR != 0 {
			// l is contained in r.
			comL += 1
		} else {
			// both sets are identical
			comL += 1
			comR += 1
		}
	}

	costL := float64(comL)
	costR := float64(comR)
	if r.UseLen {
		costL = costL / r.Rec[setL].Node.Len
		costR = costR / r.Rec[setR].Node.Len
	}

	return costL + costR + r.VicC
}

// Eval store the evaluation of a given reconstruction.
type Eval struct {
	Vics  int // Number of vicariant events
	Symp  int // Number of sympatry events
	Point int // Number of punctual sympatry events
	Found int // Number of founder events
}

// Evaluate returns an evaluation of a reconstruction.
func (r *Recons) Evaluate() Eval {
	var e Eval
	for i := range r.Rec {
		if r.Rec[i].Node.First != nil {
			switch r.Rec[i].Flag {
			case Vic:
				e.Vics++
			case SympU, SympL, SympR:
				e.Symp++
			case PointL, PointR:
				e.Point++
			case FoundL, FoundR:
				e.Found++
			}
		}
	}
	return e
}
