package main

import (
	"strings"
	"testing"
)

func TestEval(t *testing.T) {
}

func TestLoadFENs(t *testing.T) {
	positions, err := loadFENs(strings.NewReader(`
# comment
rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1
8/8/8/8/8/8/8/8 w - - 0 1
`), "test")
	if err != nil {
		t.Fatalf("loadFENs failed: %v", err)
	}
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %v", len(positions))
	}
	if positions[0].label != "test:3" {
		t.Fatalf("expected first label test:3, got %v", positions[0].label)
	}
}

func TestLoadFENsInvalidFEN(t *testing.T) {
	_, err := loadFENs(strings.NewReader("not a fen\n"), "test")
	if err == nil {
		t.Fatalf("expected invalid FEN error")
	}
	if !strings.Contains(err.Error(), "test:1 invalid FEN") {
		t.Fatalf("expected error to include line number, got %v", err)
	}
}

func TestLoadFENsEmpty(t *testing.T) {
	_, err := loadFENs(strings.NewReader("\n# comment\n"), "test")
	if err == nil {
		t.Fatalf("expected empty FEN file error")
	}
	if !strings.Contains(err.Error(), "no FEN positions found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
