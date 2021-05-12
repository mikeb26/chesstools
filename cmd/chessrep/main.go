/* Utility for validating that an opening repertoire is self consistent.
 * Given an opening repertoire in .pgn format & color, the tool will validate
 * that the same move is played in every unique position regardless of move
 * order.
 */

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/notnil/chess"
)

type RepValidator struct {
	color             chess.Color
	pgnFileList       []string
	moveMap           map[string]MoveMapValue
	uniquePosCount    uint
	dupPosCount       uint
	conflictPosCount  uint
	gameList          []*chess.Game
	whiteConflictList []Conflict
	blackConflictList []Conflict
}

type MoveMapValue struct {
	move    string
	game    *chess.Game
	gameNum int
}

type Conflict struct {
	existingMove MoveMapValue
	conflictMove MoveMapValue
}

func NewRepValidator(c chess.Color, pgns []string) *RepValidator {
	rv := &RepValidator{
		color:             c,
		pgnFileList:       make([]string, len(pgns)),
		moveMap:           make(map[string]MoveMapValue),
		uniquePosCount:    0,
		dupPosCount:       0,
		conflictPosCount:  0,
		gameList:          make([]*chess.Game, 0),
		whiteConflictList: make([]Conflict, 0),
		blackConflictList: make([]Conflict, 0),
	}
	for ii, p := range pgns {
		rv.pgnFileList[ii] = p
	}

	return rv
}

/*
* load the pgn
* for each unique game & variation in the pgn, instantiate a game instance
* for each game, invoke Positions() & Moves() function. emit into a map
* each unique position using a truncated FEN as key and move as value. when
* adding to a map, first verify that the existing value is the same as the
* value being inserted.
 */

func main() {
	var color chess.Color
	pgnList, err := parseArgs(&color)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	rv := NewRepValidator(color, pgnList)
	err = rv.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load PGN files: %v\n", err)
		return
	}

	rv.printStatsAndConflicts()
}

func parseArgs(c *chess.Color) ([]string, error) {
	f := flag.NewFlagSet("fenvalidate", flag.ExitOnError)
	var colorFlag string

	f.StringVar(&colorFlag, "color", "", "<white|black> (repertoire color)")
	f.Parse(os.Args[1:])
	switch strings.ToUpper(colorFlag) {
	case "WHITE":
		fallthrough
	case "W":
		*c = chess.White
	case "BLACK":
		fallthrough
	case "B":
		*c = chess.Black
	default:
		return nil, fmt.Errorf("please specify --color <white|black>")
	}

	if len(f.Args()) == 0 {
		return nil, fmt.Errorf("please specify 1 or more PGN files representing a repertoire for %v", (*c).Name())
	}
	return f.Args(), nil
}

func (rv *RepValidator) printStatsAndConflicts() {
	fmt.Printf("Loaded %v games from %v pgn files.\n", len(rv.gameList), len(rv.pgnFileList))
	fmt.Printf("\tUnique Posisitions: %v\n\tDuplicate Positions: %v\n\tConflict Posisitions: %v (white:%v black:%v)\n", rv.uniquePosCount, rv.dupPosCount, rv.conflictPosCount, len(rv.whiteConflictList), len(rv.blackConflictList))

	var conflictList *[]Conflict
	if rv.color == chess.Black {
		conflictList = &rv.blackConflictList
	} else {
		conflictList = &rv.whiteConflictList
	}

	if len(*conflictList) == 0 {
		return
	}

	fmt.Printf("\tConflicts:\n")

	for _, c := range *conflictList {
		fmt.Printf("\t\tMove %v from game %v(#%v) conflicts with move %v from game %v(#%v)\n", c.conflictMove.move, getGameName(c.conflictMove.game), c.conflictMove.gameNum, c.existingMove.move, getGameName(c.existingMove.game), c.existingMove.gameNum)
	}
}

func (rv *RepValidator) Load() error {
	for _, pgnFilename := range rv.pgnFileList {
		f, err := os.OpenFile(pgnFilename, os.O_RDONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()

		err = rv.processOnePGN(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func getGameName(g *chess.Game) string {
	gn := "?"
	tagPair := g.GetTagPair("Event")
	if tagPair != nil {
		gn = tagPair.Value
	}

	return gn
}

func (rv *RepValidator) processOnePGN(f io.Reader) error {
	var opts chess.ScannerOpts
	opts.ExpandVariations = true

	scanner := chess.NewScannerWithOptions(f, opts)

	var err error
	ii := 1
	for scanner.Scan() {
		g := scanner.Next()
		err = rv.processOneGame(g, ii)
		if err != nil {
			return err
		}
		ii++
	}

	return scanner.Err()
}

func (rv *RepValidator) processOneGame(g *chess.Game, gameNumLocal int) error {
	rv.gameList = append(rv.gameList, g)
	moves := g.Moves()
	var m string
	for ii, p := range g.Positions() {
		if ii >= len(moves) {
			continue
		}

		var notation chess.AlgebraicNotation
		m = notation.Encode(p, moves[ii])
		err := rv.processOneMove(g, gameNumLocal, p, m)
		if err != nil {
			return err
		}
	}

	return nil
}

func getPositionKey(fen string) (string, error) {
	// for opening repertoire purposes strip off the halfmove clock and
	// full move number fields from the FEN as these may differ across
	// variations/transpositions. keep castling rights, active color, and
	// en-passant square as all of these are material. for a future release
	// consider situations where the chosen move in a position with castling
	// rights is not a castle as potentially equivalent to the same position
	// without castling rights. similarly for en-passant where the chosen
	// move is not an en-passant capture. FEN reference:
	// https://en.wikipedia.org/wiki/Forsyth%E2%80%93Edwards_Notation

	fenFields := strings.Split(fen, " ")
	if len(fenFields) != 6 {
		return "", fmt.Errorf("Invalid FEN:{%v} expecting 6 fields but found %v", fen, len(fenFields))
	}

	var sb strings.Builder
	var err error
	for ii := 0; ii < 4; ii++ {
		_, err = sb.WriteString(fenFields[ii])
		if err != nil {
			return "", err
		}
		if ii >= 3 {
			continue
		}

		_, err = sb.WriteRune(' ')
		if err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func (rv *RepValidator) processOneMove(g *chess.Game, gameNumLocal int, p *chess.Position, m string) error {
	k, err := getPositionKey(p.String())
	if err != nil {
		return err
	}

	val, present := rv.moveMap[k]
	if !present {
		rv.moveMap[k] = MoveMapValue{move: m, game: g, gameNum: gameNumLocal}
		rv.uniquePosCount++
		return nil
	} // else

	if val.move != m {
		conflictVal := MoveMapValue{move: m, game: g, gameNum: gameNumLocal}
		c := Conflict{existingMove: val, conflictMove: conflictVal}
		if p.Turn() == chess.Black {
			rv.blackConflictList = append(rv.blackConflictList, c)
		} else {
			rv.whiteConflictList = append(rv.whiteConflictList, c)
		}
		rv.conflictPosCount++
	} else {
		rv.dupPosCount++
	}

	return nil
}
