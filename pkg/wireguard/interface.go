package wireguard

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

// Platform-specific implementations

func (i *Interface) createLinux() error {
	// Create interface using ip link
	cmd := exec.Command("ip", "link", "add", "dev", i.Name, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create interface: %w, output: %s", err, string(output))
	}

	// Set IP address
	cmd = exec.Command("ip", "addr", "add", i.Address, "dev", i.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w, output: %s", err, string(output))
	}

	// Bring interface up
	cmd = exec.Command("ip", "link", "set", "up", "dev", i.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up interface: %w, output: %s", err, string(output))
	}

	return nil
}

func (i *Interface) createDarwin() error {
	// On macOS, we use wireguard-go userspace implementation
	// The interface is created differently
	cmd := exec.Command("wireguard-go", i.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if already exists
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to create interface: %w, output: %s", err, string(output))
		}
	}

	// Set IP address
	cmd = exec.Command("ifconfig", i.Name, "inet", i.Address, i.Address)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w, output: %s", err, string(output))
	}

	// Bring interface up
	cmd = exec.Command("ifconfig", i.Name, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up interface: %w, output: %s", err, string(output))
	}

	return nil
}

func (i *Interface) createWindows() error {
	// On Windows, we use wireguard-go userspace implementation
	// This is simpler and more portable than using the Windows service

	// Start wireguard-go
	cmd := exec.Command("wireguard-go", i.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if already exists
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to create interface: %w, output: %s", err, string(output))
		}
	}

	// Wait a moment for interface to be ready
	time.Sleep(500 * time.Millisecond)

	// Set IP address using netsh
	// Extract IP and mask from CIDR notation
	ip := strings.Split(i.Address, "/")[0]

	// Add IP address
	cmd = exec.Command("netsh", "interface", "ip", "set", "address",
		"name="+i.Name, "static", ip, "255.255.255.255")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set IP address: %w, output: %s", err, string(output))
	}

	return nil
}

func (i *Interface) destroyLinux() error {
	cmd := exec.Command("ip", "link", "del", "dev", i.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to destroy interface: %w, output: %s", err, string(output))
	}
	return nil
}

func (i *Interface) destroyDarwin() error {
	// Kill wireguard-go process
	cmd := exec.Command("pkill", "-f", "wireguard-go "+i.Name)
	_ = cmd.Run() // Ignore errors as process might not exist

	return nil
}

func (i *Interface) destroyWindows() error {
	// Kill wireguard-go process
	cmd := exec.Command("taskkill", "/F", "/IM", "wireguard-go.exe")
	_ = cmd.Run() // Ignore errors as process might not exist

	// Alternative: try to kill by window title/interface name
	cmd = exec.Command("wmic", "process", "where",
		fmt.Sprintf("name='wireguard-go.exe' and commandline like '%%%s%%'", i.Name),
		"delete")
	_ = cmd.Run() // Ignore errors

	return nil
}

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
