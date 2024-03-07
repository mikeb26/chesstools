package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

func main() {
	evalCtx := chesstools.NewEvalCtx(false)
	defer evalCtx.Close()

	dark, doUpgrade := parseArgs(evalCtx)
	evalCtx.InitEngine()
	if doUpgrade {
		evalCtx.UpgradeCache()
		return
	} // else

	er := evalCtx.Eval()
	displayOutput(evalCtx, er, dark)
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
	fmt.Printf("Depth: %v\n", er.Depth)
	fmt.Printf("k-nodes/s: %v\n", er.KNPS)
	if er.SearchTimeInSeconds != chesstools.UnknownSearchTime {
		fmt.Printf("SearchTime: %vs\n", uint(math.Round(er.SearchTimeInSeconds)))
	}
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

	fmt.Printf(b.Draw2(p.Turn(), dark))
}

func parseArgs(evalCtx *chesstools.EvalCtx) (bool, bool) {
	f := flag.NewFlagSet("cteval", flag.ExitOnError)

	var pgnFile string
	f.StringVar(&pgnFile, "pgn", "", "<pgnFileName>")
	var fen string
	f.StringVar(&fen, "fen", "", "<FEN string>")
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
	f.BoolVar(&staleOk, "staleok", false, "accept cached evals from older engine versions")
	var noCloudCache bool
	f.BoolVar(&noCloudCache, "nocloudcache", false, "do not reference lichess APIs for cached evaluations")
	var doUpgrade bool
	f.BoolVar(&doUpgrade, "upgrade", false, "upgrade all existing cached evaluations using the most recently installed engine version")

	f.Parse(os.Args[1:])

	if doUpgrade {
		return false, true
	}

	var turn chess.Color
	if pgnFile == "" && fen == "" {
		panic("please specify --pgn <pgnFile> or --fen <FEN string>")
	}
	if fen != "" && (pgnFile != "" || moveNum != 0 || colorFlag != "") {
		panic("please specify either (--pgn <pgnFile> --move <moveNum> --turn <white|black>) or --fen <FEN string>")
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
	} else {
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
	if staleOk {
		evalCtx = evalCtx.WithStaleOk()
	}
	if noCloudCache {
		evalCtx = evalCtx.WithoutCloudCache()
	}

	return dark, false
}
