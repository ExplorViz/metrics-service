.PHONY: all generate build run

all: generate build

generate:
	go generate

build: generate
	go build

run: generate
	@go run . || true