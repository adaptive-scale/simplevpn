package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/conn"
)

func ensureKeys(dir string) (privBase64, pubBase64 string, created bool, err error) {
	privPath := filepath.Join(dir, "server_private.key")
	pubPath := filepath.Join(dir, "server_public.key")

	// Check if keys exist
	privBytes, errPriv := ioutil.ReadFile(privPath)
	pubBytes, errPub := ioutil.ReadFile(pubPath)
	if errPriv == nil && errPub == nil {
		return string(privBytes), string(pubBytes), false, nil
	}

	// Generate new key pair
	priv := make([]byte, 32)
	_, err = rand.Read(priv)
	if err != nil {
		return "", "", false, err
	}
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64

	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", "", false, err
	}

	privBase64 = base64.StdEncoding.EncodeToString(priv)
	pubBase64 = base64.StdEncoding.EncodeToString(pub)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", false, err
	}
	if err := ioutil.WriteFile(privPath, []byte(privBase64), 0600); err != nil {
		return "", "", false, err
	}
	if err := ioutil.WriteFile(pubPath, []byte(pubBase64), 0644); err != nil {
		return "", "", false, err
	}
	return privBase64, pubBase64, true, nil
}

func firstUsableIP(subnet *net.IPNet) net.IP {
	ip := subnet.IP.To4()
	ipInt := big.NewInt(0).SetBytes(ip)
	ipInt.Add(ipInt, big.NewInt(1)) // .1 is first usable
	return net.IP(ipInt.Bytes())
}

func maskToDotted(mask net.IPMask) string {
	return net.IP(mask).String()
}

func maskLength(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}

func main() {
	subnetFlag := flag.String("subnet", "10.0.0.0/24", "Tunnel subnet (CIDR)")
	flag.Parse()

	// Ensure $HOME/.simplevpn dir and keys
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get user home directory: %v", err)
	}
	keyDir := filepath.Join(homeDir, ".simplevpn")
	_, pubKey, created, err := ensureKeys(keyDir)
	if err != nil {
		log.Fatalf("failed to ensure server keys: %v", err)
	}
	if created {
		fmt.Printf("[SimpleVPN] Server public key: %s\n", pubKey)
	}

	_, subnet, err := net.ParseCIDR(*subnetFlag)
	if err != nil {
		log.Fatalf("invalid subnet: %v", err)
	}
	ipAddr := firstUsableIP(subnet).String()
	cidr := fmt.Sprintf("%s/%d", ipAddr, maskLength(subnet.Mask))

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

	// Assign internal IP address based on OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("ifconfig", ifName, "inet", ipAddr, ipAddr, "netmask", maskToDotted(subnet.Mask), "up")
	case "linux":
		cmd = exec.Command("ip", "addr", "add", cidr, "dev", ifName)
		upCmd := exec.Command("ip", "link", "set", "up", "dev", ifName)
		if err := upCmd.Run(); err != nil {
			log.Printf("Warning: failed to bring up interface: %v", err)
		}
	default:
		log.Printf("OS %s not supported for automatic IP assignment", runtime.GOOS)
	}
	if cmd != nil {
		if err := cmd.Run(); err != nil {
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

	log.Printf("WireGuard device %s is up and running (embedded mode). Configure it using wg or wgctrl-go.", ifName)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down WireGuard device...")
	dev.Close()
}