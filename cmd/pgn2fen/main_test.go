package main

import (
	"strings"
	"testing"

	"github.com/corentings/chess/v2"
)

func loadPgn(t *testing.T) *chess.Game {
	pgn := `1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 4. Ba4 Nge7 5. Nc3 d6 6. O-O Rb8 7. h3 b5 8. Bb3 Na5 9. Re1 Ng6 10. d4 Nxb3 11. axb3 f6 12. dxe5 fxe5 13. Nd5 Be7 14. Bd2 Qd7 15. h4 h6 16. h5 Nf8 17. Ba5 Bd8 18. Nh2 Nh7 19. f4 O-O 20. fxe5 dxe5 21. Nxc7 Nf6 22. Qxd7 Bxd7 23. Nxa6 Rc8 24. Bc3 Bb6+ 25. Kh1 Nxh5 26. Nf3 Bf2 27. Red1 Bg4 28. Nb4 Ng3+ 29. Kh2 Nxe4 30. Bxe5 Rce8 31. Ra7 Rxe5 32. Nxe5 Bg3+ 33. Kh1 Bxd1 0-1`

	pgnArgs, err := chess.PGN(strings.NewReader(pgn))
	if err != nil {
		t.Fatalf("Failed to read pgn: %v", err)
	}

	return chess.NewGame(pgnArgs)
}

