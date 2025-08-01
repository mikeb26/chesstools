/* Utility for comreating an opening repertoire
 */

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/corentings/chess/v2"
	"github.com/mikeb26/chesstools"
)

type RepBldOpts struct {
	color        chess.Color
	threshold    float64
	maxDepth     int
	startMoves   string
	inputFile    string
	outputFile   string
	outputMode   OutputMode
	keepExisting bool
	engineSelect bool
	engineTime   uint
	opponent     string
	dark         bool
	minGames     int
	expandVar    bool
	noAtime      bool
}

type MoveMapValue struct {
	move          string
	game          *chess.Game
	gameNum       int
	pgnFilename   string
	actualFen     string
	normalizedFen string
	totalPct      float64
	hitCount      int
}

var moveMap map[string]*MoveMapValue
var dag *Dag
var evalCtx *chesstools.EvalCtx

const DefaultMinGames = 200

func parseArgs(opts *RepBldOpts) error {

	f := flag.NewFlagSet("repbld", flag.ExitOnError)
	var colorFlag string
	var format string

	f.StringVar(&colorFlag, "color", "", "<white|black> (repertoire color)")
	f.StringVar(&opts.startMoves, "start", "", "<pgnStart> (starting moves)")
	f.StringVar(&opts.inputFile, "input", "", "<existingRep>")
	f.StringVar(&opts.outputFile, "output", "", "<outputFile>")
	f.StringVar(&format, "format", "flattened", "<flattened|consolidated>")
	f.Float64Var(&opts.threshold, "threshold", 0.02, "<thresholdPct>")
	f.IntVar(&opts.maxDepth, "maxdepth", 14, "<max depth>")
	f.BoolVar(&opts.keepExisting, "keepexisting", true, "<true|false>")
	f.BoolVar(&opts.dark, "dark", false, "<true|false>")
	f.BoolVar(&opts.engineSelect, "engineselect", false, "<true|false>")
	f.UintVar(&opts.engineTime, "enginetime", 300, "<engine search time per position>")
	f.StringVar(&opts.opponent, "opponent", "", "<lichess_username> (opponent to prep for)")
	f.IntVar(&opts.minGames, "mingames", DefaultMinGames, "<minimum games to consider from an opening book position>")
	f.BoolVar(&opts.expandVar, "includevar", true, "include variations in input pgn <true|false>")

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
		return fmt.Errorf("please specify --color <white|black>")
	}

	switch strings.ToUpper(format) {
	case "FLATTENED":
		opts.outputMode = Flattened
	case "CONSOLIDATED":
		opts.outputMode = Consolidated
	default:
		return fmt.Errorf("please specify --format <flattened|consolidated>")
	}

	if opts.outputFile == "" {
		return fmt.Errorf("please specify --output <outputFile>")
	}

	return nil
}

func main() {
	var opts RepBldOpts
	err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		os.Exit(1)
		return
	}

	mainWork(&opts)
}

func mainWork(opts *RepBldOpts) {
	moveMap = make(map[string]*MoveMapValue)
	dag = NewDag(opts.color, opts.outputMode)
	if opts.engineSelect {
		evalCtx = chesstools.NewEvalCtx(false).WithEvalTime(opts.engineTime)
		if opts.noAtime {
			evalCtx = evalCtx.WithoutAtime()
		}
		defer evalCtx.Close()
		evalCtx.InitEngine()
	}

	_ = os.Remove(opts.outputFile)
	outFile, err := os.OpenFile(opts.outputFile, os.O_CREATE|os.O_RDWR, 0644)
	defer outFile.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open output '%v': %v\n",
			opts.outputFile, err)
		os.Exit(1)
	}

	if opts.inputFile != "" {
		inFile, err := chesstools.OpenPgn(opts.inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open %v: %v\n", opts.inputFile,
				err)
			os.Exit(1)
		}
		defer inFile.Close()

		err = processOnePGN(opts, inFile, dag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %v: %v\n", opts.inputFile,
				err)
			os.Exit(1)
		}
	}

	var openingGame *chesstools.OpeningGame
	if opts.startMoves != "" {
		startMovesReader := strings.NewReader(opts.startMoves)
		var pgnReader func(*chess.Game)
		pgnReader, err = chess.PGN(startMovesReader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %v: %v\n", opts.startMoves,
				err)
			os.Exit(1)
		}
		startGame := chess.NewGame(pgnReader)
		openingGame =
			chesstools.NewOpeningGame().WithGame(startGame).WithThreshold(opts.threshold)
		if opts.opponent != "" {
			openingGame = openingGame.WithOpponent(opts.opponent, opts.color.Other()).WithFullRatingRange(true)
		}
		openingGame = openingGame.WithTopReplies(true).WithEval(opts.color == startGame.Position().Turn())
	} else {
		openingGame = chesstools.NewOpeningGame().WithThreshold(opts.threshold)
		if opts.opponent != "" {
			openingGame = openingGame.WithOpponent(opts.opponent, opts.color.Other()).WithFullRatingRange(true)
		}
		openingGame = openingGame.WithTopReplies(true).WithEval(opts.color == chess.White)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init: %v\n", err)
		os.Exit(1)
		return
	}
	_, err = buildRep(opts, openingGame, 1.0, outFile, 0)

	dag.emit(outFile)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build rep: %v\n", err)
		os.Exit(1)
		return
	}

	return
}

