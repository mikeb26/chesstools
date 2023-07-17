package main

import (
	"testing"
)

func TestCtSplunk(t *testing.T) {
	opts := SplunkOpts{
		fenColorList: "r2b1rk1/1b1Q1pp1/p3p1np/n7/B3P3/5N2/PP3PPP/2R2RK1 w - - 1 17:white,rnbqkbnr/pppppppp/8/8/3P4/8/PPP1PPPP/RNBQKBNR b KQkq - 0 1:black,r2q1rk1/ppp3pp/2nbb1n1/3pppP1/5P2/1P2P2P/PBPPN1B1/RN1QK2R w KQ - 1 10:black",
	}

	playerList, err := mainWork(&opts)
	if err != nil {
		t.Fatalf("ctsplunk main test failed with err: %v", err)
	}
	if len(playerList) != 1 {
		t.Fatalf("ctsplunk main test failed; unexpected playerList len %v", len(playerList))
	}
	if playerList[0] != "MassterofMayhem" {
		t.Fatalf("ctsplunk main test failed; unexpected player %v", playerList[0])
	}
}
