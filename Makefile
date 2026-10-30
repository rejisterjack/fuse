.PHONY: test vet build run

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/fuse ./cmd/fuse

run:
	go run ./cmd/fuse -config config.example.yaml
