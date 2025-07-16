package main

import (
	"flag"
	"fmt"
	"os"

	client "github.com/adaptive-scale/simplevpn/client/pkg/sdk"
)

func main() {
	mode := flag.String("mode", "embedded", "Mode: 'config' to generate config, 'embedded' to run embedded client")
	serverIP := flag.String("server-ip", "127.0.0.1", "Server IP address")
	serverPort := flag.Int("server-port", 51820, "Server WireGuard port")
	serverPubKey := flag.String("server-pubkey", "", "Server public key (base64)")
	clientIP := flag.String("client-ip", "10.0.0.2", "Client tunnel IP address (without mask)")
	flag.Parse()

	switch *mode {
	case "config":
		client.RunConfigGenerator(*serverIP, *serverPort, *serverPubKey, *clientIP)
	case "embedded":
		client.RunEmbeddedClient(*serverIP, *serverPort, *serverPubKey, *clientIP)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
} 