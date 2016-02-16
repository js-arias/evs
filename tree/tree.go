// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package tree

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// A Node is a node of a phylogenetic tree.
type Node struct {
	Index  int
	ID     string
	Anc    *Node
	Sister *Node
	First  *Node
	Term   string
}

// A Tree is a phylogenetic tree.
type Tree struct {
	ID    string
	Root  *Node
	Nodes []*Node
}

// Read reads one or more trees in csv format from an input stream.
func Read(in io.Reader) ([]*Tree, error) {
	var tr []*Tree
	r := csv.NewReader(in)
	r.TrimLeadingSpace = true

	// reads the file header
	h, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("header (tree): %v", err)
	}
	tree := -1
	node := -1
	anc := -1
	term := -1
	for i, v := range h {
		switch strings.ToLower(v) {
		case "tree":
			tree = i
		case "node id", "node":
			node = i
		case "ancestor", "anc", "parent":
			anc = i
		case "term", "terminal", "termname":
			term = i
		}
	}
	if (tree < 0) || (node < 0) || (anc < 0) || (term < 0) {
		return nil, errors.New("header (tree): incomplete header")
	}

	// reads the data
	// map of tree-id:tree
	tids := make(map[string]*Tree)
	// map of tree-id:map of node-id:node-index
	tn := make(map[string]map[string]int)
	for i := 1; ; i++ {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("(tree) row %d: %v", i, err)
		}
		if lr := len(row); (lr <= tree) || (lr <= node) || (lr <= anc) || (lr <= term) {
			continue
		}
		if (len(row[tree]) == 0) || (len(row[node]) == 0) {
			continue
		}
		t, ok := tids[row[tree]]
		if !ok {
			t = &Tree{ID: row[tree]}
			tids[row[tree]] = t
			tr = append(tr, t)
			tn[row[tree]] = make(map[string]int)
		}
		ids := tn[row[tree]]
		if _, ok := ids[row[node]]; ok {
			return nil, fmt.Errorf("(tree) row %d: node id %s (tree %s) repeated", i, row[node], row[tree])
		}
		if (len(row[anc]) == 0) || (row[anc] == "-1") || (row[anc] == "xx") {
			if t.Root != nil {
				return nil, fmt.Errorf("(tree) row %d: node without parent", i)
			}
			n := &Node{
				Index: len(t.Nodes),
				ID:    row[node],
			}
			t.Root = n
			ids[n.ID] = n.Index
			t.Nodes = append(t.Nodes, n)
			continue
		}
		av, ok := ids[row[anc]]
		if !ok {
			return nil, fmt.Errorf("(tree) row %d: ancestor %s of node id %s (tree %s) not found", i, row[anc], row[node], row[tree])
		}
		a := t.Nodes[av]
		if len(a.Term) > 0 {
			return nil, fmt.Errorf("(tree) row %d: ancestor %s of node id %s (tree %s) is a terminal", i, row[anc], row[node], row[tree])
		}
		var tx string
		if len(row[term]) > 0 {
			tx = strings.Join(strings.Fields(row[term]), " ")
		}
		n := &Node{
			Index: len(t.Nodes),
			ID:    row[node],
			Anc:   a,
			Term:  tx,
		}
		if a.First != nil {
			for d := a.First; ; d = d.Sister {
				if d.Sister == nil {
					d.Sister = n
					break
				}
			}
		} else {
			a.First = n
		}
		ids[n.ID] = n.Index
		t.Nodes = append(t.Nodes, n)
	}
	return tr, nil
}

