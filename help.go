// Copyright (c) 2015, J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
// All rights reserved.
// Distributed under BSD2 license that can be found in the LICENSE file.

package main

import "github.com/js-arias/evs/cmdapp"

var about = &cmdapp.Command{
	UsageLine: "about",
	Short:     "about Evs",
	Long: `
Evs implements the geographically explicit event model for phylogenetic
biogeography using explicit terminal ranges. The main objective of the
method is to optimize both the ancestral distribution and  the biogeographic
event associated with each node.

Author:
    J. Salvador Arias <jsalarias@csnat.unt.edu.ar>
    CONICET, UEL, Facultad de Ciencias Naturales e Instituto Miguel Lillo,
    Universidad Nacional de Tucumán, Miguel Lillo 205, S.M. de Tucumán (4000),
    Tucumán, Argentina

Please report any bug to J.S. Arias <jsalarias@csnat.unt.edu.ar>.
	`,
}

var recordsHelp = &cmdapp.Command{
	UsageLine: "records",
	Short:     "taxon-records file",
	Long: `
In Evs the taxon and records input is from a file is called 'records.csv'. The
file must have at least the following columns:

    Name
      Stores the name of the taxon. This column can be also named
      'Scientific name'.

    Longitude
    Latitude
      The goegraphic position of each record is stored in these columns.

Optionally it can include the column 'Catalog' for a reference of the catalog
code (or any other record identifier) of each record.
	`,
}

var treesHelp = &cmdapp.Command{
	UsageLine: "trees",
	Short:     "trees file",
	Long: `
In Evs the tree data is stored in a file called 'trees.csv'. The file has at
least the following columns:

    Tree
      Tree identifier

    Node
      Node identifier

    Ancestor
      Identifier of the ancestor of the node

    Terminal
      The name of the terminal taxon

The table must be sorted in a form that each node is read only after its
ancestor was already readed.
	`,
}
