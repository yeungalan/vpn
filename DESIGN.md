# High-Level Design (HLD): Cross-Platform WireGuard-Based Mesh VPN in Go

## 1. Overview

This document describes the high-level design of a cross-platform virtual private network (VPN) built on top of WireGuard, targeting Windows, macOS, and Linux. The system provides automatic peer discovery, secure key exchange, mesh connectivity, and exit-node based traffic routing, similar in behavior to Tailscale, while being fully implemented in Golang.

Each device joining the network is automatically assigned a stable private IP address and can securely communicate with other devices in the same virtual network without manual configuration.

## 2. Goals & Non-Goals

### 2.1 Goals

- **Cross-platform support**: Windows, macOS, Linux
- **Zero-touch setup**:
  - Automatic key generation
  - Automatic peer discovery and handshake
  - Full mesh connectivity between nodes
  - Stable private IP address per node
- **Support exit nodes** with policy-based routing
- **Minimal user configuration**
- **Implemented entirely in Golang**

### 2.2 Non-Goals

- Mobile support (iOS/Android) in initial version
- NAT traversal via STUN/TURN (may be added later)
- Built-in DNS server (can be added as extension)
- User authentication (initial version uses implicit trust)
- Web UI/dashboard (command-line first)

## 3. Architecture Overview

The system follows a client-server architecture with a centralized coordination server and distributed clients forming a mesh network.

```
┌─────────────────────────────────────────────────────────────────┐
│                     Coordination Server                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Peer Registry│  │ IP Allocator │  │ Health Check │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└────────────┬────────────────────────────────────┬───────────────┘
             │ Registration/Discovery              │ Heartbeat
             │                                     │
    ┌────────┴────────┬───────────────────────────┴────────┐
    │                 │                                     │
┌───▼────┐       ┌────▼───┐                           ┌────▼───┐
│Client A│◄─────►│Client B│◄─────────────────────────►│Client C│
│10.x.x.1│       │10.x.x.2│    WireGuard Mesh         │10.x.x.3│
└────────┘       └────────┘                           └────────┘
```

### 3.1 Components

#### 3.1.1 Coordination Server
- **Purpose**: Central control plane for peer coordination
- **Responsibilities**:
  - Peer registration and deregistration
  - Virtual IP address allocation (IPAM)
  - Peer discovery and listing
  - Health monitoring via heartbeats
  - Persistent peer storage
- **Does NOT**: Route data traffic (data plane is fully distributed)

#### 3.1.2 VPN Client
- **Purpose**: Agent running on each peer device
- **Responsibilities**:
  - WireGuard interface creation and configuration
  - Registration with coordination server
  - Periodic heartbeat transmission
  - Peer list synchronization
  - Mesh network establishment
  - Local routing configuration

## 4. Detailed Design

### 4.1 Peer Registration Flow

```
Client                           Server
  │                                 │
  │ 1. Generate WireGuard keypair   │
  │    (if not exists)              │
  │                                 │
  │ 2. POST /register               │
  │    {public_key, hostname, ...}  │
  ├────────────────────────────────►│
  │                                 │ 3. Allocate virtual IP
  │                                 │    from pool (10.100.0.0/16)
  │                                 │
  │                                 │ 4. Store peer metadata
  │                                 │    in registry
  │                                 │
  │ 5. Response                     │
  │    {assigned_ip, peer_id, ...}  │
  │◄────────────────────────────────┤
  │                                 │
  │ 6. Create WireGuard interface   │
  │    with assigned IP             │
  │                                 │
```

**Key Design Decisions**:
- Public keys serve as unique peer identifiers (prevents spoofing)
- IP allocation is deterministic and persistent per public key
- Registration is idempotent (re-registration returns same IP)

### 4.2 Mesh Network Establishment

```
Client A                         Server                   Client B
  │                                 │                         │
  │ 1. GET /peers?peer_id=A         │                         │
  ├────────────────────────────────►│                         │
  │                                 │                         │
  │ 2. {peers: [B, C, ...]}         │                         │
  │◄────────────────────────────────┤                         │
  │                                 │                         │
  │ 3. For each peer:               │                         │
  │    wg set wg0 peer <pubkey>     │                         │
  │    allowed-ips <peer_ip>/32     │                         │
  │    endpoint <peer_endpoint>     │                         │
  │                                 │                         │
  │ 4. WireGuard handshake          │                         │
  │◄────────────────────────────────┼────────────────────────►│
  │         (encrypted tunnel)      │                         │
```

**Key Design Decisions**:
- Each client independently configures all peers (eventual consistency)
- Peer list refresh interval: 60 seconds
- WireGuard handles encryption, authentication, and NAT traversal
- Allowed IPs set to /32 for point-to-point mesh

### 4.3 Health Monitoring

```
Client                           Server
  │                                 │
  │ Every 30 seconds:               │
  │ POST /heartbeat                 │
  │    {peer_id, endpoint}          │
  ├────────────────────────────────►│
  │                                 │ Update last_heartbeat
  │                                 │ Mark peer as online
  │ Response {success: true}        │
  │◄────────────────────────────────┤
  │                                 │

Server cleanup routine (every 60s):
  - If (now - last_heartbeat) > 120s: mark peer offline
  - Offline peers excluded from peer list responses
```

