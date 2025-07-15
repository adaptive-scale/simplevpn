package main

import (
	"bytes"
	"flag"
	"os/exec"
	"github.com/adaptive-scale/simplevpn/client"
)

// generateKey runs `wg genkey` and returns the key as a string.
func generateKey() (string, error) {
	out, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

// generatePublicKey runs `wg pubkey` with the private key.
func generatePublicKey(privateKey string) (string, error) {
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewBufferString(privateKey)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

func main() {
	embedded := flag.Bool("embedded", true, "Run as an embedded WireGuard client (in-process tunnel)")
	serverIP := flag.String("server-ip", "1.2.3.4", "WireGuard server IP")
	serverPort := flag.Int("server-port", 51820, "WireGuard server port")
	serverPubKey := flag.String("server-pubkey", "<server-public-key>", "WireGuard server public key (base64)")
	flag.Parse()

	if *embedded {
		client.RunEmbeddedClient(*serverIP, *serverPort, *serverPubKey)
	} else {
		client.RunConfigGenerator(*serverIP, *serverPort, *serverPubKey)
	}
} 