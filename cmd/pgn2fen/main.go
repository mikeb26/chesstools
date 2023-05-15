package main

import (
	"fmt"
	"os"

	"github.com/notnil/chess"
)

func main() {
	pgn, err := chess.PGN(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	g := chess.NewGame(pgn)
	fmt.Printf("%v\n", g.FEN())
}
