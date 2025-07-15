package sdk

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"bufio"
	"strings"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	)

func generateKey() (string, error) {
	out, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

func generatePublicKey(privateKey string) (string, error) {
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewBufferString(privateKey)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

// Remove all file persistence for client private key. Always generate a new key in memory.
func generatePeerConfig(serverIP string, serverPort int, serverPubKey string) (string, string, error) {
	// Always generate a new client private key in memory
	clientPriv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}

	fmt.Println("Client Private Key:", clientPriv)

	// Convert base64 public key to hex for IPC
	pubKeyBytes, err := base64.StdEncoding.DecodeString(serverPubKey)
	if err != nil {
		return "", "", fmt.Errorf("Invalid base64 server public key: %v", err)
	}

	fmt.Println("Server Public Key:", serverPubKey)

	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	config := fmt.Sprintf(
		"public_key=%s endpoint=%s:%d allowed_ip=0.0.0.0/0 persistent_keepalive_interval=25",
		pubKeyHex, serverIP, serverPort,
	)

	fmt.Println("Config:", config)
	
	return config, clientPriv.String(), nil
}

func runConfigGenerator(serverIP string, serverPort int, serverPubKey string, clientIP string) {
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

	serverEndpoint := fmt.Sprintf("%s:%d", serverIP, serverPort)
	clientCIDR := fmt.Sprintf("%s/32", clientIP)

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
`, privKey, clientCIDR, serverPubKey, serverEndpoint)

	fmt.Println("\nTo use this config, save it as client.conf and run:")
	fmt.Println("  sudo wg-quick up ./client.conf")
}

// runEmbeddedClient should use only Go native code to set up the WireGuard device and peer
func runEmbeddedClient(serverIP string, serverPort int, serverPubKey string, clientIP string) {
	tunName := "utun"
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

	ipAddr := clientIP
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

	// Set default gateway to the WireGuard server's IP
	if runtime.GOOS == "linux" {
		err := exec.Command("ip", "route", "replace", "default", "via", serverIP, "dev", ifName).Run()
		if err != nil {
			log.Printf("Failed to set default gateway on Linux: %v", err)
		} else {
			log.Printf("Set default gateway to %s on %s (Linux)", serverIP, ifName)
		}
	} else if runtime.GOOS == "darwin" {
		err := exec.Command("route", "change", "default", serverIP).Run()
		if err != nil {
			log.Printf("Failed to set default gateway on macOS: %v", err)
		} else {
			log.Printf("Set default gateway to %s (macOS)", serverIP)
		}
	}

	// Add routes to send all traffic via the tunnel, without replacing the default route
	switch runtime.GOOS {
	case "linux":
		cmd1 := exec.Command("ip", "route", "add", "0.0.0.0/1", "dev", ifName)
		cmd2 := exec.Command("ip", "route", "add", "128.0.0.0/1", "dev", ifName)
		if err := cmd1.Run(); err != nil {
			log.Printf("Warning: failed to add 0.0.0.0/1 route: %v", err)
		} else {
			log.Printf("Added 0.0.0.0/1 route via %s", ifName)
		}
		if err := cmd2.Run(); err != nil {
			log.Printf("Warning: failed to add 128.0.0.0/1 route: %v", err)
		} else {
			log.Printf("Added 128.0.0.0/1 route via %s", ifName)
		}
	case "darwin":
		cmd1 := exec.Command("route", "-n", "add", "-net", "0.0.0.0/1", ipAddr)
		cmd2 := exec.Command("route", "-n", "add", "-net", "128.0.0.0/1", ipAddr)
		if err := cmd1.Run(); err != nil {
			log.Printf("Warning: failed to add 0.0.0.0/1 route: %v", err)
		} else {
			log.Printf("Added 0.0.0.0/1 route via %s", ipAddr)
		}
		if err := cmd2.Run(); err != nil {
			log.Printf("Warning: failed to add 128.0.0.0/1 route: %v", err)
		} else {
			log.Printf("Added 128.0.0.0/1 route via %s", ipAddr)
		}
	}

	// After assigning IP, add a route for the VPN subnet via the system gateway
	vpnSubnet := "10.0.0.0/24"
	if runtime.GOOS == "linux" {
		// Get default gateway
		out, err := exec.Command("ip", "route", "show", "default").Output()
		if err == nil {
			fields := strings.Fields(string(out))
			gw := ""
			for i, f := range fields {
				if f == "via" && i+1 < len(fields) {
					gw = fields[i+1]
					break
				}
			}
			if gw != "" {
				log.Printf("Adding route for %s via gateway %s on %s", vpnSubnet, gw, ifName)
				err := exec.Command("ip", "route", "add", vpnSubnet, "via", gw, "dev", ifName).Run()
				if err != nil {
					log.Printf("Failed to add route via gateway: %v", err)
				}
			}
		}
	} else if runtime.GOOS == "darwin" {
		// Get default gateway
		out, err := exec.Command("route", "-n", "get", "default").Output()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			gw := ""
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "gateway:") {
					parts := strings.Fields(line)
					if len(parts) == 2 {
						gw = parts[1]
						break
					}
				}
			}
			if gw != "" {
				log.Printf("Adding route for %s via gateway %s", vpnSubnet, gw)
				err := exec.Command("route", "add", "-net", vpnSubnet, gw).Run()
				if err != nil {
					log.Printf("Failed to add route via gateway: %v", err)
				}
			}
		}
	}

	logger := device.NewLogger(device.LogLevelVerbose, "WGD: ")
	bind := conn.NewDefaultBind()
	dev := device.NewDevice(tunDev, bind, logger)

	fmt.Printf("Device type: %T\n", dev)
	
	if serverPubKey == "" || serverPubKey == "<server-public-key>" {
		log.Printf("[WARNING] No valid server public key provided. Skipping peer configuration.")
	} else {

	go dev.Up()

    clientPriv, err := wgtypes.GeneratePrivateKey()
    if err != nil {
        log.Fatalf("Failed to generate key: %v", err)
    }

    log.Println("Client Private Key:", hex.EncodeToString(clientPriv[:])) // hex

    // UAPI config expects base64
    err = dev.IpcSet(fmt.Sprintf("private_key=%s", 
	 hex.EncodeToString(clientPriv[:])))
    if err != nil {
        log.Fatalf("Failed to set private key: %v", err)
    }

    log.Println("Private key set successfully.")

		// Convert server public key from base64 to hex for IPC
		pubKeyBytes, err := base64.StdEncoding.DecodeString(serverPubKey)
		if err != nil {
			log.Fatalf("Invalid base64 server public key: %v", err)
		}
		pubKeyHex := hex.EncodeToString(pubKeyBytes)
		log.Println("Server Public Key (hex):", pubKeyHex)

		// Prepare peer config string
		peerConfig := fmt.Sprintf(
			"public_key=%s\nendpoint=%s:%d\n",
			pubKeyHex, serverIP, serverPort,
		)
		err = dev.IpcSet(peerConfig)
		if err != nil {
			log.Fatalf("Failed to apply config: %v", err)
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

// Exported wrappers
func RunConfigGenerator(serverIP string, serverPort int, serverPubKey string, clientIP string) {
	runConfigGenerator(serverIP, serverPort, serverPubKey, clientIP)
}

func RunEmbeddedClient(serverIP string, serverPort int, serverPubKey string, clientIP string) {
	runEmbeddedClient(serverIP, serverPort, serverPubKey, clientIP)
} 