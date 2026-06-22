package eval

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/corentings/chess/v2"
	"github.com/mikeb26/chesstools"
)

type inputPosition struct {
	fen   string
	label string
}

func Main(args []string) {
	evalCtx := chesstools.NewEvalCtx(false)
	defer evalCtx.Close()

	dark, doUpgrade, fenFile := parseArgs(args, evalCtx)

	var positions []inputPosition
	if fenFile != "" {
		var err error
		positions, err = loadFENFile(fenFile)
		if err != nil {
			panic(err)
		}
		evalCtx.WithFEN(positions[0].fen)
	}

	evalCtx.InitEngine()
	if doUpgrade {
		evalCtx.UpgradeCache()
		return
	} // else

	if fenFile == "" {
		er := evalCtx.Eval()
		displayOutput(evalCtx, er, dark)
		return
	}

	for ii, position := range positions {
		if ii != 0 {
			fmt.Printf("\n")
			evalCtx.SetFEN(position.fen)
		}

		fmt.Printf("=== Position %v (%v) ===\n", ii+1, position.label)
		er := evalCtx.Eval()
		displayOutput(evalCtx, er, dark)
		if er == nil {
			os.Exit(1)
		}
	}
}

func displayOutput(evalCtx *chesstools.EvalCtx, er *chesstools.EvalResult,
	dark bool) {
	if er == nil {
		fmt.Printf("Not found\n")
		return
	}

	fen := evalCtx.GetPosition()
	fmt.Printf("FEN: %v\n", fen)
	fmt.Printf("Best Move: %v\n", er.BestMove)
	if er.Mate == 0 {
		fmt.Printf("Eval: %.2v\n", float32(er.CP)/100)
	} else {
		fmt.Printf("Eval: mate-in-%v\n", er.Mate)
	}
	fmt.Printf("Win/Draw/Loss: %v%%/%v%%/%v%%\n", uint(er.WinPct*100),
		uint(er.DrawPct*100), uint(er.LossPct*100))
	fmt.Printf("Depth: %v\n", er.Depth)
	fmt.Printf("k-nodes/s: %v\n", er.KNPS)
	if er.SearchTimeInSeconds != chesstools.UnknownSearchTime {
		fmt.Printf("SearchTime: %vs\n", uint(math.Round(er.SearchTimeInSeconds)))
	}
	fmt.Printf("Type: %v\n", er.Type)
	if er.EngVersion != chesstools.UnknownEngVer {
		fmt.Printf("EngVer: %v\n", er.EngVersion)
	} else {
		fmt.Printf("EngVer: <unknown>\n")
	}

	newGameArgs, err := chess.FEN(fen)
	if err != nil {
		panic(fmt.Sprintf("FEN invalid err:%v fen:%v", err, fen))
	}

	g := chess.NewGame(newGameArgs)
	p := g.Position()
	b := p.Board()

	fmt.Print(b.Draw2(p.Turn(), dark))
}

