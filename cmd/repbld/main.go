/* Utility for comreating an opening repertoire
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

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

const MinGames = 200

func parseArgs(c *chess.Color, threshold *float64, maxDepth *int,
	startMoves *string, inputFile *string, outputFile *string,
	outputMode *OutputMode, keepExisting *bool) error {

	f := flag.NewFlagSet("repbld", flag.ExitOnError)
	var colorFlag string
	var format string

	f.StringVar(&colorFlag, "color", "", "<white|black> (repertoire color)")
	f.StringVar(startMoves, "start", "", "<pgnStart> (starting moves)")
	f.StringVar(inputFile, "input", "", "<existingRep>")
	f.StringVar(outputFile, "output", "", "<outputFile>")
	f.StringVar(&format, "format", "", "<flattened|consolidated>")
	f.Float64Var(threshold, "threshold", 0.02, "<thresholdPct>")
	f.IntVar(maxDepth, "maxdepth", 14, "<max depth>")
	f.BoolVar(keepExisting, "keepexisting", false, "<true|false>")
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
		return fmt.Errorf("please specify --color <white|black>")
	}
	switch strings.ToUpper(format) {
	case "FLATTENED":
		*outputMode = Flattened
	case "CONSOLIDATED":
		*outputMode = Consolidated
	default:
		return fmt.Errorf("please specify --format <flattened|consolidated>")
	}

	return nil
}

func main() {
	var color chess.Color
	var threshold float64
	var maxDepth int
	var startMoves string
	var inputFile string
	var outputFile string
	var outputMode OutputMode
	var keepExisting bool
	err := parseArgs(&color, &threshold, &maxDepth, &startMoves, &inputFile,
		&outputFile, &outputMode, &keepExisting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse arguments: %v\n", err)
		os.Exit(1)
		return
	}

	mainWork(color, threshold, maxDepth, startMoves, inputFile, outputFile,
		outputMode, keepExisting)
}

func mainWork(color chess.Color, threshold float64, maxDepth int,
	startMoves string, inputFile string, outputFile string,
	outputMode OutputMode, keepExisting bool) {

	moveMap = make(map[string]*MoveMapValue)
	dag = NewDag(color, outputMode)

	_ = os.Remove(outputFile)
	outFile, err := os.OpenFile(outputFile, os.O_CREATE|os.O_RDWR, 0644)
	defer outFile.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open %v: %v\n", outputFile, err)
		os.Exit(1)
	}

	if inputFile != "" {
		inFile, err := chesstools.OpenPgn(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open %v: %v\n", inputFile, err)
			os.Exit(1)
		}
		defer inFile.Close()

		err = processOnePGN(color, inFile, inputFile, dag, keepExisting)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %v: %v\n", inputFile, err)
			os.Exit(1)
		}
	}

	var openingGame *chesstools.OpeningGame
	if startMoves != "" {
		startMovesReader := strings.NewReader(startMoves)
		var pgnReader func(*chess.Game)
		pgnReader, err = chess.PGN(startMovesReader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse %v: %v\n", startMoves, err)
			os.Exit(1)
		}
		startGame := chess.NewGame(pgnReader)
		openingGame, err = chesstools.NewOpeningGame2(startGame, true,
			threshold, color == startGame.Position().Turn())
	} else {
		openingGame, err = chesstools.NewOpeningGame(nil, "", true,
			threshold, color == chess.White)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init: %v\n", err)
		os.Exit(1)
		return
	}
	_, err = buildRep(color, openingGame, 1.0, outFile, 0, maxDepth)

	dag.emit(outFile)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build rep: %v\n", err)
		os.Exit(1)
		return
	}

	return
}

func buildRep(color chess.Color,
	openingGame *chesstools.OpeningGame,
	totalPct float64,
	output io.Writer,
	stackDepth int,
	maxMoveDepth int) (bool, error) {

	if openingGame.Turn() == color {
		var mv string
		err := io.EOF
		var childGame *chesstools.OpeningGame
		for {
			mv, err = selectMove(openingGame, totalPct)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if mv == "quit" {
				return false, fmt.Errorf("told to quit")
			} else if mv == "endvar" {
				return false, nil
			}
			childGame, err = chesstools.NewOpeningGame(openingGame, mv, true,
				openingGame.Threshold, false)

			if err == nil {
				break
			}
		}

		totalPct, err = processOneMove(openingGame.G, "<stdin>", 0,
			openingGame.G.Position(), 0, mv, totalPct)
		if err != nil {
			return false, err
		}
		emittedAny := false
		if childGame.GetMoveCount() < maxMoveDepth {
			emittedAny, err = buildRep(color, childGame, totalPct, output,
				stackDepth+1, maxMoveDepth)
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
	if respTotal < MinGames {
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
		if alreadyKnowMove(openingGame, mv.San) {
			needEvals = false
		}
		childGame, err := chesstools.NewOpeningGame(openingGame, mv.San, needEvals,
			openingGame.Threshold, needEvals)
		if err != nil {
			return false, err
		}
		emittedAny = true
		_, err = buildRep(color, childGame, childTotalPct, output, stackDepth+1,
			maxMoveDepth)
		if err != nil {
			return false, err
		}
	}

	return emittedAny, nil
}

func selectMove(openingGame *chesstools.OpeningGame,
	totalPct float64) (string, error) {

	fen, err := chesstools.NormalizeFEN(openingGame.G.Position().XFENString())
	if err != nil {
		return "", err
	}

	val, present := moveMap[fen]
	if present {
		return val.move, nil
	}

	fmt.Printf("Opening Name: %v\n", openingGame.String())
	fmt.Printf("Current Moves: %v\n", openingGame.G.String())
	fmt.Printf("Percent of total: %v\n", chesstools.PctS2(totalPct))
	fmt.Printf("Choices: \n%v\n", openingGame.ChoicesString(true))

	fmt.Printf("%v", openingGame.G.Position().Board().Draw())
	fmt.Printf("Enter move: ")
	mv := ""
	fmt.Scanf("%s", &mv)
	mv = strings.TrimSpace(mv)
	return mv, nil
}

func processOnePGN(color chess.Color, f io.Reader, pgnFilename string,
	dag *Dag, keepExisting bool) error {

	var opts chess.ScannerOpts
	opts.ExpandVariations = true

	scanner := chess.NewScannerWithOptions(f, opts)

	var err error
	ii := 1
	for scanner.Scan() {
		g := scanner.Next()
		if len(g.Moves()) == 0 {
			continue
		}
		err = processOneGame(color, g, pgnFilename, ii)
		if err != nil {
			return err
		}
		if keepExisting {
			dag.addNodesFromGame(g)
		}
		ii++
	}

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return err
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
	if tagPair != nil {
		gn = tagPair.Value
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
	tmpGame, err := chesstools.NewOpeningGame(openingGame, mv, false,
		openingGame.Threshold, false)
	if err != nil {
		panic(err)
	}

	fen, err := chesstools.NormalizeFEN(tmpGame.G.Position().XFENString())
	if err != nil {
		panic(err)
	}

	_, present := moveMap[fen]

	return present
}
