// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/js-arias/evs/cmdapp"
)

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
