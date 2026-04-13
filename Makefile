.PHONY: build run test clean

BINARY=mini-tmk-agent

build:
	go build -ldflags "-s -w" -o $(BINARY) ./cmd/mini-tmk-agent

run: build
	./$(BINARY)

test:
	go test ./...

test-integration:
	go test -tags integration ./...

clean:
	rm -f $(BINARY)