// Write writes a tree as csv into an output stream. If header is false, it
// will not print the column names (the header).
func (t *Tree) Write(out io.Writer, header bool) error {
	w := csv.NewWriter(out)
	defer w.Flush()
	if header {
		err := w.Write([]string{"Tree", "Node", "Ancestor", "Terminal"})
		if err != nil {
			return err
		}
	}
	for _, n := range t.Nodes {
		anc := "-1"
		if n.Anc != nil {
			anc = n.Anc.ID
		}
		rec := []string{
			t.ID,
			n.ID,
			anc,
			n.Term,
		}
		err := w.Write(rec)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadParenthetic reads a single tree in parenthetical format.
func ReadParenthetic(in io.Reader, id string) (*Tree, error) {
	r := bufio.NewReader(in)
	for {
		r1, _, err := r.ReadRune()
		if err != nil {
			return nil, fmt.Errorf("(tree): %v", err)
		}
		if r1 == '(' {
			break
		}
	}
	t := &Tree{ID: id}
	n, err := t.readNode(r, nil)
	if err != nil {
		return nil, fmt.Errorf("(tree): %v", err)
	}
	t.Root = n
	return t, nil
}

// readNode reads a node in parenthetical format.
func (t *Tree) readNode(r *bufio.Reader, anc *Node) (*Node, error) {
	n := &Node{
		Index: len(t.Nodes),
		ID:    strconv.FormatInt(int64(len(t.Nodes)), 10),
		Anc:   anc,
	}
	t.Nodes = append(t.Nodes, n)
	num := 0
	var last, desc *Node
	for {
		r1, _, err := r.ReadRune()
		if err != nil {
			return nil, err
		}
		if unicode.IsSpace(r1) {
			continue
		}
		if r1 == ',' {
			continue
		}
		if r1 == ':' {
			err = skipLen(r)
			if err != nil {
				return nil, err
			}
			continue
		}
		if r1 == '(' {
			desc, err = t.readNode(r, n)
			if err != nil {
				return nil, err
			}
			num++
			if last != nil {
				last.Sister = desc
			} else {
				n.First = desc
			}
			last = desc
			continue
		}
		if r1 == ')' {
			break
		}
		// a terminal
		r.UnreadRune()
		tx, err := readTerm(r)
		if err != nil {
			return nil, err
		}
		desc = &Node{
			Index: len(t.Nodes),
			ID:    strconv.FormatInt(int64(len(t.Nodes)), 10),
			Anc:   n,
			Term:  tx,
		}
		t.Nodes = append(t.Nodes, desc)
		num++
		if last != nil {
			last.Sister = desc
		} else {
			n.First = desc
		}
		last = desc
	}
	if num < 2 {
		return nil, fmt.Errorf("node %d with too few descendants", num)
	}
	return n, nil
}

// readTerm reads a terminal from a tree string.
func readTerm(r *bufio.Reader) (string, error) {
	r1, _, _ := r.ReadRune()
	if r1 == '\'' {
		return readBlock(r, '\'')
	}
	r.UnreadRune()
	var nm []rune
	first := true
	space := false
	for {
		r1, _, err := r.ReadRune()
		if err != nil {
			return "", err
		}
		if unicode.IsSpace(r1) || (r1 == ',') {
			break
		}
		if (r1 == '(') || (r1 == ')') || (r1 == ':') {
			r.UnreadRune()
			break
		}
		if r1 == '_' {
			space = true
			continue
		}
		if space {
			if !first {
				nm = append(nm, ' ')
			}
			space = false
		}
		first = false
		nm = append(nm, r1)
	}
	if len(nm) == 0 {
		return "", errors.New("empty taxon name (just underlines)")
	}
	return string(nm), nil
}

// readBlock reads a string inside a block.
func readBlock(r *bufio.Reader, delim rune) (string, error) {
	var s []rune
	first := true
	space := false
	for {
		r1, _, err := r.ReadRune()
		if err != nil {
			return "", err
		}
		if r1 == delim {
			break
		}
		if unicode.IsSpace(r1) {
			space = true
			continue
		}
		if space {
			if !first {
				s = append(s, ' ')
			}
			space = false
		}
		first = false
		s = append(s, r1)
	}
	if len(s) == 0 {
		return "", errors.New("empty block string")
	}
	return string(s), nil
}

// skipLen skips the lenght of the node, if defined.
func skipLen(r *bufio.Reader) error {
	for {
		r1, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		if unicode.IsSpace(r1) || (r1 == ',') {
			break
		}
		if (r1 == '(') || (r1 == ')') {
			r.UnreadRune()
			break
		}
	}
	return nil
}