func parseArgs(args []string, evalCtx *chesstools.EvalCtx) (bool, bool, string) {
	f := flag.NewFlagSet("cteval", flag.ExitOnError)

	var pgnFile string
	f.StringVar(&pgnFile, "pgn", "", "<pgnFileName>")
	var fen string
	f.StringVar(&fen, "fen", "", "<FEN string>")
	var fenFile string
	f.StringVar(&fenFile, "fenfile", "", "<fenFileName|- for stdin>")
	var colorFlag string
	f.StringVar(&colorFlag, "turn", "", "<white|black>")
	var moveNum uint
	f.UintVar(&moveNum, "move", 0, "<moveNum>")
	var evalTimeInSec uint
	f.UintVar(&evalTimeInSec, "time", 0, "<evalTimeInSeconds>")
	var evalDepth int
	f.IntVar(&evalDepth, "depth", 0, "<evalDepthInPlies>")
	var numThreads uint64
	f.Uint64Var(&numThreads, "thread", 0, "<numThreads>")
	var hashSizeInMiB uint64
	f.Uint64Var(&hashSizeInMiB, "hash", 0, "<hashSizeInMiB>")
	var dark bool
	f.BoolVar(&dark, "dark", false, "<true|false>")
	var cacheOnly bool
	f.BoolVar(&cacheOnly, "cacheonly", false, "only return cached evaluations")
	var staleOk bool
	f.BoolVar(&staleOk, "staleok", true, "accept cached evals from older engine versions")
	var noCloudCache bool
	f.BoolVar(&noCloudCache, "nocloudcache", false, "do not reference lichess APIs for cached evaluations")
	var doUpgrade bool
	f.BoolVar(&doUpgrade, "upgrade", false, "upgrade all existing cached evaluations using the most recently installed engine version")

	f.Parse(args)

	if doUpgrade {
		return false, true, ""
	}

	var turn chess.Color
	if pgnFile == "" && fen == "" && fenFile == "" {
		panic("please specify --pgn <pgnFile>, --fen <FEN string>, or --fenfile <fenFileName|->")
	}
	if fen != "" && (pgnFile != "" || fenFile != "" || moveNum != 0 || colorFlag != "") {
		panic("please specify exactly one input mode: (--pgn <pgnFile> --move <moveNum> --turn <white|black>), --fen <FEN string>, or --fenfile <fenFileName|->")
	}
	if fenFile != "" && (pgnFile != "" || fen != "" || moveNum != 0 || colorFlag != "") {
		panic("please specify exactly one input mode: (--pgn <pgnFile> --move <moveNum> --turn <white|black>), --fen <FEN string>, or --fenfile <fenFileName|->")
	}
	if pgnFile != "" {
		if moveNum == 0 {
			panic("please specify --move <moveNum>")
		}

		switch strings.ToUpper(colorFlag) {
		case "WHITE":
			fallthrough
		case "W":
			turn = chess.White
		case "BLACK":
			fallthrough
		case "B":
			turn = chess.Black
		default:
			panic("please specify --turn <white|black>")
		}
	}
	if evalDepth != 0 && evalTimeInSec != 0 {
		panic("--depth and --time are mutually exclusive")
	}

	if pgnFile != "" {
		evalCtx = evalCtx.WithPgnFile(pgnFile).WithMoveNum(moveNum).WithTurn(turn)
	} else if fen != "" {
		evalCtx = evalCtx.WithFEN(fen)
	}
	if evalDepth != 0 {
		evalCtx = evalCtx.WithEvalDepth(evalDepth)
	} else if evalTimeInSec != 0 {
		evalCtx = evalCtx.WithEvalTime(evalTimeInSec)
	}
	if numThreads != 0 {
		evalCtx = evalCtx.WithThreads(numThreads)
	}
	if hashSizeInMiB != 0 {
		evalCtx = evalCtx.WithHashSize(hashSizeInMiB)
	}
	if cacheOnly {
		evalCtx = evalCtx.WithCacheOnly()
	}
	evalCtx = evalCtx.WithStaleOk(staleOk)
	if noCloudCache {
		evalCtx = evalCtx.WithoutCloudCache()
	}

	return dark, false, fenFile
}

func loadFENFile(fenFile string) ([]inputPosition, error) {
	if fenFile == "-" {
		return loadFENs(os.Stdin, "stdin")
	}

	file, err := os.Open(fenFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return loadFENs(file, fenFile)
}

func loadFENs(reader io.Reader, source string) ([]inputPosition, error) {
	positions := make([]inputPosition, 0)
	scanner := bufio.NewScanner(reader)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		fen := strings.TrimSpace(scanner.Text())
		if fen == "" || strings.HasPrefix(fen, "#") {
			continue
		}

		_, err := chess.FEN(fen)
		if err != nil {
			return nil, fmt.Errorf("%v:%v invalid FEN: %w", source, lineNum, err)
		}

		positions = append(positions, inputPosition{
			fen:   fen,
			label: fmt.Sprintf("%v:%v", source, lineNum),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%v: failed to read FEN file: %w", source, err)
	}
	if len(positions) == 0 {
		return nil, fmt.Errorf("%v: no FEN positions found", source)
	}

	return positions, nil
}
