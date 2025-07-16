# SimpleVPN Makefile

.PHONY: all build server client clean lint test format help

BINDIR := bin

all: build

build: server client

server:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/simplevpn-server server/cmd/main.go

client:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/simplevpn-client client/cmd/main.go

lint:
	go vet ./...

test:
	go test ./...

format:
	gofmt -s -w .

clean:
	rm -rf $(BINDIR)

help:
	@echo "SimpleVPN Makefile targets:"
	@echo "  build   - Build both server and client binaries."
	@echo "  server  - Build only the server binary."
	@echo "  client  - Build only the client binary."
	@echo "  lint    - Run go vet on all packages."
	@echo "  test    - Run all Go tests."
	@echo "  format  - Run gofmt on all Go files."
	@echo "  clean   - Remove built binaries."
	@echo "  help    - Show this help message." 