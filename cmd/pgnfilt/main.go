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
	fen string
}

type FiltCtx struct {
	pgnFileList []string
	fopts       FiltOpts
	sopts       chess.ScannerOpts
}

func NewFiltCtx(pgns []string, foptsIn FiltOpts,
	soptsIn chess.ScannerOpts) *FiltCtx {

	rv := &FiltCtx{
		pgnFileList: make([]string, len(pgns)),
		fopts:       foptsIn,
		sopts:       soptsIn,
	}
	for ii, p := range pgns {
		rv.pgnFileList[ii] = p
	}

	return rv
}

func main() {
	fopts := FiltOpts{}
	sopts := chess.ScannerOpts{}
	pgnList, err := parseArgs(&fopts, &sopts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	filtCtx := NewFiltCtx(pgnList, fopts, sopts)
	err = filtCtx.LoadAndFilter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load PGN files: %v\n", err)
		return
	}
}

func parseArgs(fopts *FiltOpts, sopts *chess.ScannerOpts) ([]string, error) {
	f := flag.NewFlagSet("pgnfilt", flag.ExitOnError)

	f.StringVar(&fopts.fen, "fen", "", "includes this specific position")
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
	scanner := chess.NewScannerWithOptions(f, filtCtx.sopts)

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
	if !filtCtx.filterMatches(g) {
		return nil
	}

	fmt.Printf("%v\n\n\n", g.String())

	return nil
}

func (filtCtx *FiltCtx) filterMatches(g *chess.Game) bool {
	isMatch := true

	if filtCtx.fopts.fen != "" {
		isMatch = false

		for _, p := range g.Positions() {
			foptsFen, _ := chesstools.NormalizeFEN(filtCtx.fopts.fen)
			gameFen, _ := chesstools.NormalizeFEN(p.XFENString())
			if foptsFen == gameFen {
				isMatch = true
				break
			}
		}
	}

	return isMatch
}
