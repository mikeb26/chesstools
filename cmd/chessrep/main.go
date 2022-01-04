/* Utility for validating that an opening repertoire is self consistent.
 * Given an opening repertoire in .pgn format & color, the tool will validate
 * that the same move is played in every unique position regardless of move
 * order.
 */

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type RepValidator struct {
	color               chess.Color
	scoreDepth          int
	scoreExceptions     map[string]string
	scoreExceptionsFile string
	pgnFileList         []string
	moveMap             map[string]MoveMapValue
	uniquePosCount      uint
	dupPosCount         uint
	conflictPosCount    uint
	gameList            []*chess.Game
	whiteConflictList   []Conflict
	blackConflictList   []Conflict
	evalCtx             *chesstools.EvalCtx
}

type MoveMapValue struct {
	move        string
	game        *chess.Game
	gameNum     int
	pgnFilename string
}

type Conflict struct {
	existingMove MoveMapValue
	conflictMove MoveMapValue
}

func NewRepValidator(c chess.Color, sd int, pgns []string,
	scoreExceptionsFileIn string) *RepValidator {

	rv := &RepValidator{
		color:               c,
		scoreDepth:          sd,
		scoreExceptions:     make(map[string]string, 0),
		scoreExceptionsFile: scoreExceptionsFileIn,
		pgnFileList:         make([]string, len(pgns)),
		moveMap:             make(map[string]MoveMapValue),
		uniquePosCount:      0,
		dupPosCount:         0,
		conflictPosCount:    0,
		gameList:            make([]*chess.Game, 0),
		whiteConflictList:   make([]Conflict, 0),
		blackConflictList:   make([]Conflict, 0),
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
	var scoreDepth int
	var scoreExceptionsFile string
	pgnList, err := parseArgs(&color, &scoreDepth, &scoreExceptionsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	rv := NewRepValidator(color, scoreDepth, pgnList, scoreExceptionsFile)
	err = rv.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize validator: %v\n", err)
		return
	}

	rv.printStatsAndConflicts()
	if rv.evalCtx != nil {
		rv.evalCtx.Close()
		rv.evalCtx = nil
	}
}

func parseArgs(c *chess.Color, scoreDepth *int, scoreExceptionsFile *string) ([]string, error) {
	f := flag.NewFlagSet("fenvalidate", flag.ExitOnError)
	var colorFlag string

	f.StringVar(&colorFlag, "color", "", "<white|black> (repertoire color)")
	f.StringVar(scoreExceptionsFile, "exceptions", "", "file with score exceptions")
	f.IntVar(scoreDepth, "depth", 0, "<evalDepthInPlies>")
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
		fmt.Printf("\t\tMove %v from game %v(%v#%v) conflicts with move %v from	game %v(%v#%v)\n",
			c.conflictMove.move, getGameName(c.conflictMove.game),
			c.conflictMove.pgnFilename, c.conflictMove.gameNum, c.existingMove.move,
			getGameName(c.existingMove.game), c.existingMove.pgnFilename,
			c.existingMove.gameNum)
	}
}

func (rv *RepValidator) loadExceptions() error {
	encodedExceptions, err := ioutil.ReadFile(rv.scoreExceptionsFile)
	if err != nil {
		return fmt.Errorf("Failed to open exceptions file %v: %w",
			rv.scoreExceptionsFile, err)
	}

	type ScoreException struct {
		FEN  string
		Move string
	}

	var exceptions []ScoreException
	err = json.Unmarshal(encodedExceptions, &exceptions)
	if err != nil {
		return fmt.Errorf("Failed to parse exceptions file %v: %w",
			rv.scoreExceptionsFile, err)
	}
	for _, e := range exceptions {
		normalizedFen, err := normalizeFEN(e.FEN)
		if err != nil {
			return fmt.Errorf("Failed to parse FEN %v in exceptions file %v: %w",
				e.FEN, rv.scoreExceptionsFile, err)
		}
		rv.scoreExceptions[normalizedFen] = e.Move
	}

	return nil
}

