package main

import (
	"fmt"

	"github.com/mikeb26/chesstools"
)

func printFENs(uniqFENs []string) {
	for _, fen := range uniqFENs {
		fmt.Println(fen)
	}
}

func main() {
	uniqFENs := chesstools.Get960StartFENs()

	printFENs(uniqFENs)
}
