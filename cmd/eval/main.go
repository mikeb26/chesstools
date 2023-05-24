package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

func main() {
	evalCtx := chesstools.NewEvalCtx(false)
	defer evalCtx.Close()

	parseArgs(evalCtx)
	evalCtx.InitEngine()
	er := evalCtx.Eval()
	displayOutput(evalCtx, er)
}

func displayOutput(evalCtx *chesstools.EvalCtx, er *chesstools.EvalResult) {
	fmt.Printf("FEN: %v\n", evalCtx.GetPosition())
	fmt.Printf("Best Move: %v\n", er.BestMove)
	fmt.Printf("Score: cp: %v\n", er.CP)
	fmt.Printf("Score: mate: %v\n", er.Mate)
	fmt.Printf("Depth: %v\n", er.Depth)
	fmt.Printf("k-nodes/s: %v\n", er.KNPS)
}

func parseArgs(evalCtx *chesstools.EvalCtx) {
	f := flag.NewFlagSet("eval", flag.ExitOnError)

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

	f.Parse(os.Args[1:])

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
}
