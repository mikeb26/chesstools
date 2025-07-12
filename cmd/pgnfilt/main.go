/* Utility for filtering large pgn files */

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/corentings/chess/v2"
	"github.com/mikeb26/chesstools"
)

type FiltOpts struct {
	fen string
}

type FiltCtx struct {
	pgnFileList []string
	fopts       FiltOpts
	expandVar   bool
}

func NewFiltCtx(pgns []string, foptsIn FiltOpts,
	expandVarIn bool) *FiltCtx {

	rv := &FiltCtx{
		pgnFileList: make([]string, len(pgns)),
		fopts:       foptsIn,
		expandVar:   expandVarIn,
	}
	for ii, p := range pgns {
		rv.pgnFileList[ii] = p
	}

	return rv
}

func main() {
	fopts := FiltOpts{}
	expandVar := false
	pgnList, err := parseArgs(&fopts, &expandVar)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	filtCtx := NewFiltCtx(pgnList, fopts, expandVar)
	err = filtCtx.LoadAndFilter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load PGN files: %v\n", err)
		return
	}
}

func parseArgs(fopts *FiltOpts, expandVar *bool) ([]string, error) {
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
	var scanner *chess.Scanner
	if filtCtx.expandVar {
		scanner = chess.NewScanner(f, chess.WithExpandVariations())
	} else {
		scanner = chess.NewScanner(f)
	}

	for scanner.HasNext() {
		g, err := scanner.ParseNext()
		if err != nil {
			return err
		}
		if len(g.Moves()) == 0 {
			continue
		}

		err = filtCtx.processOneGame(g)
		if err != nil {
			return err
		}
	}

	return nil
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