func buildRep(opts *RepBldOpts,
	openingGame *chesstools.OpeningGame,
	totalPct float64,
	output io.Writer,
	stackDepth int) (bool, error) {

	if openingGame.Turn() == opts.color {
		var mv string
		err := io.EOF
		var childGame *chesstools.OpeningGame
		for {
			mv, err = selectMove(openingGame, totalPct, opts.engineSelect, opts)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if mv == "quit" {
				dag.addNodesFromGame(openingGame.Parent.G)
				return false, fmt.Errorf("told to quit")
			} else if mv == "endvar" {
				dag.addNodesFromGame(openingGame.Parent.G)
				return false, nil
			}
			childGame = chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv).WithThreshold(openingGame.Threshold).WithTopReplies(true)
			break
		}

		totalPct, err = processOneMove(openingGame.G, "<stdin>", 0,
			openingGame.G.Position(), 0, mv, totalPct)
		if err != nil {
			return false, err
		}
		emittedAny := false
		if childGame.GetMoveCount() < opts.maxDepth {
			emittedAny, err = buildRep(opts, childGame, totalPct, output,
				stackDepth+1)
			if err != nil {
				return false, err
			}
		}
		if !emittedAny {
			dag.addNodesFromGame(childGame.G)
		}
		return true, nil
	} // else opponent's turn

	respTotal := openingGame.OpeningResp.Total()
	if opts.opponent == "" && respTotal < opts.minGames {
		return false, nil
	}

	emittedAny := false
	for _, mv := range openingGame.OpeningResp.Moves {
		mvTotal := mv.Total()
		childTotalPct := totalPct * chesstools.Pct(mvTotal, respTotal)
		if childTotalPct < openingGame.Threshold {
			continue
		}
		needEvals := true
		if alreadyKnowMove(openingGame, mv.San) || opts.engineSelect {
			needEvals = false
		}
		childGame := chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv.San).WithThreshold(openingGame.Threshold).WithTopReplies(needEvals).WithEval(needEvals)
		emittedAny = true
		_, err := buildRep(opts, childGame, childTotalPct, output, stackDepth+1)
		if err != nil {
			return false, err
		}
	}

	return emittedAny, nil
}

func selectMove(openingGame *chesstools.OpeningGame,
	totalPct float64, engineSelect bool, opts *RepBldOpts) (string, error) {

	fen, err := chesstools.NormalizeFEN(openingGame.G.Position().XFENString())
	if err != nil {
		return "", err
	}

	val, present := moveMap[fen]
	if present {
		return val.move, nil
	}

	if engineSelect {
		return selectMoveViaEngine(openingGame, fen)
	} // else

	return selectMoveInteractive(openingGame, totalPct, opts)
}