**Key Design Decisions**:
- Heartbeat interval: 30 seconds (balance between responsiveness and load)
- Timeout threshold: 120 seconds (allows 4 missed heartbeats)
- Graceful degradation: offline peers remain in registry for recovery

### 4.4 IP Address Management (IPAM)

**Default Network**: 10.100.0.0/16 (65,534 usable addresses)

**Allocation Strategy**:
```
┌────────────────────────────────────────────────┐
│ Network: 10.100.0.0/16                         │
├────────────────────────────────────────────────┤
│ 10.100.0.0      - Network address (reserved)   │
│ 10.100.0.1      - First client                 │
│ 10.100.0.2      - Second client                │
│ ...                                            │
│ 10.100.255.254  - Last usable                  │
│ 10.100.255.255  - Broadcast (reserved)         │
└────────────────────────────────────────────────┘
```

**Implementation**:
- Linear allocation with occupied-IP tracking
- Persistent mapping: public_key → IP address
- Re-allocation support for known peers

### 4.5 Exit Node Support

**Configuration**:
- Clients can register as exit nodes via flag: `--exit-node`
- Exit nodes advertise allowed IPs: `["0.0.0.0/0"]` (all traffic)

**Routing**:
```
Regular Client              Exit Node Client
┌─────────────┐            ┌─────────────────┐
│ AllowedIPs: │            │ AllowedIPs:     │
│ 10.100.0.2  │───────────►│ 0.0.0.0/0       │
│             │  WireGuard │ (+ IP forward)  │
└─────────────┘            └────────┬────────┘
                                    │
                                    │ NAT/Forward
                                    ▼
                              Internet
```

**Key Design Decisions**:
- Exit nodes enable IP forwarding
- Clients can select exit node by configuring routes
- Multiple exit nodes supported (client chooses)

## 5. Protocol Specification

### 5.1 Message Format

All messages use JSON over HTTP.

**Base Message**:
```json
{
  "type": "message_type",
  "timestamp": "2024-01-01T12:00:00Z",
  "payload": { ... }
}
```

### 5.2 API Endpoints

#### POST /register
**Request**:
```json
{
  "public_key": "base64-encoded-wireguard-public-key",
  "hostname": "my-laptop",
  "os": "linux",
  "endpoint": "1.2.3.4:51820",
  "request_ip": true,
  "exit_node": false,
  "allowed_ips": []
}
```

**Response**:
```json
{
  "success": true,
  "assigned_ip": "10.100.0.1",
  "network_cidr": "10.100.0.0/16",
  "peer_id": "peer-1234567890",
  "server_public_key": "base64-encoded-key"
}
```

#### POST /heartbeat
**Request**:
```json
{
  "peer_id": "peer-1234567890",
  "endpoint": "1.2.3.4:51820"
}
```

**Response**:
```json
{
  "success": true
}
```

#### GET /peers
**Query Parameters**: `?peer_id=peer-1234567890`

