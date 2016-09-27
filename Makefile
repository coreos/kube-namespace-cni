.PHONY: all build

all: build

build:
	@go build -o kube-namespace

test:
	@go test -v .
