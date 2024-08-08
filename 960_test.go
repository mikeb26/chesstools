package chesstools

import (
	"testing"
)

func TestBishops(t *testing.T) {
	backrank := []rune("rrnnbbqk")
	lastIdx := len(backrank) - 1

	if !hasOppositeColorBishops(backrank, lastIdx) {
		t.Fatalf("backrank has opposite bishops but hasOppositeColorBishops() failed")
	}

	backrank = []rune("rrnnbqbk")
	if hasOppositeColorBishops(backrank, lastIdx) {
		t.Fatalf("backrank does not have opposite bishops but hasOppositeColorBishops() failed")
	}
}

func TestKing(t *testing.T) {
	backrank := []rune("rrnnbbqk")
	lastIdx := len(backrank) - 1

	if isKingBetweenRooks(backrank, lastIdx) {
		t.Fatalf("backrank has both rooks before king but isKingBetweenRooks() returned true")
	}

	backrank = []rune("krrnnbbq")

	if isKingBetweenRooks(backrank, lastIdx) {
		t.Fatalf("backrank has king before both rooks but isKingBetweenRooks() returned true")
	}

	backrank = []rune("nnrbqbkr")
	if !isKingBetweenRooks(backrank, lastIdx) {
		t.Fatalf("backrank has king between rooks but hasOppositeColorBishops()	returned false")
	}
}
