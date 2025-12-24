# WireGuard Mesh VPN

A cross-platform WireGuard-based mesh VPN system implemented in Go, providing automatic peer discovery, secure key exchange, and zero-configuration mesh networking.

## Features

- **Cross-Platform Support**: Windows, macOS, and Linux
- **Zero-Touch Setup**: Automatic key generation and peer discovery
- **Full Mesh Connectivity**: All peers can communicate directly
- **Stable Private IP Addressing**: Each peer gets a persistent IP address
- **Exit Node Support**: Route all traffic through designated exit nodes
- **Automatic Configuration**: No manual WireGuard configuration needed
- **Secure by Default**: Automatic key exchange using Curve25519
- **Heartbeat & Health Monitoring**: Automatic peer health tracking

## Architecture

The system consists of two main components:

### 1. Coordination Server
- Manages peer registration and discovery
- Allocates virtual IP addresses
- Tracks peer health through heartbeats
- Provides peer list for mesh connectivity
- Persists peer information

### 2. VPN Client
- Automatically registers with the coordination server
- Creates and configures WireGuard interface
- Maintains mesh connectivity with all peers
- Sends periodic heartbeats
- Syncs peer list and updates WireGuard configuration

## Architecture Diagram

```
┌─────────────┐         ┌─────────────────────┐         ┌─────────────┐
│   Client A  │◄───────►│ Coordination Server │◄───────►│   Client B  │
│ 10.100.0.1  │         │   (Peer Registry)   │         │ 10.100.0.2  │
└──────┬──────┘         └─────────────────────┘         └──────┬──────┘
       │                                                        │
       │            WireGuard Mesh Connection                   │
       └────────────────────────────────────────────────────────┘
```

## Prerequisites

### Linux
```bash
# Install WireGuard
sudo apt-get install wireguard  # Debian/Ubuntu
sudo yum install wireguard-tools  # CentOS/RHEL

# Ensure kernel module is loaded
sudo modprobe wireguard
```

### macOS
```bash
# Install WireGuard
brew install wireguard-tools wireguard-go
```

### Windows
- Download and install WireGuard from https://www.wireguard.com/install/
- Ensure the WireGuard service is running

## Installation

### From Source

#### Linux / macOS

```bash
# Clone the repository
git clone https://github.com/vpn/wireguard-mesh
cd wireguard-mesh

# Build binaries
make build

# Install (optional - Linux/macOS only)
sudo make install
```

#### Windows

```cmd
# Clone the repository
git clone https://github.com/vpn/wireguard-mesh
cd wireguard-mesh

# Build using the batch script
build.bat

# Or use PowerShell
powershell -ExecutionPolicy Bypass -File build.ps1

# Or build directly with Go
go build -o bin\vpn-server.exe .\cmd\server
go build -o bin\vpn-client.exe .\cmd\client
```

### Pre-built Binaries

Download the latest release for your platform from the releases page.

## Quick Start

### 1. Start the Coordination Server

**Linux / macOS:**
```bash
# Run with default settings
sudo ./bin/vpn-server

# Or specify custom settings
sudo ./bin/vpn-server -listen :8080 -network 10.100.0.0/16
```

**Windows (run as Administrator):**
```cmd
# Run with default settings
.\bin\vpn-server.exe

# Or specify custom settings
.\bin\vpn-server.exe -listen :8080 -network 10.100.0.0/16
```

The server will:
- Listen on port 8080 (HTTP)
- Create a VPN network with CIDR 10.100.0.0/16
- Generate server keys automatically
- Store configuration in `~/.config/wireguard-mesh/`

### 2. Connect Clients

On each client machine:

**Linux / macOS:**
```bash
# Run with default settings (requires root for interface creation)
sudo ./bin/vpn-client -server http://SERVER_IP:8080

# Run as exit node
sudo ./bin/vpn-client -server http://SERVER_IP:8080 -exit-node
```

