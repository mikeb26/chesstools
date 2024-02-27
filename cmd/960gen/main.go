package main

import (
	"fmt"
	"strings"
)

func isKingBetweenRooks(backrank []rune, lastIdx int) bool {
	firstRookIdx := -1
	lastRookIdx := -1

	for ii := 0; ii <= lastIdx; ii++ {
		if backrank[ii] == 'k' {
			break
		}

		if backrank[ii] != 'r' {
			continue
		}

		if firstRookIdx == -1 {
			firstRookIdx = ii
			continue
		}
		lastRookIdx = ii
		break
	}

	return firstRookIdx != -1 && lastRookIdx == -1
}

func hasOppositeColorBishops(backrank []rune, lastIdx int) bool {
	firstBishopIdx := -1
	lastBishopIdx := -1

	for ii := 0; ii <= lastIdx; ii++ {
		if backrank[ii] != 'b' {
			continue
		}

		if firstBishopIdx == -1 {
			firstBishopIdx = ii
			continue
		}
		lastBishopIdx = ii
		break
	}

	return firstBishopIdx%2 != lastBishopIdx%2
}

func isLegalBackrank(backrank []rune, lastIdx int) bool {
	return isKingBetweenRooks(backrank, lastIdx) &&
		hasOppositeColorBishops(backrank, lastIdx)
}

func addPermutations(backrank []rune, leftIdx int, lastIdx int,
	uniqFENs map[string]bool) {

	if leftIdx == lastIdx {
		if isLegalBackrank(backrank, lastIdx) {
			fen := fmt.Sprintf("%v/pppppppp/8/8/8/8/PPPPPPPP/%v w KQkq - 0 1",
				string(backrank), strings.ToUpper(string(backrank)))
			uniqFENs[fen] = true
		}
		return
	}

	for ii := leftIdx; ii <= lastIdx; ii++ {
		// swap
		backrank[leftIdx], backrank[ii] = backrank[ii], backrank[leftIdx]

		addPermutations(backrank, leftIdx+1, lastIdx, uniqFENs)

		// unswap
		backrank[leftIdx], backrank[ii] = backrank[ii], backrank[leftIdx]
	}
}

func printFENs(uniqFENs map[string]bool) {
	for fen, _ := range uniqFENs {
		fmt.Println(fen)
	}
}

func main() {
	uniqFENs := make(map[string]bool)
	str := "rrnnbbqk"
	backrank := []rune(str)
	addPermutations(backrank, 0, len(backrank)-1, uniqFENs)

	printFENs(uniqFENs)
}
