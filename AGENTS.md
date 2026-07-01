# AGENTS.md

## Scope
These instructions apply to the entire repository.

## Project overview
`chesstools` is a Go module (`github.com/mikeb26/chesstools`) containing chess utilities and command-line tools for PGN/FEN processing, repertoire creation/validation, opening lookup, crosstable queries, and Stockfish/Lichess-based evaluation.

Key areas:
- Root package (`*.go`): shared library code for Chess960 starts, PGN opening, FEN normalization, ECO/opening lookup, Lichess APIs, crosstables, and engine/cache evaluation.
- `cmd/*`: CLI tools (`960gen`, `cteval`, `ctsplunk`, `fencat`, `pgn2fen`, `pgnfilt`, `pgnmk`, `repmk`, `repvld`).
- `eco/`: ECO/opening TSV inputs and `build.sh`; `eco/all_fen.tsv` is embedded by `openings.go` via `//go:embed`.
- `assets/` and `cmd/repmk/tests/`: test PGNs/fixtures.

## Development guidelines
- Use Go 1.22.x semantics; keep `go.mod` and `go.sum` consistent.
- Run `gofmt` on any modified `.go` file before finishing.
- Prefer small, focused changes. Preserve existing CLI flags and output formats unless explicitly asked to change them.
- Keep generated/local artifacts out of git. Important ignored outputs include command binaries, `cache/`, `vendor/`, root-level `*.pgn`, `exceptions.json`, and `score_all.sh`.
- Do not edit ECO TSV data or regenerate `eco/all_fen.tsv` unless the task is specifically about opening data.
- Network-facing code talks to Lichess and may rate-limit/sleep on HTTP 429. Avoid adding tests that depend on live network unless clearly marked or gated.

## Build and test commands
Useful commands from a clean checkout:

```sh
go test ./...
go test ./cmd/pgn2fen ./cmd/repvld ./cmd/repmk ./cmd/ctsplunk

go build ./cmd/pgn2fen
go build ./cmd/cteval
go build ./cmd/repmk
go build ./cmd/repvld
```

The `Makefile` exports `GOFLAGS=-mod=vendor`. If `vendor/` is absent or stale, `make test`/`make build` can fail with inconsistent vendoring. Either run `go mod vendor` first when using `make`, or use direct `go test`/`go build` commands without the Makefile for module-mode work.

## External dependencies and test caveats
Some packages/tests require external services or binaries:
- Stockfish must be installed and available as `stockfish` on `PATH` for engine evaluation paths (`cteval`, `repmk` engine selection, scoring tests, cache upgrades).
- Lichess opening/crosstable/cloud eval APIs are used by several code paths. Some current Lichess Explorer requests require `LICHESS_TOKEN` to be set.
- Full `go test ./...` may fail in minimal/offline environments because `cmd/ctsplunk`, `cmd/repmk`, and `cmd/repvld` tests can hit Lichess APIs and/or require Stockfish. Prefer targeted tests for packages touched by a change when those dependencies are unavailable.

When reporting test results, mention any failures caused by missing `stockfish`, missing `LICHESS_TOKEN`, live network/API responses, or stale vendoring.

## CLI notes
- Most commands use Go's standard `flag` package and print errors to stderr before exiting nonzero.
- `cteval` evaluates a PGN position or FEN and maintains local cache files under `cache/`.
- `repmk` can run interactively, call Lichess Explorer, and optionally use Stockfish for engine-selected repertoire moves.
- `repvld` validates repertoire consistency and can optionally score moves with Stockfish.
- `pgn2fen` supports stdin when no PGN files are provided.

## Repository hygiene for agents
- Before editing, inspect the relevant files rather than guessing.
- After editing Go code, run `gofmt` and the narrowest relevant tests first.
- Do not commit secrets, API tokens, large PGN databases, engine binaries, generated caches, or vendored dependencies unless explicitly requested.