**Windows (run as Administrator):**
```cmd
# Run with default settings
.\bin\vpn-client.exe -server http://SERVER_IP:8080

# Run as exit node
.\bin\vpn-client.exe -server http://SERVER_IP:8080 -exit-node
```

The client will:
- Generate keys automatically on first run
- Register with the coordination server
- Receive a virtual IP address
- Create a WireGuard interface (wg0)
- Establish mesh connections to all other peers
- Send periodic heartbeats

### 3. Verify Connectivity

```bash
# Check client status
./bin/vpn-client -status

# Ping another peer
ping 10.100.0.2

# Check WireGuard interface
sudo wg show wg0
```

## Configuration

### Server Configuration

Default location: `~/.config/wireguard-mesh/server.json`

```json
{
  "listen_addr": ":8080",
  "network_cidr": "10.100.0.0/16",
  "private_key": "...",
  "public_key": "...",
  "db_path": "/path/to/peers.json"
}
```

### Client Configuration

Default location: `~/.config/wireguard-mesh/client.json`

```json
{
  "server_addr": "http://SERVER_IP:8080",
  "interface_name": "wg0",
  "private_key": "...",
  "public_key": "...",
  "peer_id": "peer-123456",
  "assigned_ip": "10.100.0.1",
  "exit_node": false,
  "listen_port": 51820
}
```

## Usage Examples

### Basic Mesh Network

Connect three devices in a mesh:

```bash
# Server
sudo ./bin/vpn-server

# Client A
sudo ./bin/vpn-client -server http://SERVER:8080

# Client B
sudo ./bin/vpn-client -server http://SERVER:8080

# Client C
sudo ./bin/vpn-client -server http://SERVER:8080
```

Now all clients can communicate:
- Client A (10.100.0.1) ↔ Client B (10.100.0.2)
- Client A (10.100.0.1) ↔ Client C (10.100.0.3)
- Client B (10.100.0.2) ↔ Client C (10.100.0.3)

### Exit Node Setup

Route all traffic through a designated exit node:

```bash
# Exit node (e.g., cloud server with public IP)
sudo ./bin/vpn-client -server http://SERVER:8080 -exit-node

# Regular clients will automatically route traffic through the exit node
```

### Check Status

```bash
# View client status
./bin/vpn-client -status

# Output:
# {
#   "peer_id": "peer-123456",
#   "assigned_ip": "10.100.0.1",
#   "network": "10.100.0.0/16",
#   "public_key": "...",
#   "interface": {
#     "name": "wg0",
#     "num_peers": 2,
#     "peers": [...]
#   }
# }
```

## How It Works

### Registration Flow

1. Client generates WireGuard key pair (if not exists)
2. Client sends registration request to server with public key
3. Server allocates a virtual IP address
4. Server stores peer information
5. Server responds with assigned IP and network info
6. Client creates WireGuard interface with assigned IP

### Mesh Establishment

1. Client requests peer list from server
2. Server returns all registered peers (excluding requester)
3. Client adds each peer to WireGuard configuration
4. WireGuard establishes encrypted tunnels to all peers
5. Peers can now communicate directly over the mesh

### Health Monitoring

1. Clients send heartbeats every 30 seconds
2. Server updates last-seen timestamp
3. Server marks peers offline after 2 minutes of no heartbeat
4. Clients sync peer list every 60 seconds
5. Offline peers are removed from active mesh

## Project Structure

```
wireguard-mesh/
├── cmd/
│   ├── server/          # Server executable
│   │   └── main.go
│   └── client/          # Client executable
│       └── main.go
├── pkg/
│   ├── protocol/        # Protocol definitions and messages
│   │   └── messages.go
│   ├── crypto/          # Key generation and crypto operations
│   │   └── keys.go
│   ├── network/         # IP allocation and network utilities
│   │   └── ipam.go
│   ├── wireguard/       # WireGuard interface management
│   │   └── interface.go
│   ├── config/          # Configuration management
│   │   └── config.go
│   ├── server/          # Server implementation
│   │   ├── server.go
│   │   └── store.go
│   └── client/          # Client implementation
│       └── client.go
├── Makefile
├── go.mod
└── README.md
```

