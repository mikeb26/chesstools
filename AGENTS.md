# AGENTS.md

## Scope
These instructions apply to the entire repository.

## Project overview
`chesstools` is a Go module (`github.com/mikeb26/chesstools`) that provides:

- A root library package (`package chesstools`) for chess utilities.
- A single multi-command CLI binary, `ct`, under `cmd/ct`.

The project focuses on PGN/FEN processing, Chess960 start positions, ECO/opening lookup, Lichess Explorer/crosstable/cloud-eval integrations, local Stockfish evaluation with file-based caching, and opening repertoire creation/validation.

## Current structure

- Root package (`*.go`):
  - `960.go`: legal Chess960 starting FEN generation.
  - `pgnReader.go`: PGN file/Lichess URL opening plus FEN normalization.
  - `openings.go`: embedded ECO/opening lookup and Lichess Explorer access.
  - `eval.go`: Stockfish/cloud evaluation and local cache management.
  - `crosstable.go`: Lichess crosstable lookup.
  - `init.go`: package initialization.
- `cmd/ct`: CLI entrypoint, versioning, self-upgrade logic, and subcommands.
- `cmd/ct/960gen`: prints Chess960 start FENs.
- `cmd/ct/eval`: evaluates FENs or PGN positions; can batch FENs via `--fenfile`.
- `cmd/ct/fencat`: renders FENs as ASCII boards.
- `cmd/ct/pgn2fen`: converts PGN games to FEN positions; supports stdin and variation expansion.
- `cmd/ct/pgnfilt`: filters PGNs by normalized FEN or White tag.
- `cmd/ct/pgnmk`: interactive PGN creation with tags, clocks, results, and opening metadata.
- `cmd/ct/repmk`: repertoire generation from Lichess Explorer data/existing PGNs/optional engine-selected moves.
- `cmd/ct/repvld`: repertoire validation for transpositions, gaps, and optional engine scoring.
- `cmd/ct/splunk`: Lichess-backed search for players who reached specified FEN/color positions.
- `eco/`: ECO/opening TSV inputs, generation script, and embedded `eco/all_fen.tsv`.
- `assets/` and `cmd/ct/repmk/tests/`: test fixtures.

Historical standalone command paths such as `cmd/pgn2fen` are no longer current; commands now live under `cmd/ct/<subcommand>` and are invoked through `ct <subcommand>`.

## Go version and dependencies

- Use Go 1.25 semantics (`go.mod` currently says `go 1.25.0`).
- Keep `go.mod` and `go.sum` consistent.
- The primary Go dependency is `github.com/corentings/chess/v2`.
- Stockfish is an external binary dependency for engine evaluation paths and must be available as `stockfish` on `PATH` when those paths are used.
- Some Lichess Explorer requests require `LICHESS_TOKEN` in the environment. Do not commit tokens or other secrets.

## Development guidelines

- Inspect actual file contents before modifying; do not rely on memory or assumptions.
- Prefer small, focused changes.
- Keep this `AGENTS.md` file up to date when project structure, commands, workflows, dependencies, testing requirements, or agent-facing conventions change.
- Preserve existing CLI flags, command names, stdout/stderr formats, and exit behavior unless explicitly asked to change them.
- Run `gofmt` on any modified `.go` file before finishing.
- Do not edit ECO TSV data or regenerate `eco/all_fen.tsv` unless the task is specifically about opening data.
- Network-facing code talks to Lichess and may sleep/retry on HTTP 429. Avoid adding tests that depend on live network unless clearly marked, gated, or skippable.
- Be aware that several code paths call `log.Fatal`/`os.Exit`; tests around those paths may need subprocess-style handling or refactoring before direct assertions are possible.

## Build and test commands

Useful commands from a clean checkout:

