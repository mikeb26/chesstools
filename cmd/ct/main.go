package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"

	gen960 "github.com/mikeb26/chesstools/cmd/ct/960gen"
	"github.com/mikeb26/chesstools/cmd/ct/eval"
	"github.com/mikeb26/chesstools/cmd/ct/fencat"
	"github.com/mikeb26/chesstools/cmd/ct/pgn2fen"
	"github.com/mikeb26/chesstools/cmd/ct/pgnfilt"
	"github.com/mikeb26/chesstools/cmd/ct/pgnmk"
	"github.com/mikeb26/chesstools/cmd/ct/repmk"
	"github.com/mikeb26/chesstools/cmd/ct/repvld"
	"github.com/mikeb26/chesstools/cmd/ct/splunk"
)

type command struct {
	name        string
	description string
	run         func([]string)
}

var commands = []command{
	{name: "960gen", description: "print Chess960 start FENs", run: gen960.Main},
	{name: "eval", description: "evaluate a FEN or PGN position", run: eval.Main},
	{name: "splunk", description: "find players who have had positions", run: splunk.Main},
	{name: "fencat", description: "render FENs as ASCII boards", run: fencat.Main},
	{name: "pgn2fen", description: "convert PGNs to FENs", run: pgn2fen.Main},
	{name: "pgnfilt", description: "filter PGN files", run: pgnfilt.Main},
	{name: "pgnmk", description: "interactively create PGNs", run: pgnmk.Main},
	{name: "repmk", description: "build opening repertoires", run: repmk.Main},
	{name: "repvld", description: "validate opening repertoires", run: repvld.Main},
}

func main() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	if len(os.Args) < 2 {
		printUsage(os.Stdout)
		return
	}

	arg1 := os.Args[1]
	switch arg1 {
	case "-h", "--help":
		printUsage(os.Stdout)
		return
	case "help":
		if len(os.Args) == 2 {
			printUsage(os.Stdout)
			return
		}

		cmdName := os.Args[2]
		cmd, ok := lookupCommand(cmdName)
		if !ok {
			fmt.Fprintf(os.Stderr, "ct: unknown command %q\n", cmdName)
			os.Exit(1)
		}

		cmd.run([]string{"--help"})
		return
	}

	cmd, ok := lookupCommand(arg1)
	if !ok {
		fmt.Fprintf(os.Stderr, "ct: unknown command %q\n", arg1)
		printUsage(os.Stderr)
		os.Exit(1)
	}

	cmd.run(os.Args[2:])
}

func lookupCommand(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}

	return command{}, false
}

func printUsage(w io.Writer) {
	cmds := make([]command, len(commands))
	copy(cmds, commands)
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })

	fmt.Fprintln(w, "usage: ct <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "available commands:")
	for _, cmd := range cmds {
		fmt.Fprintf(w, "  %-8s %s\n", cmd.name, cmd.description)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "use 'ct help <command>' or 'ct <command> --help' for command-specific help")
}