## API Reference

### Server Endpoints

#### POST /register
Register a new peer or update existing peer.

**Request:**
```json
{
  "public_key": "base64-encoded-key",
  "hostname": "my-laptop",
  "os": "linux",
  "endpoint": "1.2.3.4:51820",
  "request_ip": true,
  "exit_node": false
}
```

**Response:**
```json
{
  "success": true,
  "assigned_ip": "10.100.0.1",
  "network_cidr": "10.100.0.0/16",
  "peer_id": "peer-123456",
  "server_public_key": "base64-encoded-key"
}
```

#### POST /heartbeat
Send heartbeat to maintain peer status.

**Request:**
```json
{
  "peer_id": "peer-123456",
  "endpoint": "1.2.3.4:51820"
}
```

**Response:**
```json
{
  "success": true
}
```

#### GET /peers
Get list of all peers.

**Query Parameters:**
- `peer_id`: Requesting peer's ID

**Response:**
```json
{
  "peers": [
    {
      "id": "peer-789",
      "public_key": "base64-encoded-key",
      "virtual_ip": "10.100.0.2",
      "endpoint": "1.2.3.4:51820",
      "hostname": "server-1",
      "os": "linux",
      "allowed_ips": ["10.100.0.2/32"],
      "exit_node": false,
      "online": true,
      "last_heartbeat": "2024-01-01T12:00:00Z"
    }
  ]
}
```

## Security Considerations

- All WireGuard traffic is encrypted using ChaCha20-Poly1305
- Key exchange happens over the coordination server (consider using TLS)
- Private keys never leave the client device
- Server only knows public keys and metadata
- Implement TLS for the coordination server in production
- Consider implementing authentication for peer registration
- Use firewall rules to restrict coordination server access

## Troubleshooting

### Client Can't Register

```bash
# Check server is running
curl http://SERVER:8080/peers?peer_id=test

# Check network connectivity
ping SERVER_IP

# Check server logs
```

### WireGuard Interface Creation Fails

```bash
# Ensure you have root privileges
sudo ./bin/vpn-client

# Check WireGuard is installed
which wg

# On Linux, check kernel module
lsmod | grep wireguard
```

### Peers Can't Communicate

```bash
# Check WireGuard status
sudo wg show wg0

# Verify peer configuration
sudo wg show wg0 allowed-ips

# Check if peers are online
./bin/vpn-client -status

# Test basic connectivity
ping -I wg0 PEER_IP
```

### Firewall Issues

Ensure UDP port 51820 (or your configured port) is open:

```bash
# Linux (iptables)
sudo iptables -A INPUT -p udp --dport 51820 -j ACCEPT

# Linux (firewalld)
sudo firewall-cmd --add-port=51820/udp --permanent
sudo firewall-cmd --reload
```

## Performance Tuning

### Server Optimization

- Increase heartbeat timeout for networks with high latency
- Adjust cleanup interval based on peer count
- Use a proper database instead of JSON for large deployments

### Client Optimization

- Adjust peer sync interval based on mesh stability
- Configure WireGuard MTU for your network
- Use persistent keepalive for NAT traversal

## Contributing

Contributions are welcome! Please submit pull requests or open issues for bugs and feature requests.

## License

MIT License - see LICENSE file for details.

## Acknowledgments

- Built on top of [WireGuard](https://www.wireguard.com/)
- Uses [wgctrl-go](https://github.com/WireGuard/wgctrl-go) for WireGuard management
- Inspired by [Tailscale](https://tailscale.com/)

## Roadmap

- [ ] Add TLS support for coordination server
- [ ] Implement authentication and authorization
- [ ] Add web dashboard for server
- [ ] Support for IPv6
- [ ] NAT traversal improvements
- [ ] Mobile client support (iOS/Android)
- [ ] Relay servers for difficult NAT scenarios
- [ ] DNS integration
- [ ] ACL/firewall rules
- [ ] Metrics and monitoring
