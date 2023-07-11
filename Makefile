export GO111MODULE=on
export GOFLAGS=-mod=vendor

.PHONY: all
all: build test

.PHONY: build
build: eco/all_fen.tsv pgn2fen
	go build github.com/mikeb26/chesstools/cmd/repvld
	go build github.com/mikeb26/chesstools/cmd/repbld
	go build github.com/mikeb26/chesstools/cmd/cteval
	go build github.com/mikeb26/chesstools/cmd/pgnfilt
	go build github.com/mikeb26/chesstools/cmd/pgnmk

eco/all_fen.tsv: eco/a.tsv eco/b.tsv eco/c.tsv eco/d.tsv eco/e.tsv eco/extra_fen.tsv pgn2fen
	cd eco; ./build.sh

pgn2fen:
	go build github.com/mikeb26/chesstools/cmd/pgn2fen

.PHONY: test
test:
	go test github.com/mikeb26/chesstools/cmd/repvld github.com/mikeb26/chesstools/cmd/repbld github.com/mikeb26/chesstools/cmd/cteval github.com/mikeb26/chesstools/cmd/pgnfilt github.com/mikeb26/chesstools/cmd/pgnmk github.com/mikeb26/chesstools/cmd/pgn2fen

.PHONY: deps
deps:
	rm -rf go.mod go.sum vendor
	go mod init github.com/mikeb26/chesstools
	go mod edit -replace=github.com/notnil/chess=github.com/mikeb26/chess@v1.9.0.mb2
	GOPROXY=direct go mod tidy
	go mod vendor
	mkdir /tmp/openings
	cd /tmp/openings; git clone https://github.com/lichess-org/chess-openings.git
	cp /tmp/openings/chess-openings/*.tsv eco/
	rm -rf /tmp/openings

vendor: go.mod
	go mod download
	go mod vendor

.PHONY: clean
clean:
	rm -f repvld repbld cteval pgnfilt pgnmk pgn2fen eco/all_fen.tsv

FORCE:
