package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type Pgn2FenOpts struct {
	all      bool
	pgnFiles []string
}

func parseArgs(opts *Pgn2FenOpts) error {

	opts.pgnFiles = make([]string, 0)
	f := flag.NewFlagSet("pgn2fen", flag.ExitOnError)

	f.BoolVar(&opts.all, "all", false, "<true|false>")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	for _, pgnFile := range f.Args() {
		opts.pgnFiles = append(opts.pgnFiles, pgnFile)
	}

	return nil
}

func main() {
	var opts Pgn2FenOpts
	err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		os.Exit(1)
		return
	}

	if len(opts.pgnFiles) == 0 {
		pgn, err := chess.PGN(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

		g := chess.NewGame(pgn)

		processOneGame(&opts, g)

		return
	}

	for _, pgnFile := range opts.pgnFiles {
		err = processOnePgn(&opts, pgnFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
}

func processOnePgn(opts *Pgn2FenOpts, pgnFile string) error {
	f, err := chesstools.OpenPgn(pgnFile)
	if err != nil {
		return err
	}
	defer f.Close()

	var scanOpts chess.ScannerOpts
	scanOpts.ExpandVariations = true

	scanner := chess.NewScannerWithOptions(f, scanOpts)

	ii := 1
	for scanner.Scan() {
		g := scanner.Next()
		if len(g.Moves()) == 0 {
			continue
		}
		processOneGame(opts, g)
		ii++
	}

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return err
}

func processOneGame(opts *Pgn2FenOpts, g *chess.Game) {
	if !opts.all {
		fmt.Printf("%v\n", g.Position().XFENString())
		return
	} // else

	for _, pos := range g.Positions() {
		fmt.Printf("%v\n", pos.XFENString())
	}
}
