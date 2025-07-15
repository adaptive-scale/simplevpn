# SimpleVPN Makefile

.PHONY: all build server client clean help

BINDIR := bin

all: build

build: server client

server:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/simplevpn-server main.go

client:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/simplevpn-client client.go

clean:
	rm -rf $(BINDIR)

help:
	@echo "SimpleVPN Makefile targets:"
	@echo "  build   - Build both server and client binaries."
	@echo "  server  - Build only the server binary."
	@echo "  client  - Build only the client binary."
	@echo "  clean   - Remove built binaries."
	@echo "  help    - Show this help message." 