**Response**:
```json
{
  "peers": [
    {
      "id": "peer-0987654321",
      "public_key": "base64-encoded-key",
      "virtual_ip": "10.100.0.2",
      "endpoint": "5.6.7.8:51820",
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

## 6. Security Model

### 6.1 Threat Model

**In Scope**:
- Eavesdropping on VPN traffic (encrypted by WireGuard)
- Peer impersonation (prevented by public key authentication)
- Man-in-the-middle on mesh connections (WireGuard handshake)

**Out of Scope (Initial Version)**:
- Malicious coordination server (trust required)
- Peer authentication/authorization (any peer can join)
- DDoS protection on coordination server

### 6.2 Security Properties

1. **Confidentiality**: All mesh traffic encrypted with ChaCha20-Poly1305
2. **Integrity**: Poly1305 MAC ensures tamper detection
3. **Authentication**: WireGuard public keys authenticate peers
4. **Forward Secrecy**: WireGuard provides perfect forward secrecy
5. **Key Management**: Private keys never transmitted

### 6.3 Recommendations for Production

- **TLS**: Enable HTTPS for coordination server
- **Authentication**: Implement token-based peer authentication
- **ACLs**: Add access control lists for peer-to-peer connectivity
- **Rate Limiting**: Protect coordination server from abuse
- **Audit Logging**: Log all registration and configuration changes

## 7. Data Model

### 7.1 Peer Object
```go
type Peer struct {
    ID            string    // Unique peer identifier
    PublicKey     string    // WireGuard public key (base64)
    VirtualIP     string    // Assigned virtual IP
    Endpoint      string    // External endpoint (IP:port)
    Hostname      string    // Device hostname
    OS            string    // Operating system
    AllowedIPs    []string  // CIDR ranges for routing
    ExitNode      bool      // Exit node capability
    LastHeartbeat time.Time // Last heartbeat timestamp
    Online        bool      // Current online status
}
```

### 7.2 Persistent Storage

**Server Storage**: JSON file (upgradeable to SQLite/PostgreSQL)
- Path: `~/.config/wireguard-mesh/peers.json`
- Format: Array of Peer objects

**Client Storage**: JSON configuration file
- Path: `~/.config/wireguard-mesh/client.json`
- Contains: keys, server address, assigned IP

## 8. Platform-Specific Implementation

### 8.1 Linux

**WireGuard Interface Creation**:
```bash
ip link add dev wg0 type wireguard
ip addr add 10.100.0.1/32 dev wg0
ip link set up dev wg0
wg set wg0 private-key <key> listen-port 51820
```

**Requirements**:
- Kernel module: `wireguard` (kernel 5.6+) or `wireguard-dkms`
- User-space: `wireguard-go` (fallback)

### 8.2 macOS

**WireGuard Interface Creation**:
```bash
# Use wireguard-go userspace implementation
wireguard-go wg0
ifconfig wg0 inet 10.100.0.1/32 10.100.0.1
ifconfig wg0 up
```

**Requirements**:
- `wireguard-go` (via Homebrew)
- `wireguard-tools` for `wg` command

### 8.3 Windows

**WireGuard Interface Creation**:
- Use WireGuard Windows service API
- Or shell out to `wireguard.exe` CLI

**Requirements**:
- WireGuard for Windows (official installer)
- Administrator privileges for interface creation

## 9. Performance Considerations

### 9.1 Scalability Limits

**Coordination Server**:
- Expected load: 1 registration/client + 2 req/min/client (heartbeat + peer sync)
- For 1000 clients: ~2000 req/min (~33 req/sec)
- Bottleneck: Peer list serialization (O(n) per request)

**Mesh Network**:
- Each client maintains n-1 WireGuard tunnels
- Practical limit: ~100-200 peers per client
- Bottleneck: WireGuard peer lookup (linear search)

### 9.2 Optimization Strategies

1. **Server**:
   - Cache serialized peer list (invalidate on updates)
   - Use database with indexing for large deployments
   - Implement peer list pagination

2. **Client**:
   - Incremental peer updates (delta sync)
   - Configurable peer sync interval
   - Lazy tunnel establishment (on-demand)

## 10. Monitoring & Observability

### 10.1 Metrics

**Server Metrics**:
- Total registered peers
- Active/online peers
- Registration rate
- Heartbeat success/failure rate
- API request latency

**Client Metrics**:
- Tunnel establishment success rate
- Data transfer bytes (tx/rx per peer)
- Heartbeat latency
- Peer list sync latency

### 10.2 Logging

**Log Levels**:
- ERROR: Failed registrations, interface creation failures
- WARN: Missed heartbeats, peer timeouts
- INFO: Registration events, peer status changes
- DEBUG: API requests, WireGuard commands

## 11. Deployment Model

### 11.1 Server Deployment

**Recommended Setup**:
- Cloud VM with public IP (AWS EC2, GCP Compute, etc.)
- Minimum specs: 1 vCPU, 512MB RAM (for <100 clients)
- Firewall: Allow TCP 8080 (API) and UDP 51820 (WireGuard)

**HA/Reliability**:
- Use systemd/supervisor for auto-restart
- Regular backups of peer database
- Consider multi-region deployment for redundancy

### 11.2 Client Deployment

**Installation**:
```bash
# Download binary
wget https://releases.example.com/vpn-client

# Make executable
chmod +x vpn-client

# Run (requires root)
sudo ./vpn-client -server http://SERVER_IP:8080
```

**Autostart**:
- Linux: systemd service
- macOS: launchd
- Windows: Windows Service

## 12. Testing Strategy

### 12.1 Unit Tests
- IPAM allocation/deallocation
- Peer serialization/deserialization
- Key generation and validation
- Protocol message encoding

### 12.2 Integration Tests
- Client registration flow
- Heartbeat mechanism
- Peer synchronization
- Mesh establishment

### 12.3 End-to-End Tests
- Multi-client connectivity
- Exit node routing
- Peer failure/recovery
- Cross-platform compatibility

## 13. Future Enhancements

### 13.1 Short-term
- [ ] TLS support for coordination server
- [ ] Systemd/launchd service files
- [ ] Graceful shutdown and cleanup
- [ ] Configuration file validation

### 13.2 Medium-term
- [ ] STUN-based NAT traversal
- [ ] Relay server for difficult NAT scenarios
- [ ] Built-in DNS server (*.mesh TLD)
- [ ] ACL support for peer access control
- [ ] Prometheus metrics exporter

### 13.3 Long-term
- [ ] Mobile clients (iOS/Android)
- [ ] Web-based admin dashboard
- [ ] Multi-server federation
- [ ] BGP integration for advanced routing
- [ ] SSO/OAuth integration

## 14. References

- [WireGuard Protocol](https://www.wireguard.com/protocol/)
- [WireGuard Cross-Platform](https://www.wireguard.com/xplatform/)
- [wgctrl-go Library](https://github.com/WireGuard/wgctrl-go)
- [Tailscale Architecture](https://tailscale.com/blog/how-tailscale-works/)
- [RFC 4632 - CIDR](https://tools.ietf.org/html/rfc4632)