func (rv *RepValidator) Load() error {
	var err error

	if rv.scoreDepth > 0 && rv.scoreExceptionsFile != "" {
		err = rv.loadExceptions()
		if err != nil {
			return err
		}
	}
	for _, pgnFilename := range rv.pgnFileList {
		f, err := chesstools.OpenPgn(pgnFilename)
		if err != nil {
			return err
		}
		defer f.Close()

		err = rv.processOnePGN(f, pgnFilename)
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

func (rv *RepValidator) processOnePGN(f io.Reader, pgnFilename string) error {
	var opts chess.ScannerOpts
	opts.ExpandVariations = true

	scanner := chess.NewScannerWithOptions(f, opts)

	var err error
	ii := 1
	for scanner.Scan() {
		g := scanner.Next()
		err = rv.processOneGame(g, pgnFilename, ii)
		if err != nil {
			return err
		}
		ii++
	}

	return scanner.Err()
}

func (rv *RepValidator) processOneGame(g *chess.Game, pgnFilename string,
	gameNumLocal int) error {
	rv.gameList = append(rv.gameList, g)
	moves := g.Moves()
	var m string

	moveCount := 1
	scoreFutureMovesThisGame := true
	for ii, p := range g.Positions() {
		if ii >= len(moves) {
			continue
		}

		//var notation chess.AlgebraicNotation
		//m = notation.Encode(p, moves[ii])
		m = moves[ii].String()
		err := rv.processOneMove(g, pgnFilename, gameNumLocal, p, moveCount, m,
			&scoreFutureMovesThisGame)
		if err != nil {
			return err
		}

		if ii%2 == 1 {
			moveCount++
		}
	}

	return nil
}

func normalizeFEN(fen string) (string, error) {
	// for opening repertoire purposes zero the halfmove clock field and reset
	// the full move number field from the FEN as these may differ across
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

		_, err = sb.WriteRune(' ')
		if err != nil {
			return "", err
		}
	}
	_, err = sb.WriteString("0 1")
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

func sprintMove(moveCount int, m string, c chess.Color) string {
	ret := fmt.Sprintf("%v. ", moveCount)

	if c == chess.Black {
		ret += "... "
	}

	ret += m

	return ret
}

func (rv *RepValidator) scoreMove(g *chess.Game, pgnFilename string,
	gameNumLocal int, fen string, moveCount int, m string) bool {

	if rv.evalCtx == nil {
		rv.evalCtx = chesstools.NewEvalCtx().WithFEN(fen).WithEvalDepth(rv.scoreDepth)
		rv.evalCtx.InitEngine()
	} else {
		rv.evalCtx.SetFEN(fen)
	}

	er := rv.evalCtx.Eval()
	if er.BestMove != m {
		exceptionsMove, ok := rv.scoreExceptions[fen]
		if !ok {
			fmt.Printf("Engine recommends %v instead of %v in game %v(%v#%v) FEN:%v\n",
				sprintMove(moveCount, er.BestMove, rv.color),
				sprintMove(moveCount, m, rv.color), getGameName(g), pgnFilename,
				gameNumLocal, fen)

			return false
		} // else
		if exceptionsMove == m {
			fmt.Printf("Ignoring engine recommended %v instead of %v in game %v(%v#%v) FEN:%v\n",
				sprintMove(moveCount, er.BestMove, rv.color),
				sprintMove(moveCount, m, rv.color), getGameName(g), pgnFilename,
				gameNumLocal, fen)

			return true
		} // else

		fmt.Printf("Exceptions move %v does not match repertoire move %v in game %v(%v#%v) FEN:%v\n",
			sprintMove(moveCount, exceptionsMove, rv.color),
			sprintMove(moveCount, m, rv.color), getGameName(g), pgnFilename,
			gameNumLocal, fen)

		return false
	}

	return true
}

func (rv *RepValidator) processOneMove(g *chess.Game, pgnFilenameLocal string,
	gameNumLocal int, p *chess.Position, moveCount int, m string,
	scoreFutureMovesThisGame *bool) error {
	fen, err := normalizeFEN(p.String())
	if err != nil {
		return err
	}

	val, present := rv.moveMap[fen]
	if !present {
		rv.moveMap[fen] = MoveMapValue{move: m, game: g, gameNum: gameNumLocal,
			pgnFilename: pgnFilenameLocal}
		rv.uniquePosCount++

		if p.Turn() == rv.color && rv.scoreDepth != 0 && moveCount > 3 {
			if *scoreFutureMovesThisGame {
				*scoreFutureMovesThisGame = rv.scoreMove(g, pgnFilenameLocal,
					gameNumLocal, fen, moveCount, m)
			} else {
				fmt.Printf("Skipping scoring move %v in game %v(%v#%v) FEN:%v due to earlier move engine recommendation\n",
					moveCount, getGameName(g), pgnFilenameLocal, gameNumLocal, fen)
			}
		}
		return nil
	} // else

	if val.move != m {
		conflictVal := MoveMapValue{move: m, game: g, gameNum: gameNumLocal,
			pgnFilename: pgnFilenameLocal}
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
