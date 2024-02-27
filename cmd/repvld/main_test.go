package main

import (
	"errors"
	"os"
	"testing"

	"github.com/notnil/chess"
)

func TestNewRepValidator(t *testing.T) {
	rv := NewRepValidator(chess.White, 0, 0, []string{"foo.pgn", "bar.pgn"},
		"", false, false, 0.04, 0)
	if rv.color != chess.White {
		t.Fatalf("NewRepValidator failed to initialize color")
	}
	if rv.pgnFileList == nil || len(rv.pgnFileList) != 2 {
		t.Fatalf("NewRepValidator failed to initialize pgn file list")
	}
	if rv.moveMap == nil || len(rv.moveMap) != 0 {
		t.Fatalf("NewRepValidator failed to initialize moveMap")
	}
	if rv.gameList == nil || len(rv.gameList) != 0 {
		t.Fatalf("NewRepValidator failed to initialize gameList")
	}
	if rv.whiteConflictList == nil || len(rv.whiteConflictList) != 0 {
		t.Fatalf("NewRepValidator failed to initialize w conflict list")
	}
	if rv.blackConflictList == nil || len(rv.blackConflictList) != 0 {
		t.Fatalf("NewRepValidator failed to initialize w conflict list")
	}
}

func TestLoad(t *testing.T) {
	rv := NewRepValidator(chess.White, 0, 0, []string{"../../assets/test1.pgn"},
		"", false, false, 0.04, 0)
	err := rv.Load()
	if err != nil {
		t.Fatalf("rv.Load() failed: %v", err)
	}

	if rv.uniquePosCount != 47 {
		t.Fatalf("Expected 47 uniquePosCount but got %v", rv.uniquePosCount)
	}
	if rv.dupPosCount != 61 {
		t.Fatalf("Expected 61 dupPosCount but got %v", rv.dupPosCount)
	}
	if rv.conflictPosCount != 13 {
		t.Fatalf("Expected 13 conflictPosCount but got %v", rv.conflictPosCount)
	}
	if len(rv.whiteConflictList) != 3 {
		t.Fatalf("Expected 3 whiteConflicts but got %v", len(rv.whiteConflictList))
	}
	if len(rv.blackConflictList) != 10 {
		t.Fatalf("Expected 10 blackConflicts but got %v", len(rv.blackConflictList))
	}
	if rv.whiteConflictList[0].existingMove.move != "Qe2" ||
		rv.whiteConflictList[0].conflictMove.move != "Bf4" {
		t.Fatalf("1st conflict does not match")
	}
	if rv.whiteConflictList[1].existingMove.move != "f4" ||
		rv.whiteConflictList[1].conflictMove.move != "Bd3" {
		t.Fatalf("2nd conflict does not match existing:%v conflict:%v",
			rv.whiteConflictList[1].existingMove.move,
			rv.whiteConflictList[1].conflictMove.move)
	}
	if rv.whiteConflictList[2].existingMove.move != "O-O" ||
		rv.whiteConflictList[2].conflictMove.move != "e5" {
		t.Fatalf("3rd conflict does not match existing:%v conflict:%v",
			rv.whiteConflictList[2].existingMove.move,
			rv.whiteConflictList[2].conflictMove.move)
	}
}

func TestMissingFileLoad(t *testing.T) {
	rv := NewRepValidator(chess.Black, 0, 0, []string{"bogus.pgn"}, "", false, false, 0.04, 0)
	err := rv.Load()
	if err == nil {
		t.Fatalf("rv.Load() succeeded but should have failed")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rv.Load() failed as expected but not with correct error value: %v", err)
	}
}

func TestCorruptFileLoad(t *testing.T) {
	rv := NewRepValidator(chess.Black, 0, 0, []string{"../../assets/test2.pgn"},
		"", false, false, 0.04, 0)
	err := rv.Load()
	if err == nil {
		t.Fatalf("rv.Load() succeeded but should have failed")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rv.Load() failed as expected but not with correct error value: %v", err)
	}
}
