# SimpleVPN Architecture

SimpleVPN is a minimal VPN solution built in Go, leveraging WireGuard for secure tunneling. It consists of two main components: a server and a client, both of which use the WireGuard userspace tools for key management and interface configuration.

## Components

- **Server (`main.go`)**
  - Generates a WireGuard keypair on startup.
  - Creates and configures a WireGuard interface (`wg0`).
  - Listens for incoming VPN connections on UDP port 51820.
  - Provides instructions for adding client peers.

- **Client (`client.go`)**
  - Generates a WireGuard keypair for the client.
  - Outputs a WireGuard configuration file template for connecting to the server.
  - The user fills in the server's public key and endpoint, then brings up the interface using `wg-quick`.

## Data Flow

1. **Key Generation**
   - Both server and client generate their own private/public keypairs.
2. **Configuration**
   - The server sets up the `wg0` interface and listens for connections.
   - The client generates a config file referencing the server's public key and endpoint.
3. **Peer Exchange**
   - The server admin adds the client's public key and allowed IP to the server interface.
   - The client uses the generated config to connect to the server.
4. **Encrypted Tunnel**
   - All traffic between client and server is encrypted via WireGuard.

## Extensibility Points
- **Peer Management**: The server can be extended to provide an API or CLI for dynamic peer management.
- **Authentication**: Add authentication or access control for clients.
- **Monitoring**: Integrate logging or monitoring for active connections.

## Architecture Diagram

```
flowchart TD
    subgraph Server
        S1["WireGuard Interface (wg0)"]
        S2["Server Private/Public Key"]
    end
    subgraph Client
        C1["WireGuard Config"]
        C2["Client Private/Public Key"]
    end
    S1 <--> S2
    C1 <--> C2
    C1 -- Connects to --> S1
    S1 -- Allows --> C2
```

## Notes
- All network configuration is performed via shelling out to `wg` and `ip` tools.
- The architecture is intentionally simple for ease of understanding and extension. 