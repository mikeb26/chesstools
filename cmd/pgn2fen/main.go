package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type Pgn2FenOpts struct {
	all      bool
	color    string
	colorc   chess.Color
	pgnFiles []string
}

func parseArgs(opts *Pgn2FenOpts) error {

	opts.pgnFiles = make([]string, 0)
	f := flag.NewFlagSet("pgn2fen", flag.ExitOnError)

	opts.colorc = chess.NoColor
	f.BoolVar(&opts.all, "all", false, "<true|false>")
	f.StringVar(&opts.color, "color", "", "<white|black>")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if opts.color != "" && opts.all {
		return fmt.Errorf("--all and --color are mutually exclusive")
	}
	if opts.color != "" {
		if strings.ToLower(opts.color) == "white" {
			opts.colorc = chess.White
		} else if strings.ToLower(opts.color) == "black" {
			opts.colorc = chess.Black
		} else {
			return fmt.Errorf("color must be white | black")
		}
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
	if !opts.all && opts.colorc == chess.NoColor {
		fmt.Printf("%v\n", g.Position().XFENString())
		return
	} // else

	for _, pos := range g.Positions() {
		if opts.colorc == chess.NoColor || opts.colorc == pos.Turn() {
			fmt.Printf("%v\n", pos.XFENString())
		}
	}
}
