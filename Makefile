.PHONY: all
all: build test

.PHONY: build
build:
	go build github.com/mikeb26/chesstools/cmd/chessrep

.PHONY: test
test:
	go test github.com/mikeb26/chesstools/cmd/chessrep

.PHONY: vendor
vendor:
	rm -rf vendor go.sum
	go mod vendor

.PHONY: clean
clean:
	rm -f chessrep
