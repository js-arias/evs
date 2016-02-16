// Copyright (c) 2015, J. Salvador Arias <jsalarias@gmail.com>
// All rights reserved.
// Distributed under BSD-style license that can be found in the LICENSE file.
//
// This work is derived from the go tool source code
// Copyright 2011 The Go Authors.  All rights reserved.

// Package cmdapp implements a command line application that host a set of
// commands as in the go tool or git.
package cmdapp

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Short is a short description of the application.
var Short string

// Commands is the list of available commands and help topics. The order in
// the list is used for help output.
var Commands []*Command

// Common exit codes.
const (
	ExitOk   = iota // success
	ExitErr         // any processing error
	ExitArgs        // invalid arguments
	ExitIn          // error while reading input
	ExitOut         // error while writing output
)

// Name returns the application's name, from the argument list.
func Name() string {
	return os.Args[0]
}

// Run runs the application.
func Run() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}
	if args[0] == "help" {
		help(args[1:])
		return
	}

	for _, c := range Commands {
		if (c.Name() == args[0]) && (c.Run != nil) {
			c.Flag.Usage = func() { c.Usage() }
			c.Flag.Parse(args[1:])
			c.Run(c, c.Flag.Args())
			return
		}
	}

	fmt.Fprintf(os.Stderr, "%s: unknown subcommand %s\nRun '%s help' for usage.\n", Name(), Name())
	os.Exit(ExitArgs)
}

// usage prints application's help and exits.
func usage() {
	printUsage(os.Stderr)
	os.Exit(ExitArgs)
}

// printUsage outputs the application usage help.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "%s\n\n", Short)
	fmt.Fprintf(w, "Usage:\n\n    %s [help] <command> [<args>...]\n\n", Name())
	fmt.Fprintf(w, "The commands are:\n")
	for _, c := range Commands {
		if c.Run == nil {
			continue
		}
		fmt.Fprintf(w, "    %-16s %s\n", c.Name(), c.Short)
	}
	fmt.Fprintf(w, "\nUse '%s help <command>' for more information about a command.\n\n", Name())
	fmt.Fprintf(w, "\nAdditional help topics:\n\n")
	for _, c := range Commands {
		if c.Run != nil {
			continue
		}
		fmt.Fprintf(w, "    %-16s %s\n", c.Name(), c.Short)
	}
	fmt.Fprintf(w, "\nUse '%s help <topic>' for more information about that topic.\n\n", Name())
}

// help implements the 'help' command.
func help(args []string) {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "%s: help: too many arguments.\nusage: '%s help <command>'\n", Name(), Name())
		os.Exit(ExitArgs)
	}

	arg := args[0]

	// 'help documentation' generates doc.go.
	if arg == "documentation" {
		f, err := os.Create("doc.go")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: help: %v\n", Name(), err)
			os.Exit(ExitOut)
		}
		defer f.Close()
		fmt.Fprintf(f, "%s", goHead)
		printUsage(f)
		for _, c := range Commands {
			c.documentation(f)
		}
		fmt.Fprintf(f, "%s", goFoot)
		return
	}

	for _, c := range Commands {
		if c.Name() == arg {
			c.help()
			return
		}
	}

	fmt.Fprintf(os.Stderr, "%s: help: unknown help topic.\nRun '%s help'.\n", Name(), Name())
	os.Exit(ExitArgs)
}

var goHead = `// Authomatically generated doc.go file for use with godoc.

/*
`

var goFoot = `
*/
package main`
