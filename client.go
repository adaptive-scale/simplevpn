package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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

func runConfigGenerator(serverIP string, serverPort int, serverPubKey string) {
	// Generate client private and public keys
	privKey, err := generateKey()
	if err != nil {
		panic(err)
	}
	pubKey, err := generatePublicKey(privKey)
	if err != nil {
		panic(err)
	}

	fmt.Println("Client Private Key:", privKey)
	fmt.Println("Client Public Key:", pubKey)

	// Replace these with your server's public key and endpoint
	serverEndpoint := fmt.Sprintf("%s:%d", serverIP, serverPort)

	clientIP := "10.0.0.2/32" // Assign a unique IP for each client

	fmt.Println("\n--- WireGuard Client Config ---")
	fmt.Printf(`[Interface]
PrivateKey = %s
Address = %s
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`, privKey, clientIP, serverPubKey, serverEndpoint)

	fmt.Println("\nTo use this config, save it as client.conf and run:")
	fmt.Println("  sudo wg-quick up ./client.conf")
}

func runEmbeddedClient(serverIP string, serverPort int, serverPubKey string) {
	tunName := "utun" // Use 'utun' for macOS compatibility
	tunDev, err := tun.CreateTUN(tunName, 1420)
	if err != nil {
		log.Fatalf("failed to create TUN device: %v", err)
	}

	ifName, err := tunDev.Name()
	if err != nil {
		log.Printf("Warning: could not get interface name: %v", err)
	} else {
		log.Printf("Created TUN device: %s", ifName)
	}

	// Assign a static IP for demo (update as needed)
	ipAddr := "10.0.0.2"
	mask := "255.255.255.0"
	var assignCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		assignCmd = exec.Command("ifconfig", ifName, "inet", ipAddr, ipAddr, "netmask", mask, "up")
	case "linux":
		assignCmd = exec.Command("ip", "addr", "add", fmt.Sprintf("%s/24", ipAddr), "dev", ifName)
	}
	if assignCmd != nil {
		if err := assignCmd.Run(); err != nil {
			log.Fatalf("failed to assign IP address: %v", err)
		}
		log.Printf("Assigned IP %s to interface %s", ipAddr, ifName)
	}

	logger := device.NewLogger(device.LogLevelVerbose, "WGD: ")
	bind := conn.NewDefaultBind()
	dev := device.NewDevice(tunDev, bind, logger)

	err = dev.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	// Configure the peer (server) using wgctrl-go
	if serverPubKey == "" || serverPubKey == "<server-public-key>" {
		log.Printf("[WARNING] No valid server public key provided. Skipping peer configuration.")
	} else {
		client, err := wgctrl.New()
		if err != nil {
			log.Fatalf("failed to open wgctrl client: %v", err)
		}
		defer client.Close()

		pubKey, err := wgtypes.ParseKey(serverPubKey)
		if err != nil {
			log.Fatalf("invalid server public key: %v", err)
		}

		peerConfig := wgtypes.PeerConfig{
			PublicKey: pubKey,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(serverIP),
				Port: serverPort,
			},
			AllowedIPs: []net.IPNet{{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}},
			PersistentKeepaliveInterval: func() *time.Duration { d := 25 * time.Second; return &d }(),
		}

		cfg := wgtypes.Config{
			Peers: []wgtypes.PeerConfig{peerConfig},
		}

		if err := client.ConfigureDevice(ifName, cfg); err != nil {
			log.Fatalf("failed to configure peer: %v", err)
		}
		log.Printf("Peer (server) configured: %s:%d", serverIP, serverPort)
	}

	log.Printf("WireGuard client device %s is up and running (embedded mode).", ifName)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down WireGuard client device...")
	dev.Close()
}

func main() {
	embedded := flag.Bool("embedded", true, "Run as an embedded WireGuard client (in-process tunnel)")
	serverIP := flag.String("server-ip", "1.2.3.4", "WireGuard server IP")
	serverPort := flag.Int("server-port", 51820, "WireGuard server port")
	serverPubKey := flag.String("server-pubkey", "<server-public-key>", "WireGuard server public key (base64)")
	flag.Parse()

	if *embedded {
		runEmbeddedClient(*serverIP, *serverPort, *serverPubKey)
	} else {
		runConfigGenerator(*serverIP, *serverPort, *serverPubKey)
	}
} 