func selectMoveInteractive(openingGame *chesstools.OpeningGame,
	totalPct float64, opts *RepBldOpts) (string, error) {
	numBookMoves := len(openingGame.OpeningResp.Moves)

	g := openingGame.G
	pos := g.Position()
	board := pos.Board()

	fmt.Printf("\nOpening: %v (%v)\n", openingGame.String(), openingGame.Eco)
	fmt.Printf("PGN:%v\nFEN: \"%v\"\n", g.String(), pos.XFENString())
	fmt.Printf("Percent of total: %v\n", chesstools.PctS2(totalPct))
	fmt.Printf("Move Choices: \n%v", openingGame.ChoicesString(true))
	fmt.Printf("  %v. End this variation\n", numBookMoves+1)
	fmt.Printf("  %v. Quit\n", numBookMoves+2)

	fmt.Printf("%v", board.Draw2(pos.Turn(), opts.dark))
	fmt.Printf("\nEnter move: ")
	selection := ""
	fmt.Scanf("%s", &selection)
	selection = strings.TrimSpace(selection)

	// users can either enter the move directly or pick a number on the
	// presented list
	mv := selection
	selectNum, err := strconv.ParseInt(selection, 10, 32)
	if err == nil {
		if selectNum >= 1 && selectNum <= int64(numBookMoves) {
			mv = openingGame.OpeningResp.Moves[selectNum-1].San
		} else if selectNum == int64(numBookMoves)+1 {
			mv = "endvar"
		} else if selectNum == int64(numBookMoves)+2 {
			mv = "quit"
		}
	}

	return mv, nil
}

func selectMoveViaEngine(openingGame *chesstools.OpeningGame,
	fen string) (string, error) {

	evalCtx.SetFEN(fen)

	er := evalCtx.Eval()

	return er.BestMove, nil
}

func processOnePGN(repOpts *RepBldOpts, f io.Reader, dag *Dag) error {

	var scanner *chess.Scanner
	if repOpts.expandVar {
		scanner = chess.NewScanner(f, chess.WithExpandVariations())
	} else {
		scanner = chess.NewScanner(f)
	}

	ii := 1
	for scanner.HasNext() {
		g, err := scanner.ParseNext()
		if err != nil {
			return err
		}
		if len(g.Moves()) == 0 {
			continue
		}
		err = processOneGame(repOpts.color, g, repOpts.inputFile, ii)
		if err != nil {
			return err
		}
		if repOpts.keepExisting {
			dag.addNodesFromGame(g)
		}
		ii++
	}

	return nil
}

func processOneGame(color chess.Color, g *chess.Game, pgnFilename string,
	gameNumLocal int) error {

	moves := g.Moves()
	var m string

	moveCount := 1
	for ii, p := range g.Positions() {
		if ii >= len(moves) {
			continue
		}
		if p.Turn() != color {
			continue
		}

		encoder := chess.AlgebraicNotation{}
		m = encoder.Encode(p, moves[ii])
		_, err := processOneMove(g, pgnFilename, gameNumLocal, p, moveCount, m,
			0.0)
		if err != nil {
			return err
		}

		if ii%2 == 1 {
			moveCount++
		}
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

func processOneMove(g *chess.Game, pgnFilenameLocal string,
	gameNumLocal int, p *chess.Position, moveCount int,
	m string, existingTotalPct float64) (float64, error) {

	fen, err := chesstools.NormalizeFEN(p.XFENString())
	if err != nil {
		return 0.0, err
	}

	val, present := moveMap[fen]
	if !present {
		moveMap[fen] = &MoveMapValue{move: m, game: g, gameNum: gameNumLocal,
			pgnFilename: pgnFilenameLocal, actualFen: p.XFENString(),
			normalizedFen: fen, totalPct: existingTotalPct, hitCount: 0}
		return existingTotalPct, nil
	} // else

	val.hitCount++
	if val.move != m {
		conflictVal := MoveMapValue{move: m, game: g, gameNum: gameNumLocal,
			pgnFilename: pgnFilenameLocal}
		fmt.Printf("* Move %v from game %v(%v#%v) conflicts with move %v from	game %v(%v#%v)\n",
			conflictVal.move, getGameName(conflictVal.game),
			conflictVal.pgnFilename, conflictVal.gameNum, val.move,
			getGameName(val.game), val.pgnFilename,
			val.gameNum)
		fmt.Printf("*   Using the latter\n")
	} else {
		// @todo math here is not quite right but may be good enough?
		// the point is to treat positions that transpose with their aggregate
		// percentage so that we deepen the derivitive variations
		val.totalPct += existingTotalPct
	}

	return val.totalPct, nil
}

func alreadyKnowMove(openingGame *chesstools.OpeningGame, mv string) bool {
	tmpGame := chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv).WithThreshold(openingGame.Threshold)

	fen, err := chesstools.NormalizeFEN(tmpGame.G.Position().XFENString())
	if err != nil {
		panic(err)
	}

	_, present := moveMap[fen]

	return present
}