```sh
# Fast/common package tests. Integration tests should skip when dependencies are missing.
go test ./...

# Targeted tests for actively maintained command packages.
go test ./cmd/ct/pgn2fen ./cmd/ct/eval ./cmd/ct/repvld ./cmd/ct/repmk ./cmd/ct/splunk

# Build the CLI.
go build ./cmd/ct

# Optional local binary through Makefile.
make build
```

The `Makefile` uses vendoring for build-related targets (`vendor` is generated from `go.mod`). Direct `go test`/`go build` commands are usually preferable during module-mode development. `make test` deliberately clears `GOFLAGS` and runs `go test -mod=mod ./...`.

`make build` may regenerate `eco/all_fen.tsv` if its dependencies are newer and builds a root-level `./ct` binary. Do not include generated/local artifacts in changes unless explicitly requested.

## External dependencies and test caveats

Some packages/tests require external services or binaries:

- `stockfish` on `PATH` is required for `ct eval`, engine-selected `repmk`, Stockfish scoring in `repvld`, and cache upgrade paths.
- `LICHESS_TOKEN` may be required for Lichess Explorer-backed tests and workflows.
- Lichess cloud evaluation, Explorer, game export, study export, and crosstable endpoints are live network dependencies and can fail due to network policy, API changes, missing token, or rate limiting.
- `cmd/ct/repmk` tests skip when Stockfish is missing; some flows also call Lichess Explorer.
- `cmd/ct/repvld` integration tests skip when Stockfish or `LICHESS_TOKEN` is missing.
- `cmd/ct/splunk` tests skip when `LICHESS_TOKEN` is missing.

When reporting test results, mention skipped or failed tests caused by missing `stockfish`, missing `LICHESS_TOKEN`, live network/API responses, proxy policy, or stale vendoring.

## CLI notes

- The only supported binary is `ct`.
- Top-level usage: `ct <command> [args]`.
- Help: `ct --help`, `ct help <command>`, or `ct <command> --help`.
- Current commands: `960gen`, `eval`, `fencat`, `pgn2fen`, `pgnfilt`, `pgnmk`, `repmk`, `repvld`, `splunk`, `upgrade`, and `version`.
- `ct eval` requires exactly one input mode unless `--upgrade` is used:
  - `--fen <FEN>`
  - `--fenfile <file|- for stdin>`
  - `--pgn <file-or-lichess-url> --move <n> --turn <white|black>`
- Commands that accept PGN inputs generally use `chesstools.OpenPgn`, so they may accept local files or Lichess game/study URLs.
- `ct pgn2fen` reads PGN from stdin when no PGN files are provided.
- `ct eval` maintains local cache files under the user's config directory by default, typically `~/.config/chesstools/cache` on Linux.
- `ct repmk` writes the requested output file after removing any existing file of the same name.
- `ct upgrade` contacts GitHub releases and, for Homebrew builds, runs Homebrew commands.

## ECO/opening data

- `openings.go` embeds `eco/all_fen.tsv` via `//go:embed`.
- `eco/build.sh` combines Lichess opening TSVs and uses the local `./ct pgn2fen` binary to produce `eco/all_fen.tsv`, then appends `eco/extra_fen.tsv`.
- Do not edit or regenerate this data unless the user specifically requests opening data changes.

## Repository hygiene for agents

- Before editing, inspect relevant files with the provided tools.
- Use patch-based edits for existing files unless replacing the entire file is appropriate.
- After editing Go code, run `gofmt` and targeted tests.
- For documentation-only changes, no Go formatting is needed; run tests only if the documentation change depends on behavior that should be verified.
- Keep generated/local artifacts out of git. Important ignored outputs include:
  - `/ct`
  - `/cache`
  - `/vendor`
  - root-level `/*.pgn`
  - `/exceptions.json`
  - `/score_all.sh`
  - `/unit-tests.xml`
  - `*.test`, `*.out`
- Do not commit secrets, API tokens, large PGN databases, engine binaries, generated caches, or vendored dependencies unless explicitly requested.
