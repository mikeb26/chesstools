export GO111MODULE=on
export GOFLAGS=-mod=vendor

.PHONY: all
all: build

.PHONY: build
build: eco/all_fen.tsv ct
	go build ./cmd/ct

eco/all_fen.tsv: eco/a.tsv eco/b.tsv eco/c.tsv eco/d.tsv eco/e.tsv eco/extra_fen.tsv ct
	cd eco; ./build.sh

ct:
	go build ./cmd/ct

.PHONY: test
test:
	GOFLAGS= go test -mod=mod ./cmd/ct/...

.PHONY: deps
deps:
	rm -rf go.mod go.sum vendor
	go mod init github.com/mikeb26/chesstools
	#go mod edit -replace=github.com/corentings/chess/v2=github.com/mikeb26/corentings-chess/v2@v2.1.0.mb4
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
	rm -f ct eco/all_fen.tsv

FORCE:
