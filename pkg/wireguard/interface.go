package wireguard

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	DefaultInterfaceName = "wg0"
	DefaultListenPort    = 51820
)

// Interface represents a WireGuard network interface
type Interface struct {
	Name       string
	PrivateKey string
	ListenPort int
	Address    string
	client     *wgctrl.Client
}

// Config holds the configuration for a WireGuard interface
type Config struct {
	InterfaceName string
	PrivateKey    string
	ListenPort    int
	Address       string
}

// PeerConfig represents the configuration for a WireGuard peer
type PeerConfig struct {
	PublicKey  string
	Endpoint   string
	AllowedIPs []string
	KeepAlive  time.Duration
}

// NewInterface creates a new WireGuard interface
func NewInterface(config Config) (*Interface, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	iface := &Interface{
		Name:       config.InterfaceName,
		PrivateKey: config.PrivateKey,
		ListenPort: config.ListenPort,
		Address:    config.Address,
		client:     client,
	}

	return iface, nil
}

// Create creates the WireGuard interface
func (i *Interface) Create() error {
	switch runtime.GOOS {
	case "linux":
		return i.createLinux()
	case "darwin":
		return i.createDarwin()
	case "windows":
		return i.createWindows()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Configure configures the WireGuard interface
func (i *Interface) Configure() error {
	privateKey, err := wgtypes.ParseKey(i.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	port := i.ListenPort
	config := wgtypes.Config{
		PrivateKey: &privateKey,
		ListenPort: &port,
	}

	if err := i.client.ConfigureDevice(i.Name, config); err != nil {
		return fmt.Errorf("failed to configure device: %w", err)
	}

	return nil
}

// AddPeer adds a peer to the WireGuard interface
func (i *Interface) AddPeer(peer PeerConfig) error {
	publicKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	var endpoint *net.UDPAddr
	if peer.Endpoint != "" {
		endpoint, err = net.ResolveUDPAddr("udp", peer.Endpoint)
		if err != nil {
			return fmt.Errorf("failed to resolve endpoint: %w", err)
		}
	}

	allowedIPs := make([]net.IPNet, len(peer.AllowedIPs))
	for j, ip := range peer.AllowedIPs {
		_, ipNet, err := net.ParseCIDR(ip)
		if err != nil {
			// Try parsing as single IP
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				return fmt.Errorf("invalid IP or CIDR: %s", ip)
			}
			// Convert to CIDR
			if parsedIP.To4() != nil {
				_, ipNet, _ = net.ParseCIDR(ip + "/32")
			} else {
				_, ipNet, _ = net.ParseCIDR(ip + "/128")
			}
		}
		allowedIPs[j] = *ipNet
	}

	keepAlive := peer.KeepAlive
	if keepAlive == 0 {
		keepAlive = 25 * time.Second
	}

	peerConfig := wgtypes.PeerConfig{
		PublicKey:                   publicKey,
		Endpoint:                    endpoint,
		AllowedIPs:                  allowedIPs,
		PersistentKeepaliveInterval: &keepAlive,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := i.client.ConfigureDevice(i.Name, config); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	return nil
}

// RemovePeer removes a peer from the WireGuard interface
func (i *Interface) RemovePeer(publicKey string) error {
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	peerConfig := wgtypes.PeerConfig{
		PublicKey: key,
		Remove:    true,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := i.client.ConfigureDevice(i.Name, config); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	return nil
}

// Destroy destroys the WireGuard interface
func (i *Interface) Destroy() error {
	defer i.client.Close()

	switch runtime.GOOS {
	case "linux":
		return i.destroyLinux()
	case "darwin":
		return i.destroyDarwin()
	case "windows":
		return i.destroyWindows()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Platform-specific implementations are in interface_unix.go and interface_windows.go

// GetStats returns statistics for the interface
func (i *Interface) GetStats() (map[string]interface{}, error) {
	device, err := i.client.Device(i.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	stats := map[string]interface{}{
		"name":        device.Name,
		"public_key":  device.PublicKey.String(),
		"listen_port": device.ListenPort,
		"num_peers":   len(device.Peers),
		"peers":       []map[string]interface{}{},
	}

	peers := []map[string]interface{}{}
	for _, peer := range device.Peers {
		peerStats := map[string]interface{}{
			"public_key":            peer.PublicKey.String(),
			"endpoint":              peer.Endpoint,
			"last_handshake":        peer.LastHandshakeTime,
			"receive_bytes":         peer.ReceiveBytes,
			"transmit_bytes":        peer.TransmitBytes,
			"allowed_ips":           peer.AllowedIPs,
			"persistent_keepalive":  peer.PersistentKeepaliveInterval,
		}
		peers = append(peers, peerStats)
	}
	stats["peers"] = peers

	return stats, nil
}
