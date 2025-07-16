package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/conn"

	sdk "github.com/adaptive-scale/simplevpn/server/pkg/sdk"
)

func enableForwardingAndNAT() (restore func(), err error) {
	if runtime.GOOS == "linux" {
		// Save original IP forwarding state
		out, err := exec.Command("sysctl", "-n", "net.ipv4.ip_forward").Output()
		if err != nil {
			return nil, err
		}
		origForward := strings.TrimSpace(string(out))

		// Enable forwarding
		if err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
			return nil, err
		}
		log.Println("Enabled IP forwarding (Linux)")

		// Add NAT rule
		if err := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.0.0.0/24", "-o", "eth0", "-j", "MASQUERADE").Run(); err != nil {
			return nil, err
		}
		log.Println("Added NAT rule (Linux)")

		return func() {
			exec.Command("sysctl", "-w", "net.ipv4.ip_forward="+origForward).Run()
			exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", "10.0.0.0/24", "-o", "eth0", "-j", "MASQUERADE").Run()
			log.Println("Restored IP forwarding and removed NAT rule (Linux)")
		}, nil
	} else if runtime.GOOS == "darwin" {
		// Save original IP forwarding state
		out, err := exec.Command("sysctl", "-n", "net.inet.ip.forwarding").Output()
		if err != nil {
			return nil, err
		}
		origForward := strings.TrimSpace(string(out))

		// Enable forwarding
		if err := exec.Command("sysctl", "-w", "net.inet.ip.forwarding=1").Run(); err != nil {
			return nil, err
		}
		log.Println("Enabled IP forwarding (macOS)")

		pfRule := "nat on en0 from 10.0.0.0/24 to any -> (en0)"
		pfConf := "/etc/pf.conf"
		backup := pfConf + ".bak"

		// Backup pf.conf
		exec.Command("cp", pfConf, backup).Run()

		// Check if rule already exists
		data, _ := os.ReadFile(pfConf)
		if !strings.Contains(string(data), pfRule) {
			f, err := os.OpenFile(pfConf, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString("\n" + pfRule + "\n")
				f.Close()
				log.Println("Appended NAT rule to pf.conf (macOS)")
			} else {
				log.Printf("Failed to append NAT rule to pf.conf: %v", err)
			}
		} else {
			log.Println("NAT rule already present in pf.conf (macOS)")
		}

		// Reload pf
		exec.Command("pfctl", "-f", pfConf).Run()
		exec.Command("pfctl", "-e").Run()
		log.Println("Reloaded and enabled pf (macOS)")

		return func() {
			exec.Command("sysctl", "-w", "net.inet.ip.forwarding="+origForward).Run()
			exec.Command("mv", backup, pfConf).Run()
			exec.Command("pfctl", "-f", pfConf).Run()
			log.Println("Restored IP forwarding and pf.conf (macOS)")
		}, nil
	}
	return func() {}, nil // no-op for other OS
}

func main() {
	subnetFlag := flag.String("subnet", "10.0.0.0/24", "Tunnel subnet (CIDR)")
	listenPort := flag.Int("listen-port", 51820, "WireGuard server listen port")
	allowedIPs := flag.String("allowed-ips", "0.0.0.0/0", "Allowed IPs for the peer (comma-separated)")
	flag.Parse()

	// Ensure $HOME/.simplevpn dir and keys
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get user home directory: %v", err)
	}
	keyDir := filepath.Join(homeDir, ".simplevpn")
	_, pubKey, created, err := sdk.EnsureKeys(keyDir)
	if err != nil {
		log.Fatalf("failed to ensure server keys: %v", err)
	}
	if created {
		fmt.Printf("[SimpleVPN] Server public key: %s\n", pubKey)
	} else {
		fmt.Printf("[SimpleVPN] Server public key: %s\n", pubKey)
	}

	_, subnet, err := net.ParseCIDR(*subnetFlag)
	if err != nil {
		log.Fatalf("invalid subnet: %v", err)
	}
	ipAddr := sdk.FirstUsableIP(subnet).String()
	cidr := fmt.Sprintf("%s/%d", ipAddr, sdk.MaskLength(subnet.Mask))

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
		cmd = exec.Command("ifconfig", ifName, "inet", ipAddr, ipAddr, "netmask", sdk.MaskToDotted(subnet.Mask), "up")
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

	dest := "10.0.0.1"
	gw := "192.168.2.254"
	
	if runtime.GOOS == "linux" {
		err := exec.Command("ip", "route", "add", dest, "via", gw, "dev", ifName).Run()
		if err != nil {
			log.Printf("Failed to add static route on Linux: %v", err)
		} else {
			log.Printf("Added static route: %s via %s dev %s", dest, gw, ifName)
		}
	} else if runtime.GOOS == "darwin" {
		err := exec.Command("route", "add", "-host", dest, gw).Run()
		if err != nil {
			log.Printf("Failed to add static route on macOS: %v", err)
		} else {
			log.Printf("Added static route: %s via %s", dest, gw)
		}
	}

	logger := device.NewLogger(device.LogLevelVerbose, "WGD: ")
	// Bind to all interfaces (0.0.0.0) for listening
	bind := conn.NewDefaultBind()
	dev := device.NewDevice(tunDev, bind, logger)

	// Set the listen port for the WireGuard device
	if err := dev.IpcSet(fmt.Sprintf("listen_port=%d\n", *listenPort)); err != nil {
		log.Fatalf("failed to set listen port: %v", err)
	}

	err = dev.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	log.Printf("WireGuard device %s is up and running (embedded mode) on port %d. Configure it with simplevpn client.", ifName, *listenPort)
	log.Printf("Allowed IPs for peer: %s", *allowedIPs)

	restore, err := enableForwardingAndNAT()
	if err != nil {
		log.Fatalf("Failed to enable forwarding/NAT: %v", err)
	}
	defer restore()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down WireGuard device...")
	dev.Close()
}