package gen960

import (
	"fmt"
	"os"

	"github.com/mikeb26/chesstools"
)

func printFENs(uniqFENs []string) {
	for _, fen := range uniqFENs {
		fmt.Println(fen)
	}
}

func Main(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "usage: ct 960gen")
			return
		default:
			fmt.Fprintf(os.Stderr, "960gen: does not accept arguments\n")
			os.Exit(1)
		}
	}

	uniqFENs := chesstools.Get960StartFENs()

	printFENs(uniqFENs)
}
