/* Copyright © 2021-2024 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this package for license terms
 */
package chesstools

import (
	"fmt"
	"strings"
)

var startFENs960G map[string]bool

func init960() {
	startFENs960G = make(map[string]bool)
	str := "rrnnbbqk"
	backrank := []rune(str)
	addPermutations(backrank, 0, len(backrank)-1, startFENs960G)
}

func Get960StartFENs() []string {
	outFENs := make([]string, 0)
	for fen, _ := range startFENs960G {
		outFENs = append(outFENs, fen)
	}

	return outFENs
}

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
	startFENs960L map[string]bool) {

	if leftIdx == lastIdx {
		if isLegalBackrank(backrank, lastIdx) {
			fen := fmt.Sprintf("%v/pppppppp/8/8/8/8/PPPPPPPP/%v w KQkq - 0 1",
				string(backrank), strings.ToUpper(string(backrank)))
			startFENs960L[fen] = true
		}
		return
	}

	for ii := leftIdx; ii <= lastIdx; ii++ {
		// swap
		backrank[leftIdx], backrank[ii] = backrank[ii], backrank[leftIdx]

		addPermutations(backrank, leftIdx+1, lastIdx, startFENs960L)

		// unswap
		backrank[leftIdx], backrank[ii] = backrank[ii], backrank[leftIdx]
	}
}
