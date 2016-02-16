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

// AddFounder adds founder event to a set of events.
func AddFounder(events []int) []int {
	if HasFounder(events) {
		return events
	}
	return append(events, FoundL, FoundR)
}

// AddPoint adds point sympatry to a set of events.
func AddPoint(events []int) []int {
	if HasPoint(events) {
		return events
	}
	return append(events, PointL, PointR)
}

// AddSymp adds full sympatry to a set of events.
func AddSymp(events []int) []int {
	if HasSymp(events) {
		return events
	}
	return append(events, SympU, SympL, SympR)
}

// AddVic adds vicariance to a set of events.
func AddVic(events []int) []int {
	if HasVic(events) {
		return events
	}
	return append(events, Vic)
}

// HasFounder returns true if a set of events include founder event.
func HasFounder(events []int) bool {
	l, r := false, false
	for _, e := range events {
		switch e {
		case FoundL:
			l = true
		case FoundR:
			r = true
		}
		if l && r {
			return true
		}
	}
	return false
}

// HasPoint returns true if a set of events include punctual sympatry.
func HasPoint(events []int) bool {
	l, r := false, false
	for _, e := range events {
		switch e {
		case PointL:
			l = true
		case PointR:
			r = true
		}
		if l && r {
			return true
		}
	}
	return false
}

// HasSymp returns true if a set of events include sympatry.
func HasSymp(events []int) bool {
	l, r, u := false, false, false
	for _, e := range events {
		switch e {
		case SympU:
			u = true
		case SympL:
			l = true
		case SympR:
			r = true
		}
		if u && l && r {
			return true
		}
	}
	return false
}

// HasVic returns true if a set of events include vicariance.
func HasVic(events []int) bool {
	for _, e := range events {
		if e == Vic {
			return true
		}
	}
	return false
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
	Size   int
}

// OR creates an OR reconstruction based on raster and tree data. If scaled is
// true, then the cost at each node will be scaled by the ancestral
// distribution.
func OR(r *raster.Raster, t *tree.Tree, size int) *Recons {
	or := &Recons{
		ID:     "or",
		Tree:   t,
		Raster: r,
		Rec:    make([]Node, len(t.Nodes)),
		Size:   size,
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
			sz := or.Rec[i].Obs.Count()
			cs += float64(sz) / float64(or.Size)
			cv += float64(sz) / float64(or.Size)
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

// Read reads a reconstruction from one or most trees in csv format from an
// input stream.
func Read(in io.Reader, ras *raster.Raster, ts []*tree.Tree, size int) ([]*Recons, error) {
	var recs []*Recons
	r := csv.NewReader(in)
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
			nr = OR(ras, t, size)
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
		ID:     r.ID,
		Tree:   r.Tree,
		Raster: r.Raster,
		Rec:    make([]Node, len(r.Rec)),
		Size:   r.Size,
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
		sz := r.Rec[n].Obs.Count()
		cost += float64(sz) / float64(r.Size)
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
		return float64(2*cellF) + 2
	}
	return float64(cellF + comF)
}

// point calculates the cost of a point sympatry event in which one of the
// descendants is identical to n (its ancestor), and the other (p) starts as
// a point inside the ancestral distribution.
func (r *Recons) point(n, p int) float64 {
	comP := r.Rec[p].Obs.Common(r.Rec[n].Fill)
	cellP := r.Rec[p].Obs.Count()
	onlyP := cellP - comP

	if comP == 0 {
		return float64(2*cellP) + 2
	}
	return float64(cellP + onlyP)
}

// sympatry calculates the cost of full sympatry.
func (r *Recons) sympatry(n int) float64 {
	setL := r.Rec[n].SetL
	setR := r.Rec[n].SetR
	cellN := r.Rec[n].Obs.Count()

	cellL := r.Rec[setL].Obs.Count()
	onlyL := cellL - r.Rec[setL].Obs.Common(r.Rec[n].Fill)
	notL := cellN - r.Rec[n].Obs.Common(r.Rec[setL].Fill)

	cellR := r.Rec[setR].Obs.Count()
	onlyR := cellR - r.Rec[setR].Obs.Common(r.Rec[n].Fill)
	notR := cellN - r.Rec[n].Obs.Common(r.Rec[setR].Fill)

	return float64(onlyL + notL + onlyR + notR)
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

	cost := float64(comL + comR)
	if (onlyL == 0) || (onlyR == 0) {
		if onlyL != 0 {
			// r is cointained in l.
			cost += 1
		} else if onlyR != 0 {
			// l is contained in r.
			cost += 1
		} else {
			// both sets are identical
			cost += 2
		}
	}
	return cost
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
