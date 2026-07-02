# chesstools (`ct`)

[![Release](https://img.shields.io/github/v/release/mikeb26/chesstools)](https://github.com/mikeb26/chesstools/releases)
[![CircleCI](https://img.shields.io/circleci/build/github/mikeb26/chesstools/main?label=CircleCI)](https://app.circleci.com/pipelines/github/mikeb26/chesstools)
[![Go Reference](https://pkg.go.dev/badge/github.com/mikeb26/chesstools.svg)](https://pkg.go.dev/github.com/mikeb26/chesstools)
[![License: MIT](https://img.shields.io/github/license/mikeb26/chesstools)](LICENSE)

`chesstools` is a Go library and a single command-line tool, `ct`, for working with chess PGNs, FENs, openings, repertoires, Lichess data, and Stockfish evaluations.

It is especially useful for players who maintain opening repertoires, analyze recurring positions, or need scriptable PGN/FEN utilities.

## Features

- Convert PGNs to FENs, including move ranges, colors, and PGN variations.
- Render FEN positions as terminal-friendly ASCII boards.
- Generate all legal Chess960 starting FENs.
- Look up ECO codes and opening names from embedded Lichess opening data.
- Evaluate a FEN or PGN position with Stockfish, with local and Lichess cloud cache support.
- Build opening repertoires from Lichess Explorer data, existing PGNs, and optional engine-selected moves.
- Validate repertoires for transpositional consistency, book gaps, and optional engine recommendations.
- Filter PGN files by player or position.
- Search Lichess games to find players who reached specified positions.
- Fetch PGNs directly from Lichess game and study URLs where a command accepts PGN input.

## Installation

### Download the latest Linux release binary

```bash
mkdir -p "$HOME/bin"
curl -L https://github.com/mikeb26/chesstools/releases/latest/download/ct -o "$HOME/bin/ct"
chmod 755 "$HOME/bin/ct"
# Add $HOME/bin to your PATH if it is not already there.
```

### Install with Go

```bash
go install github.com/mikeb26/chesstools/cmd/ct@latest
```

### Install with Brew

```bash
brew install mikeb26/tap/chesstools
```

### Build from source

```bash
git clone https://github.com/mikeb26/chesstools.git
cd chesstools
make
```

## Quick start

```sh
# See available commands
ct --help

# Print the final FEN from a PGN
ct pgn2fen game.pgn

# Print every FEN from a PGN read from stdin
cat game.pgn | ct pgn2fen --all

# Render a board
ct fencat "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

# Evaluate a position
ct eval --fen "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1" --depth 12

# Validate a white repertoire PGN
ct repvld --color white repertoire.pgn

# Create a new repertoire interactively
ct repmk --color white --output white-repertoire.pgn
```

Use `ct help <command>` or `ct <command> --help` for command-specific options.

## Commands

| Command | What it does |
| --- | --- |
| `ct 960gen` | Prints legal Chess960 starting FENs, one per line. |
| `ct eval` | Evaluates a FEN, a PGN position, or a file/stdin list of FENs with Stockfish/cache support. |
| `ct fencat` | Renders one or more FENs as ASCII boards. |
| `ct pgn2fen` | Converts PGN games to final positions or selected position ranges. Supports stdin and variation expansion. |
| `ct pgnfilt` | Filters PGNs by normalized FEN or by the White player tag. |
| `ct pgnmk` | Interactively creates PGN files with tags, clocks, results, ECO, and opening names. |
| `ct repmk` | Builds opening repertoire PGNs using Lichess Explorer data, optional existing repertoire input, and optional engine move selection. |
| `ct repvld` | Validates repertoire consistency across transpositions and reports gaps; can optionally compare repertoire moves to Stockfish. |
| `ct splunk` | Finds players who reached specified FEN/color combinations and prints sample Lichess games. |
| `ct upgrade` | Upgrades the installed `ct` binary to the latest GitHub release, or uses Homebrew for Homebrew installs. |
| `ct version` | Prints the embedded `ct` version. |

## Examples

### Convert PGNs to FENs

```sh
# Final position of each game
ct pgn2fen games.pgn

# Every position, including PGN variations
ct pgn2fen --all --includevar games.pgn

# Only positions where Black is to move between moves 5 and 12
ct pgn2fen --color black --startmove 5 --endmove 12 games.pgn

# A Lichess game or study URL can be used where a PGN file is expected
ct pgn2fen https://lichess.org/abcdefgh
ct pgn2fen https://lichess.org/study/abcdefgh
```

### Evaluate positions

```sh
# Evaluate one FEN for 30 seconds
ct eval --fen "<fen>" --time 30

# Evaluate one FEN to a fixed depth
ct eval --fen "<fen>" --depth 16

# Evaluate a position from a PGN after White's 12th move
ct eval --pgn game.pgn --move 12 --turn white

# Evaluate a list of FENs from a file or stdin
ct eval --fenfile positions.txt --depth 10
cat positions.txt | ct eval --fenfile - --cacheonly
```

Evaluation results are cached under the user's config directory by default, typically `~/.config/chesstools/cache` on Linux. `ct eval` can also read Lichess cloud evaluations unless `--nocloudcache` is set.

### Work with repertoires

```sh
# Validate that a repertoire always chooses the same move in transposed positions
ct repvld --color white white-repertoire.pgn

# Validate and ask Stockfish to flag moves that differ from its best move
ct repvld --color black --depth 14 black-repertoire.pgn

# Build a repertoire from Lichess Explorer data, starting after a move sequence
ct repmk --color white --start "1. e4 c5 2. Nf3" --output sicilian-repertoire.pgn

# Build in consolidated format, preserving an existing repertoire
ct repmk --color black --input black-current.pgn --output black-next.pgn --format consolidated
```

## External dependencies and API access

Some commands work fully offline, but analysis and Lichess-backed workflows need extra setup:

- **Stockfish** must be installed as `stockfish` on `PATH` for `ct eval`, engine-selected repertoire building, and Stockfish-backed validation/scoring.
- **Lichess APIs** are used by opening explorer, cloud evaluation, crosstable, game export, and study export code. Some Explorer requests require a token:

  ```sh
  export LICHESS_TOKEN=<your-lichess-token>
  ```

- Lichess rate limits are respected by waiting and retrying after HTTP 429 responses.

## Library usage

The root module can also be imported by Go programs:

```go
package main

import (
    "fmt"

    "github.com/mikeb26/chesstools"
)

func main() {
    normalized, err := chesstools.NormalizeFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 7 23")
    if err != nil {
        panic(err)
    }

    fmt.Println(normalized)
    fmt.Println(len(chesstools.Get960StartFENs()))
}
```

See the [Go package documentation](https://pkg.go.dev/github.com/mikeb26/chesstools) for exported APIs.

## Development

```sh
# Run all tests. Some integration tests skip unless Stockfish/LICHESS_TOKEN are available.
go test ./...

# Run a narrower set of fast/package tests
go test ./cmd/ct/pgn2fen ./cmd/ct/eval ./cmd/ct/repvld

# Build the CLI
go build ./cmd/ct
```

The `Makefile` can also build a local `./ct` binary. It manages vendoring for build targets, while direct `go test`/`go build` commands work in module mode.

Project layout:

```text
.
├── *.go                 # root chesstools library package
├── cmd/ct               # ct CLI entrypoint and subcommands
├── eco                  # ECO/opening TSV data embedded by openings.go
├── assets               # PGN fixtures
└── cmd/ct/*/tests       # command-specific test fixtures
```

## Contributing

Issues and pull requests are welcome. For code changes:

1. Keep changes focused and preserve existing CLI output/flag behavior unless the change is intentional.
2. Run `gofmt` on modified Go files.
3. Run the narrowest relevant tests first, then broader tests when your environment has the needed dependencies.
4. Do not commit local caches, secrets, large PGN databases, Stockfish binaries, or generated vendor artifacts unless explicitly required.

## License

`chesstools` is released under the [MIT License](LICENSE).
