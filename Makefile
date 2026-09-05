.PHONY: fmt vet test race build vuln ci

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

build:
	go build -trimpath -o bin/atlasmesh ./cmd/atlasmesh

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

ci:
	test -z "$$(gofmt -l ./cmd ./internal)"
	go vet ./...
	go test -race ./...
	go build ./cmd/atlasmesh