func TestFinalPositionOnly(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()

	expectedFENs := "5rk1/R5p1/7p/1p2N3/1N2n3/1P4b1/1PP3P1/3b3K w - - 0 34\n"
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestAllPositions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.all = true

	expectedFENs := `rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1
rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2
rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2
r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3
r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 3 3
r1bqkbnr/1ppp1ppp/p1n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 0 4
r1bqkbnr/1ppp1ppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 1 4
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 2 5
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R b KQkq - 3 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R w KQkq - 0 6
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 b kq - 1 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 w k - 2 7
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 b k - 0 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 w k - 0 8
1rbqkb1r/2p1nppp/p1np4/1p2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 b k - 1 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 w k - 2 9
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 b k - 3 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 w k - 4 10
1rbqkb1r/2p2ppp/p2p2n1/np2p3/3PP3/1BN2N1P/PPP2PP1/R1BQR1K1 b k - 0 10
1rbqkb1r/2p2ppp/p2p2n1/1p2p3/3PP3/1nN2N1P/PPP2PP1/R1BQR1K1 w k - 0 11
1rbqkb1r/2p2ppp/p2p2n1/1p2p3/3PP3/1PN2N1P/1PP2PP1/R1BQR1K1 b k - 0 11
1rbqkb1r/2p3pp/p2p1pn1/1p2p3/3PP3/1PN2N1P/1PP2PP1/R1BQR1K1 w k - 0 12
1rbqkb1r/2p3pp/p2p1pn1/1p2P3/4P3/1PN2N1P/1PP2PP1/R1BQR1K1 b k - 0 12
1rbqkb1r/2p3pp/p2p2n1/1p2p3/4P3/1PN2N1P/1PP2PP1/R1BQR1K1 w k - 0 13
1rbqkb1r/2p3pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PP2PP1/R1BQR1K1 b k - 1 13
1rbqk2r/2p1b1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PP2PP1/R1BQR1K1 w k - 2 14
1rbqk2r/2p1b1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PPB1PP1/R2QR1K1 b k - 3 14
1rb1k2r/2pqb1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PPB1PP1/R2QR1K1 w k - 4 15
1rb1k2r/2pqb1pp/p2p2n1/1p1Np3/4P2P/1P3N2/1PPB1PP1/R2QR1K1 b k - 0 15
1rb1k2r/2pqb1p1/p2p2np/1p1Np3/4P2P/1P3N2/1PPB1PP1/R2QR1K1 w k - 0 16
1rb1k2r/2pqb1p1/p2p2np/1p1Np2P/4P3/1P3N2/1PPB1PP1/R2QR1K1 b k - 0 16
1rb1kn1r/2pqb1p1/p2p3p/1p1Np2P/4P3/1P3N2/1PPB1PP1/R2QR1K1 w k - 1 17
1rb1kn1r/2pqb1p1/p2p3p/Bp1Np2P/4P3/1P3N2/1PP2PP1/R2QR1K1 b k - 2 17
1rbbkn1r/2pq2p1/p2p3p/Bp1Np2P/4P3/1P3N2/1PP2PP1/R2QR1K1 w k - 3 18
1rbbkn1r/2pq2p1/p2p3p/Bp1Np2P/4P3/1P6/1PP2PPN/R2QR1K1 b k - 4 18
1rbbk2r/2pq2pn/p2p3p/Bp1Np2P/4P3/1P6/1PP2PPN/R2QR1K1 w k - 5 19
1rbbk2r/2pq2pn/p2p3p/Bp1Np2P/4PP2/1P6/1PP3PN/R2QR1K1 b k - 0 19
1rbb1rk1/2pq2pn/p2p3p/Bp1Np2P/4PP2/1P6/1PP3PN/R2QR1K1 w - - 1 20
1rbb1rk1/2pq2pn/p2p3p/Bp1NP2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 20
1rbb1rk1/2pq2pn/p6p/Bp1Np2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 0 21
1rbb1rk1/2Nq2pn/p6p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 21
1rbb1rk1/2Nq2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 1 22
1rbb1rk1/2NQ2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 22
1r1b1rk1/2Nb2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 0 23
1r1b1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 23
2rb1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 1 24
2rb1rk1/3b2p1/N4n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 b - - 2 24
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 w - - 3 25
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R2K b - - 4 25
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB5/1PP3PN/R3R2K w - - 0 26
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB2N2/1PP3P1/R3R2K b - - 1 26
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R3R2K w - - 2 27
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R2R3K b - - 3 27
2r2rk1/6p1/N6p/1p2p2n/4P1b1/1PB2N2/1PP2bP1/R2R3K w - - 4 28
2r2rk1/6p1/7p/1p2p2n/1N2P1b1/1PB2N2/1PP2bP1/R2R3K b - - 5 28
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bP1/R2R3K w - - 6 29
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bPK/R2R4 b - - 7 29
2r2rk1/6p1/7p/1p2p3/1N2n1b1/1PB2N2/1PP2bPK/R2R4 w - - 0 30
2r2rk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 b - - 0 30
4rrk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 w - - 1 31
4rrk1/R5p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/3R4 b - - 2 31
5rk1/R5p1/7p/1p2r3/1N2n1b1/1P3N2/1PP2bPK/3R4 w - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P6/1PP2bPK/3R4 b - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3PK/3R4 w - - 1 33
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3P1/3R3K b - - 2 33
5rk1/R5p1/7p/1p2N3/1N2n3/1P4b1/1PP3P1/3b3K w - - 0 34
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestAllWhitePositions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.all = true
	opts.colorc = chess.White

	expectedFENs := `rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2
r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3
r1bqkbnr/1ppp1ppp/p1n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 0 4
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 2 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R w KQkq - 0 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 w k - 2 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 w k - 0 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 w k - 2 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 w k - 4 10
1rbqkb1r/2p2ppp/p2p2n1/1p2p3/3PP3/1nN2N1P/PPP2PP1/R1BQR1K1 w k - 0 11
1rbqkb1r/2p3pp/p2p1pn1/1p2p3/3PP3/1PN2N1P/1PP2PP1/R1BQR1K1 w k - 0 12
1rbqkb1r/2p3pp/p2p2n1/1p2p3/4P3/1PN2N1P/1PP2PP1/R1BQR1K1 w k - 0 13
1rbqk2r/2p1b1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PP2PP1/R1BQR1K1 w k - 2 14
1rb1k2r/2pqb1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PPB1PP1/R2QR1K1 w k - 4 15
1rb1k2r/2pqb1p1/p2p2np/1p1Np3/4P2P/1P3N2/1PPB1PP1/R2QR1K1 w k - 0 16
1rb1kn1r/2pqb1p1/p2p3p/1p1Np2P/4P3/1P3N2/1PPB1PP1/R2QR1K1 w k - 1 17
1rbbkn1r/2pq2p1/p2p3p/Bp1Np2P/4P3/1P3N2/1PP2PP1/R2QR1K1 w k - 3 18
1rbbk2r/2pq2pn/p2p3p/Bp1Np2P/4P3/1P6/1PP2PPN/R2QR1K1 w k - 5 19
1rbb1rk1/2pq2pn/p2p3p/Bp1Np2P/4PP2/1P6/1PP3PN/R2QR1K1 w - - 1 20
1rbb1rk1/2pq2pn/p6p/Bp1Np2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 0 21
1rbb1rk1/2Nq2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 1 22
1r1b1rk1/2Nb2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 0 23
2rb1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 1 24
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 w - - 3 25
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB5/1PP3PN/R3R2K w - - 0 26
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R3R2K w - - 2 27
2r2rk1/6p1/N6p/1p2p2n/4P1b1/1PB2N2/1PP2bP1/R2R3K w - - 4 28
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bP1/R2R3K w - - 6 29
2r2rk1/6p1/7p/1p2p3/1N2n1b1/1PB2N2/1PP2bPK/R2R4 w - - 0 30
4rrk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 w - - 1 31
5rk1/R5p1/7p/1p2r3/1N2n1b1/1P3N2/1PP2bPK/3R4 w - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3PK/3R4 w - - 1 33
5rk1/R5p1/7p/1p2N3/1N2n3/1P4b1/1PP3P1/3b3K w - - 0 34
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestAllBlackPositions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.all = true
	opts.colorc = chess.Black

	expectedFENs := `rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1
rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2
r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 3 3
r1bqkbnr/1ppp1ppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 1 4
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R b KQkq - 3 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 b kq - 1 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 b k - 0 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 b k - 1 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 b k - 3 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/3PP3/1BN2N1P/PPP2PP1/R1BQR1K1 b k - 0 10
1rbqkb1r/2p2ppp/p2p2n1/1p2p3/3PP3/1PN2N1P/1PP2PP1/R1BQR1K1 b k - 0 11
1rbqkb1r/2p3pp/p2p1pn1/1p2P3/4P3/1PN2N1P/1PP2PP1/R1BQR1K1 b k - 0 12
1rbqkb1r/2p3pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PP2PP1/R1BQR1K1 b k - 1 13
1rbqk2r/2p1b1pp/p2p2n1/1p1Np3/4P3/1P3N1P/1PPB1PP1/R2QR1K1 b k - 3 14
1rb1k2r/2pqb1pp/p2p2n1/1p1Np3/4P2P/1P3N2/1PPB1PP1/R2QR1K1 b k - 0 15
1rb1k2r/2pqb1p1/p2p2np/1p1Np2P/4P3/1P3N2/1PPB1PP1/R2QR1K1 b k - 0 16
1rb1kn1r/2pqb1p1/p2p3p/Bp1Np2P/4P3/1P3N2/1PP2PP1/R2QR1K1 b k - 2 17
1rbbkn1r/2pq2p1/p2p3p/Bp1Np2P/4P3/1P6/1PP2PPN/R2QR1K1 b k - 4 18
1rbbk2r/2pq2pn/p2p3p/Bp1Np2P/4PP2/1P6/1PP3PN/R2QR1K1 b k - 0 19
1rbb1rk1/2pq2pn/p2p3p/Bp1NP2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 20
1rbb1rk1/2Nq2pn/p6p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 21
1rbb1rk1/2NQ2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 22
1r1b1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 23
2rb1rk1/3b2p1/N4n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 b - - 2 24
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R2K b - - 4 25
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB2N2/1PP3P1/R3R2K b - - 1 26
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R2R3K b - - 3 27
2r2rk1/6p1/7p/1p2p2n/1N2P1b1/1PB2N2/1PP2bP1/R2R3K b - - 5 28
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bPK/R2R4 b - - 7 29
2r2rk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 b - - 0 30
4rrk1/R5p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/3R4 b - - 2 31
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P6/1PP2bPK/3R4 b - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3P1/3R3K b - - 2 33
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestStartMove20Position(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.startMoveNum = 20

	expectedFENs := `1rbb1rk1/2pq2pn/p2p3p/Bp1Np2P/4PP2/1P6/1PP3PN/R2QR1K1 w - - 1 20
1rbb1rk1/2pq2pn/p2p3p/Bp1NP2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 20
1rbb1rk1/2pq2pn/p6p/Bp1Np2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 0 21
1rbb1rk1/2Nq2pn/p6p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 b - - 0 21
1rbb1rk1/2Nq2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R2QR1K1 w - - 1 22
1rbb1rk1/2NQ2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 22
1r1b1rk1/2Nb2p1/p4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 0 23
1r1b1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 b - - 0 23
2rb1rk1/3b2p1/N4n1p/Bp2p2P/4P3/1P6/1PP3PN/R3R1K1 w - - 1 24
2rb1rk1/3b2p1/N4n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 b - - 2 24
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R1K1 w - - 3 25
2r2rk1/3b2p1/Nb3n1p/1p2p2P/4P3/1PB5/1PP3PN/R3R2K b - - 4 25
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB5/1PP3PN/R3R2K w - - 0 26
2r2rk1/3b2p1/Nb5p/1p2p2n/4P3/1PB2N2/1PP3P1/R3R2K b - - 1 26
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R3R2K w - - 2 27
2r2rk1/3b2p1/N6p/1p2p2n/4P3/1PB2N2/1PP2bP1/R2R3K b - - 3 27
2r2rk1/6p1/N6p/1p2p2n/4P1b1/1PB2N2/1PP2bP1/R2R3K w - - 4 28
2r2rk1/6p1/7p/1p2p2n/1N2P1b1/1PB2N2/1PP2bP1/R2R3K b - - 5 28
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bP1/R2R3K w - - 6 29
2r2rk1/6p1/7p/1p2p3/1N2P1b1/1PB2Nn1/1PP2bPK/R2R4 b - - 7 29
2r2rk1/6p1/7p/1p2p3/1N2n1b1/1PB2N2/1PP2bPK/R2R4 w - - 0 30
2r2rk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 b - - 0 30
4rrk1/6p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/R2R4 w - - 1 31
4rrk1/R5p1/7p/1p2B3/1N2n1b1/1P3N2/1PP2bPK/3R4 b - - 2 31
5rk1/R5p1/7p/1p2r3/1N2n1b1/1P3N2/1PP2bPK/3R4 w - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P6/1PP2bPK/3R4 b - - 0 32
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3PK/3R4 w - - 1 33
5rk1/R5p1/7p/1p2N3/1N2n1b1/1P4b1/1PP3P1/3R3K b - - 2 33
5rk1/R5p1/7p/1p2N3/1N2n3/1P4b1/1PP3P1/3b3K w - - 0 34
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestEndMove10Positions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.endMoveNum = 10

	expectedFENs := `rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1
rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2
rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2
r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3
r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 3 3
r1bqkbnr/1ppp1ppp/p1n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 0 4
r1bqkbnr/1ppp1ppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 1 4
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 2 5
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R b KQkq - 3 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R w KQkq - 0 6
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 b kq - 1 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 w k - 2 7
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 b k - 0 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 w k - 0 8
1rbqkb1r/2p1nppp/p1np4/1p2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 b k - 1 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 w k - 2 9
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 b k - 3 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 w k - 4 10
1rbqkb1r/2p2ppp/p2p2n1/np2p3/3PP3/1BN2N1P/PPP2PP1/R1BQR1K1 b k - 0 10
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestStartEndMovePositions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.startMoveNum = 5
	opts.endMoveNum = 10

	expectedFENs := `r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 2 5
r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R b KQkq - 3 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R w KQkq - 0 6
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 b kq - 1 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 w k - 2 7
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 b k - 0 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 w k - 0 8
1rbqkb1r/2p1nppp/p1np4/1p2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 b k - 1 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 w k - 2 9
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 b k - 3 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 w k - 4 10
1rbqkb1r/2p2ppp/p2p2n1/np2p3/3PP3/1BN2N1P/PPP2PP1/R1BQR1K1 b k - 0 10
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}

