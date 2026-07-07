package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadUpgradeConfirmation(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "empty line uses default yes", in: "\n", want: true},
		{name: "whitespace line uses default yes", in: "  \t\n", want: true},
		{name: "EOF uses default yes", in: "", want: true},
		{name: "yes", in: "Y\n", want: true},
		{name: "lowercase yes", in: "y\n", want: true},
		{name: "yes word", in: "yes\n", want: true},
		{name: "no", in: "N\n", want: false},
		{name: "lowercase no", in: "n\n", want: false},
		{name: "no word", in: "no\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readUpgradeConfirmation(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("readUpgradeConfirmation() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("readUpgradeConfirmation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadUpgradeConfirmationReturnsReadError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := readUpgradeConfirmation(errorReader{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("readUpgradeConfirmation() error = %v, want %v", err, wantErr)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}
