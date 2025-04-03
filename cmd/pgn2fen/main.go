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

const NoEndMove = 10000

type Pgn2FenOpts struct {
	all          bool
	startMoveNum int
	endMoveNum   int
	color        string
	colorc       chess.Color
	pgnFiles     []string
	expandVar    bool
}

func NewPgn2FenOpts() *Pgn2FenOpts {
	opts := &Pgn2FenOpts{
		all:          false,
		startMoveNum: 0,
		endMoveNum:   NoEndMove,
		color:        "",
		colorc:       chess.NoColor,
		pgnFiles:     make([]string, 0),
		expandVar:    false,
	}

	return opts
}

func parseArgs(opts *Pgn2FenOpts) error {

	opts.pgnFiles = make([]string, 0)
	f := flag.NewFlagSet("pgn2fen", flag.ExitOnError)

	opts.colorc = chess.NoColor
	f.BoolVar(&opts.all, "all", opts.all, "<true|false>")
	f.BoolVar(&opts.expandVar, "includevar", opts.expandVar, "include variations in pgn <true|false>")
	f.StringVar(&opts.color, "color", opts.color, "<white|black>")
	f.IntVar(&opts.startMoveNum, "startmove", opts.startMoveNum,
		"start move number (defaults to 0)")
	f.IntVar(&opts.endMoveNum, "endmove", opts.endMoveNum,
		"ending move number")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if opts.color != "" && opts.all {
		return fmt.Errorf("--all and --color are mutually exclusive")
	}
	if opts.all && opts.startMoveNum != 0 {
		return fmt.Errorf("--all and --startmove are mutually exclusive")
	}
	if opts.all && opts.endMoveNum != NoEndMove {
		return fmt.Errorf("--all and --endmove are mutually exclusive")
	}
	if opts.startMoveNum > opts.endMoveNum {
		return fmt.Errorf("--startmove(%v) must be <= --endmove(%v)",
			opts.startMoveNum, opts.endMoveNum)
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
	opts := NewPgn2FenOpts()

	err := parseArgs(opts)
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

		fens := game2FENs(opts, g)
		fmt.Printf("%v", fens)

		return
	}

	for _, pgnFile := range opts.pgnFiles {
		err = processOnePgn(opts, pgnFile)
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
	scanOpts.ExpandVariations = opts.expandVar

	scanner := chess.NewScannerWithOptions(f, scanOpts)

	ii := 1
	for scanner.Scan() {
		g := scanner.Next()
		if len(g.Moves()) == 0 {
			continue
		}
		fens := game2FENs(opts, g)
		fmt.Printf("%v", fens)
		ii++
	}

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return err
}

func game2FENs(opts *Pgn2FenOpts, g *chess.Game) string {
	if !opts.all && opts.colorc == chess.NoColor && opts.startMoveNum == 0 &&
		opts.endMoveNum == NoEndMove {

		return fmt.Sprintf("%v\n", g.Position().XFENString())
	} // else

	var sb strings.Builder

	for idx, pos := range g.Positions() {
		if (opts.colorc == chess.NoColor || opts.colorc == pos.Turn()) &&
			(opts.all || ((idx/2+1) >= opts.startMoveNum &&
				(idx/2+1) <= opts.endMoveNum)) {
			sb.WriteString(fmt.Sprintf("%v\n", pos.XFENString()))
		}
	}

	return sb.String()
}
