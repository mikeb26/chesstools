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

	"github.com/corentings/chess/v2"
	"github.com/mikeb26/chesstools"
)

type RepValidatorOpts struct {
	color               chess.Color
	scoreDepth          int
	scoreTime           uint
	gapThreshold        float64
	gapSkip             int
	scoreExceptionsFile string
	cacheOnly           bool
	staleOk             bool
	minMoveNum2Eval     uint
}

type RepValidator struct {
	opts *RepValidatorOpts

	scoreExceptions map[string]string
	pgnFileList     []string
	moveMap         map[string]MoveMapValue
	// positions counts do not include the final or "leaf" position in a
	// repertoire. e.g. a white opening book consisting of just:
	// 1. e4 d5 2. Nf3 Nc6 3. Bb5
	// would be counted as having 4 unique positions, which would include
	// the starting position, the position after d5, the position after Nf3,
	// and the position after Nc6, but not the final position after Bb5. Also
	// note that move number and half-move clock are ignored for the purposes
	// of testing uniqueness
	uniquePosCount    uint
	dupPosCount       uint
	conflictPosCount  uint
	gameList          []*chess.Game
	whiteConflictList []Conflict
	blackConflictList []Conflict
	evalCtx           *chesstools.EvalCtx
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

func NewRepValidator(optsIn *RepValidatorOpts, pgns []string) *RepValidator {
	rv := &RepValidator{
		opts:              optsIn,
		scoreExceptions:   make(map[string]string, 0),
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
	opts := RepValidatorOpts{}
	pgnList, err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		return
	}

	rv := NewRepValidator(&opts, pgnList)
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

func parseArgs(opts *RepValidatorOpts) ([]string, error) {

	f := flag.NewFlagSet("repvld", flag.ExitOnError)
	var colorFlag string

	f.StringVar(&colorFlag, "color", "", "<white|black> (repertoire color)")
	f.StringVar(&opts.scoreExceptionsFile, "exceptions", "", "file with score exceptions")
	f.IntVar(&opts.scoreDepth, "depth", 0, "<evalDepthInPliesPerMove>")
	f.UintVar(&opts.scoreTime, "time", 0, "<evalTimePerMove>")
	f.Float64Var(&opts.gapThreshold, "gapthreshold", 0.04, "<gapThresholdPct>")
	f.IntVar(&opts.gapSkip, "gapskip", 0, "<gapMoveSkipCount>")
	f.BoolVar(&opts.cacheOnly, "cacheonly", false, "only return cached evaluations")
	f.BoolVar(&opts.staleOk, "staleok", true, "accept cached evals from older engine versions")
	f.UintVar(&opts.minMoveNum2Eval, "minevalmovenum", 3, "<minevalmovenum>")
	f.Parse(os.Args[1:])
	switch strings.ToUpper(colorFlag) {
	case "WHITE":
		fallthrough
	case "W":
		opts.color = chess.White
	case "BLACK":
		fallthrough
	case "B":
		opts.color = chess.Black
	default:
		return nil, fmt.Errorf("please specify --color <white|black>")
	}

	if len(f.Args()) == 0 {
		return nil, fmt.Errorf("please specify 1 or more PGN files representing a repertoire for %v", opts.color.Name())
	}
	if opts.scoreTime != 0 && opts.scoreDepth != 0 {
		return nil, fmt.Errorf("--depth and --time are mutually exclusive; please choose one or the other")
	}

	return f.Args(), nil
}

func (rv *RepValidator) printStatsAndConflicts() {
	fmt.Printf("Loaded %v games from %v pgn files.\n", len(rv.gameList), len(rv.pgnFileList))
	fmt.Printf("\tUnique Posisitions: %v\n\tDuplicate Positions: %v\n\tConflict Posisitions: %v (white:%v black:%v)\n", rv.uniquePosCount, rv.dupPosCount, rv.conflictPosCount, len(rv.whiteConflictList), len(rv.blackConflictList))

	var conflictList *[]Conflict
	if rv.opts.color == chess.Black {
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
	encodedExceptions, err := ioutil.ReadFile(rv.opts.scoreExceptionsFile)
	if err != nil {
		return fmt.Errorf("Failed to open exceptions file %v: %w",
			rv.opts.scoreExceptionsFile, err)
	}

	type ScoreException struct {
		FEN  string
		Move string
	}

	var exceptions []ScoreException
	err = json.Unmarshal(encodedExceptions, &exceptions)
	if err != nil {
		return fmt.Errorf("Failed to parse exceptions file %v: %w",
			rv.opts.scoreExceptionsFile, err)
	}
	for _, e := range exceptions {
		normalizedFen, err := chesstools.NormalizeFEN(e.FEN)
		if err != nil {
			return fmt.Errorf("Failed to parse FEN %v in exceptions file %v: %w",
				e.FEN, rv.opts.scoreExceptionsFile, err)
		}
		rv.scoreExceptions[normalizedFen] = e.Move
	}

	return nil
}

func (rv *RepValidator) shouldScoreMoves() bool {
	return rv.opts.scoreDepth > 0 || rv.opts.scoreTime > 0 || rv.opts.cacheOnly
}

func (rv *RepValidator) Load() error {
	var err error

	if rv.shouldScoreMoves() && rv.opts.scoreExceptionsFile != "" {
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

	err = rv.checkForGaps()
	if err != nil {
		return err
	}

	return nil
}

func getGameName(g *chess.Game) string {
	gn := "?"
	tagPair := g.GetTagPair("Event")
	if tagPair != "" {
		gn = tagPair
	}

	return gn
}

func (rv *RepValidator) processOnePGN(f io.Reader, pgnFilename string) error {
	scanner := chess.NewScanner(f, chess.WithExpandVariations())

	ii := 1
	for scanner.HasNext() {
		g, err := scanner.ParseNext()
		if err != nil {
			return err
		}
		if len(g.Moves()) == 0 {
			continue
		}
		err = rv.processOneGame(g, pgnFilename, ii)
		if err != nil {
			return err
		}
		ii++
	}

	return nil
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

		encoder := chess.AlgebraicNotation{}
		m = encoder.Encode(p, moves[ii])
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
		rv.evalCtx =
			chesstools.NewEvalCtx(rv.opts.cacheOnly).WithFEN(fen).WithoutCloudCache()
		if rv.opts.scoreDepth > 0 {
			rv.evalCtx = rv.evalCtx.WithEvalDepth(rv.opts.scoreDepth)
		} else if rv.opts.scoreTime > 0 {
			rv.evalCtx = rv.evalCtx.WithEvalTime(rv.opts.scoreTime)
		}
		rv.evalCtx = rv.evalCtx.WithStaleOk(rv.opts.staleOk)
		rv.evalCtx.InitEngine()
	} else {
		rv.evalCtx.SetFEN(fen)
	}

	er := rv.evalCtx.Eval()
	if er == nil {
		fmt.Printf("Skipping scoring move %v in game %v(%v#%v) FEN:%v without engine eval\n",
			moveCount, getGameName(g), pgnFilename, gameNumLocal, fen)
		return true
	}
	// BestMove is occasionally missing the check+ symbol
	if er.BestMove != m &&
		er.BestMove+"+" != m {
		if er.BestMove == "Kh1" && m == "O-O" {
			return true
		}
		if er.BestMove == "Kh8" && m == "O-O" {
			return true
		}
		exceptionsMove, ok := rv.scoreExceptions[fen]
		if !ok {
			fmt.Printf("** Engine recommends %v instead of %v in game %v(%v#%v) FEN:%v\n",
				sprintMove(moveCount, er.BestMove, rv.opts.color),
				sprintMove(moveCount, m, rv.opts.color), getGameName(g), pgnFilename,
				gameNumLocal, fen)

			return false
		} // else
		if exceptionsMove == m {
			fmt.Printf("Ignoring engine recommended %v instead of %v in game %v(%v#%v) FEN:%v\n",
				sprintMove(moveCount, er.BestMove, rv.opts.color),
				sprintMove(moveCount, m, rv.opts.color), getGameName(g), pgnFilename,
				gameNumLocal, fen)

			return true
		} // else

		fmt.Printf("Exceptions move %v does not match repertoire move %v in game %v(%v#%v) FEN:%v\n",
			sprintMove(moveCount, exceptionsMove, rv.opts.color),
			sprintMove(moveCount, m, rv.opts.color), getGameName(g), pgnFilename,
			gameNumLocal, fen)

		return false
	}

	return true
}

func (rv *RepValidator) processOneMove(g *chess.Game, pgnFilenameLocal string,
	gameNumLocal int, p *chess.Position, moveCount int, m string,
	scoreFutureMovesThisGame *bool) error {
	fen, err := chesstools.NormalizeFEN(p.XFENString())
	if err != nil {
		return err
	}

	val, present := rv.moveMap[fen]
	if !present {
		rv.moveMap[fen] = MoveMapValue{move: m, game: g, gameNum: gameNumLocal,
			pgnFilename: pgnFilenameLocal}
		rv.uniquePosCount++
		if p.Turn() == rv.opts.color && rv.shouldScoreMoves() &&
			moveCount > int(rv.opts.minMoveNum2Eval) {
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
