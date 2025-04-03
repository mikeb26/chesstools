package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/notnil/chess"
)

func TestRepBld(t *testing.T) {
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
		inputFile:    "tests/test1.input.pgn",
		outputFile:   tmpConsolidatedFile.Name(),
		outputMode:   Consolidated,
		keepExisting: true,
		expandVar:    true,
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
		t.Fatalf("Consolidated test failed: %v", err)
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

func processOneTestPGN(f io.Reader, pgnFilename string, numGames int,
	expectedPos []string) error {

	var opts chess.ScannerOpts
	opts.ExpandVariations = true

	scanner := chess.NewScannerWithOptions(f, opts)

	found := make([]bool, len(expectedPos))

	var err error
	ii := 1
	gameCount := 0
	for scanner.Scan() {
		g := scanner.Next()
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

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
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

	return err
}
