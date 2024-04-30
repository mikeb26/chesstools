package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/notnil/chess"
)

type FenCatOpts struct {
	dark bool
	fens []string
}

func parseArgs(opts *FenCatOpts) error {
	opts.fens = make([]string, 0)
	f := flag.NewFlagSet("fencat", flag.ExitOnError)

	f.BoolVar(&opts.dark, "dark", false, "<true|false>")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	for _, fen := range f.Args() {
		opts.fens = append(opts.fens, fen)
	}

	return nil
}

func main() {
	var opts FenCatOpts
	err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		os.Exit(1)
		return
	}

	if len(opts.fens) == 0 {
		return
	}

	for _, fen := range opts.fens {
		err = processOneFen(&opts, fen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
}

func processOneFen(opts *FenCatOpts, fen string) error {
	fenCheck, err := chess.FEN(fen)
	if err != nil {
		return err
	}
	g := chess.NewGame(fenCheck)
	p := g.Position()
	b := p.Board()

	fmt.Printf("%v", b.Draw2(p.Turn(), opts.dark))

	return nil
}