func TestStartEndWithColorMovePositions(t *testing.T) {

	g := loadPgn(t)
	opts := NewPgn2FenOpts()
	opts.startMoveNum = 5
	opts.endMoveNum = 10
	opts.colorc = chess.Black

	expectedFENs := `r1bqkb1r/1pppnppp/p1n5/4p3/B3P3/2N2N2/PPPP1PPP/R1BQK2R b KQkq - 3 5
r1bqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N2/PPPP1PPP/R1BQ1RK1 b kq - 1 6
1rbqkb1r/1pp1nppp/p1np4/4p3/B3P3/2N2N1P/PPPP1PP1/R1BQ1RK1 b k - 0 7
1rbqkb1r/2p1nppp/p1np4/1p2p3/4P3/1BN2N1P/PPPP1PP1/R1BQ1RK1 b k - 1 8
1rbqkb1r/2p1nppp/p2p4/np2p3/4P3/1BN2N1P/PPPP1PP1/R1BQR1K1 b k - 3 9
1rbqkb1r/2p2ppp/p2p2n1/np2p3/3PP3/1BN2N1P/PPP2PP1/R1BQR1K1 b k - 0 10
`
	fens := game2FENs(opts, g)
	if fens != expectedFENs {
		t.Fatalf("Expected %v got %v", expectedFENs, fens)
	}
}
