package main

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/corentings/chess/v2"
)

func TestRepBldBasic(t *testing.T) {
	tmpConsolidatedFile, err := os.CreateTemp("", "repbldtestc-*")
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	tmpConsolidatedFile.Close()
	defer os.Remove(tmpConsolidatedFile.Name())
	tmpFlattenedFile, err := os.CreateTemp("", "repbldtestf-*")
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	tmpFlattenedFile.Close()
	defer os.Remove(tmpFlattenedFile.Name())

	opts := RepBldOpts{
		color:        chess.Black,
		threshold:    0.99,
		maxDepth:     14,
		engineSelect: true,
		engineTime:   300,
		inputFile:    "tests/test1.input.pgn",
		outputFile:   tmpConsolidatedFile.Name(),
		outputMode:   Consolidated,
		keepExisting: true,
		expandVar:    true,
		noAtime:      true,
	}
	mainWork(&opts)
	opts.outputFile = tmpFlattenedFile.Name()
	opts.outputMode = Flattened
	mainWork(&opts)

	tmpConsolidatedFile2, err := os.Open(tmpConsolidatedFile.Name())
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	defer tmpConsolidatedFile2.Close()
	tmpFlattenedFile2, err := os.Open(tmpFlattenedFile.Name())
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	defer tmpFlattenedFile2.Close()

	expectedConsolidatedPos := []string{
		"rnbqkbnr/pp2pppp/2p5/3p4/2PP4/8/PP2PPPP/RNBQKBNR w KQkq - 0 3",
		"rnbqkb1r/pp2pppp/2p2n2/3p4/2PP4/2N2N2/PP2PPPP/R1BQKB1R b KQkq - 3 4",
		"rnbqkb1r/1p2pppp/p1p2n2/3p4/2PP4/2N2N2/PP2PPPP/R1BQKB1R w KQkq - 0 5",
		"rnbqkb1r/1p3ppp/p3pn2/3p4/3P4/2N1PN2/PP3PPP/R1BQKB1R w KQkq - 0 7",
	}
	err = processOneTestPGN(tmpConsolidatedFile2, tmpConsolidatedFile.Name(), 6,
		expectedConsolidatedPos)
	if err != nil {
		t.Fatalf("Consolidated test failed input:%v err:%v",
			tmpConsolidatedFile.Name(), err)
	}

	expectedFlattenedPos := []string{
		"rnbqkb1r/1p3ppp/p3pn2/3p4/3P4/2N1PN2/PP3PPP/R1BQKB1R w KQkq - 0 7",
	}
	err = processOneTestPGN(tmpFlattenedFile2, tmpFlattenedFile.Name(), 4,
		expectedFlattenedPos)
	if err != nil {
		t.Fatalf("Flattened test failed: %v", err)
	}

}

func TestRepBldAdv(t *testing.T) {
	fmt.Printf("DEBUG - TestRepBldAdv()\n")
	tmpFlattenedFile, err := os.CreateTemp("", "repbldtestf2-*")
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	tmpFlattenedFile.Close()
	defer os.Remove(tmpFlattenedFile.Name())

	opts := RepBldOpts{
		color:        chess.White,
		threshold:    0.10,
		engineSelect: true,
		engineTime:   300,
		minGames:     1,
		maxDepth:     14,
		outputFile:   tmpFlattenedFile.Name(),
		outputMode:   Flattened,
		startMoves:   "1. e4 c5 2. Nc3 Nc6 3. f4 g6 4. Nf3 Bg7 5. a4 Nf6 6. e5",
		noAtime:      true,
	}
	mainWork(&opts)
	opts.outputFile = tmpFlattenedFile.Name()
	opts.outputMode = Flattened

	tmpFlattenedFile2, err := os.Open(tmpFlattenedFile.Name())
	if err != nil {
		t.Fatalf("Could not open temp file: %v", err)
	}
	defer tmpFlattenedFile2.Close()

	expectedFlattenedPos := []string{
		"r1bqk2r/pp1pppb1/6p1/4Pn1p/P4P2/2N4P/1PP2QP1/R1B1KBR1 b Qkq - 1 12",
		"r1bqk2r/pp2ppbp/3p2pn/4P3/P4PP1/2N1B2P/1PP2Q2/R3KB1R b KQkq - 2 13",
		"r1bqk1nr/pp1pppbp/2n3p1/1Bp1P3/P4P2/2N2N2/1PPP2PP/R1BQK2R b KQkq - 2 7",
		"r1bqk2r/pp1pppbp/2n3p1/4P2n/P2N1P2/2N5/1PP3PP/R1BQKB1R b KQkq - 0 8",
	}
	err = processOneTestPGN(tmpFlattenedFile2, tmpFlattenedFile.Name(),
		len(expectedFlattenedPos), expectedFlattenedPos)
	if err != nil {
		t.Fatalf("Flattened test failed: %v", err)
	}

}

func processOneTestPGN(f io.Reader, pgnFilename string, numGames int,
	expectedPos []string) error {

	scanner := chess.NewScanner(f, chess.WithExpandVariations())

	found := make([]bool, len(expectedPos))

	ii := 1
	gameCount := 0
	for scanner.HasNext() {
		g, err := scanner.ParseNext()
		if err != nil {
			return err
		}
		if len(g.Moves()) == 0 {
			continue
		}

		gameCount++
		for idx, fen := range expectedPos {
			if g.Position().XFENString() == fen {
				found[idx] = true
				break
			}
		}
		ii++
	}

	if gameCount != numGames {
		return fmt.Errorf("Expected game count %v but got %v", numGames,
			gameCount)
	}
	for idx, fen := range expectedPos {
		if found[idx] != true {
			return fmt.Errorf("Missing expected pos: %v", fen)
		}
	}

	return nil
}
