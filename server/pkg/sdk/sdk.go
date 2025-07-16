package sdk

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// EnsureKeys checks for existing keys or generates new ones in the given directory.
func EnsureKeys(dir string) (privBase64, pubBase64 string, created bool, err error) {
	privPath := filepath.Join(dir, "server_private.key")
	pubPath := filepath.Join(dir, "server_public.key")

	// Check if keys exist
	privBytes, errPriv := ioutil.ReadFile(privPath)
	pubBytes, errPub := ioutil.ReadFile(pubPath)
	if errPriv == nil && errPub == nil {
		return strings.TrimSpace(string(privBytes)), strings.TrimSpace(string(pubBytes)), false, nil
	}

	// Generate new key pair using wgtypes
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", false, err
	}
	pub := priv.PublicKey()

	privBase64 = priv.String()
	pubBase64 = pub.String()

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
	// Print warning about key generation
	fmt.Println("[WARNING] New WireGuard server keys generated. These keys are sensitive and should be backed up securely. Make sure you save them and take a backup. Losing the private key will prevent you from restoring this server's identity.")
	return privBase64, pubBase64, true, nil
}

// FirstUsableIP returns the first usable IP in a subnet.
func FirstUsableIP(subnet *net.IPNet) net.IP {
	ip := subnet.IP.To4()
	ipInt := big.NewInt(0).SetBytes(ip)
	ipInt.Add(ipInt, big.NewInt(1)) // .1 is first usable
	return net.IP(ipInt.Bytes())
}

// MaskToDotted converts a net.IPMask to dotted decimal notation.
func MaskToDotted(mask net.IPMask) string {
	return net.IP(mask).String()
}

// MaskLength returns the number of leading ones in the mask.
func MaskLength(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
} 