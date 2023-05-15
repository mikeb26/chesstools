.PHONY: all
all: build test

.PHONY: build
build:
	go build github.com/mikeb26/chesstools/cmd/chessrep
	go build github.com/mikeb26/chesstools/cmd/eval
	go build github.com/mikeb26/chesstools/cmd/pgnfilt
	go build github.com/mikeb26/chesstools/cmd/pgn2fen

.PHONY: test
test:
	go test github.com/mikeb26/chesstools/cmd/chessrep github.com/mikeb26/chesstools/cmd/eval github.com/mikeb26/chesstools/cmd/pgnfilt github.com/mikeb26/chesstools/cmd/pgn2fen

.PHONY: vendor
vendor:
	rm -rf vendor go.sum
	go mod vendor

.PHONY: clean
clean:
	rm -f chessrep eval pgnfilt pgn2fen
