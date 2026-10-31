BINARY := auteur
PKG    := ./cmd/auteur
PREFIX ?= $(HOME)/.local

.PHONY: all build test fmt vet install clean

all: build

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

install:
	go install $(PKG)

clean:
	rm -f $(BINARY)
