# SimpleVPN

A minimal VPN server and client built in Go, using WireGuard for secure networking.

## Features
- Simple WireGuard-based VPN server and client
- Automatic key generation
- Easy configuration and usage

## Requirements
- Go 1.18+
- WireGuard tools (`wg`, `ip`, and `wg-quick`) installed and available in your PATH
- Root privileges to create network interfaces
- Linux/BSD for server operation (macOS for config generation only)

## Build

```sh
make build
```

Binaries will be placed in the `bin/` directory:
- `bin/simplevpn-server`
- `bin/simplevpn-client`

## Usage

### Server

1. Run the server (as root):
   ```sh
   sudo ./bin/simplevpn-server
   ```
2. The server will generate a keypair, create a `wg0` interface, and print its public key.
3. To add a client peer, use:
   ```sh
   wg set wg0 peer <client-public-key> allowed-ips <client-ip>/32
   ```

### Client

1. Run the client generator:
   ```sh
   ./bin/simplevpn-client
   ```
2. The client will generate a keypair and print a WireGuard config with placeholders for the server's public key and endpoint.
3. Fill in the server's public key and endpoint in the config.
4. Save the config as `client.conf` and bring up the interface:
   ```sh
   sudo wg-quick up ./client.conf
   ```

## Example Client Config

```
[Interface]
PrivateKey = <client-private-key>
Address = 10.0.0.2/32
DNS = 1.1.1.1

[Peer]
PublicKey = <server-public-key>
Endpoint = <server-endpoint>:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
```

## Notes
- Assign a unique IP (e.g., 10.0.0.2/32, 10.0.0.3/32, etc.) to each client.
- The server must be run as root to manage network interfaces.
- For production, consider adding authentication, peer management, and security hardening.

## License
MIT 