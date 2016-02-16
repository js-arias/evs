// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/js-arias/evs/cmdapp"
	"github.com/js-arias/evs/tree"
)

var trIn = &cmdapp.Command{
	Run:       trInRun,
	UsageLine: `tr.in [-i|--input file] tree-id`,
	Short:     "import a parenthetical tree",
	Long: `
Tr.in reads a tree in parenthetical notation, assuming that each terminal is
stored by its name (underline character will be transformed into space), and
prints in as a cvs table that contains the tree.

The output file, has the following columns:
	Tree		Tree identifier
	Node		Node identifier
	Ancestor	Identifier of the ancestor of the node
	Terminal	The name of the terminal taxon
The table must be sorted in a form that each node is read only after its
ancestor was already readed.

By default, the tree is read from the standard input. The tree must be in
parenthetical format, without any header (such as 'tread' or 'tree ='). Only
one tree will be read from the input.

Options are:

    -i file
    --input file
      If defined, the tree in parenthetical format will read from the
      indicated file, instead of the standard input.

    tree-id
      Set the id of the tree.
	`,
}

func init() {
	trIn.Flag.StringVar(&inFile, "input", "", "")
	trIn.Flag.StringVar(&inFile, "i", "", "")
	trIn.Run = trInRun
}

func trInRun(c *cmdapp.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: expecting tree ID\n", c.Name())
		os.Exit(1)
	}
	var ts []*tree.Tree
	if _, err := os.Stat(treeFileName); err == nil {
		ts, err = loadTrees()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
	}
	f := os.Stdin
	if len(inFile) != 0 {
		var err error
		f, err = os.Open(inFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		defer f.Close()
	}
	for _, t := range ts {
		if t.ID == args[0] {
			fmt.Fprintf(os.Stderr, "%s: tree ID already used\n", c.Name())
			os.Exit(1)
		}
	}

	// reads the tree
	in := os.Stdin
	if len(inFile) != 0 {
		var err error
		in, err = os.Open(inFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		defer in.Close()
	}
	t, err := tree.ReadParenthetic(in, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	ts = append(ts, t)

	// writes the trees into the database
	out, err := os.Create(treeFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	defer out.Close()
	head := true
	for _, v := range ts {
		err = v.Write(out, head)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
			os.Exit(1)
		}
		head = false
	}
}

var trLs = &cmdapp.Command{
	Run:       trLsRun,
	UsageLine: "tr.ls",
	Short:     "list trees",
	Long:      "Tr.ls list all trees in the 'trees.csv' file.",
}

func trLsRun(c *cmdapp.Command, args []string) {
	ls, err := loadTrees()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	for _, t := range ls {
		fmt.Printf("%s\n", t.ID)
	}
}
