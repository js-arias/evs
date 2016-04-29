// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/js-arias/evs/cmdapp"
)

var txLs = &cmdapp.Command{
	Run:       txLsRun,
	UsageLine: "tx.ls",
	Short:     "list terminal taxons",
	Long:      "Tx.ls list all terminals in the 'records.csv' file.",
}

func txLsRun(c *cmdapp.Command, args []string) {
	d, err := loadData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", c.Name(), err)
		os.Exit(1)
	}
	for _, tx := range d.Ls {
		fmt.Printf("%s\n", tx.Name)
	}
}
