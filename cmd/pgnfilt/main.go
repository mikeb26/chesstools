/* Utility for filtering large pgn files */

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

type FiltOpts struct {
	minELO        uint64
	minTimeInSecs uint64
}

type FiltCtx struct {
	pgnFileList []string
	opts        chess.ScannerOpts
}

func NewFiltCtx(pgns []string, optsIn chess.ScannerOpts) *FiltCtx {
	rv := &FiltCtx{
		pgnFileList: make([]string, len(pgns)),
		opts:        optsIn,
	}
	for ii, p := range pgns {
		rv.pgnFileList[ii] = p
	}

	return rv
}

func main() {
	opts := chess.ScannerOpts{}
	pgnList, err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	filtCtx := NewFiltCtx(pgnList, opts)
	err = filtCtx.LoadAndFilter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load PGN files: %v\n", err)
		return
	}
}

func parseArgs(opts *chess.ScannerOpts) ([]string, error) {
	f := flag.NewFlagSet("pgnfilt", flag.ExitOnError)

	//	f.Uint64Var(&opts.MinELO, "minelo", 0, "minimum ELO of both players")
	//	f.Uint64Var(&opts.MinTimeInSecs, "mintime", 0, "minimum clock in seconds")
	f.Parse(os.Args[1:])

	if len(f.Args()) == 0 {
		return nil, fmt.Errorf("please specify 1 or more PGN files to filter")
	}
	return f.Args(), nil
}

func (filtCtx *FiltCtx) LoadAndFilter() error {
	for _, pgnFilename := range filtCtx.pgnFileList {
		f, err := chesstools.OpenPgn(pgnFilename)
		if err != nil {
			return err
		}
		defer f.Close()

		err = filtCtx.processOnePGN(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func (filtCtx *FiltCtx) processOnePGN(f io.Reader) error {
	scanner := chess.NewScannerWithOptions(f, filtCtx.opts)

	var err error
	for scanner.Scan() {
		g := scanner.Next()
		if len(g.Moves()) == 0 {
			continue
		}

		err = filtCtx.processOneGame(g)
		if err != nil {
			return err
		}
	}

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return err
}

func (filtCtx *FiltCtx) processOneGame(g *chess.Game) error {
	fmt.Printf("%v\n", g.String())

	return nil
}
