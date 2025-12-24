package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/vpn/wireguard-mesh/pkg/config"
	"github.com/vpn/wireguard-mesh/pkg/crypto"
	"github.com/vpn/wireguard-mesh/pkg/protocol"
	"github.com/vpn/wireguard-mesh/pkg/wireguard"
)

const (
	HeartbeatInterval = 30 * time.Second
	PeerSyncInterval  = 60 * time.Second
	RetryInterval     = 10 * time.Second
)

// Client represents the VPN client
type Client struct {
	config     *config.ClientConfig
	wgInterface *wireguard.Interface
	httpClient *http.Client
	privateKey string
	publicKey  string
	peerID     string
	assignedIP string
	networkCIDR string
	serverPublicKey string
	stopChan   chan struct{}
}

// NewClient creates a new VPN client
func NewClient(cfg *config.ClientConfig) (*Client, error) {
	// Generate or load client keys
	var privateKey, publicKey string
	if cfg.PrivateKey == "" {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate client keys: %w", err)
		}
		privateKey = keyPair.PrivateKeyToString()
		publicKey = keyPair.PublicKeyToString()

		// Save keys to config
		cfg.PrivateKey = privateKey
		cfg.PublicKey = publicKey
		if err := config.SaveClientConfig(config.GetDefaultClientConfigPath(), cfg); err != nil {
			log.Printf("Warning: failed to save client config: %v", err)
		}
	} else {
		privateKey = cfg.PrivateKey
		publicKey = cfg.PublicKey
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &Client{
		config:     cfg,
		httpClient: httpClient,
		privateKey: privateKey,
		publicKey:  publicKey,
		stopChan:   make(chan struct{}),
	}, nil
}

// Start starts the VPN client
func (c *Client) Start() error {
	log.Printf("Starting VPN client...")
	log.Printf("Client public key: %s", c.publicKey)

	// Register with server
	if err := c.register(); err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}

	// Create and configure WireGuard interface
	if err := c.setupInterface(); err != nil {
		return fmt.Errorf("failed to setup interface: %w", err)
	}

	// Start background routines
	go c.heartbeatRoutine()
	go c.peerSyncRoutine()

	log.Printf("VPN client started successfully")
	log.Printf("Virtual IP: %s", c.assignedIP)
	log.Printf("Network: %s", c.networkCIDR)

	// Wait for stop signal
	<-c.stopChan

	return nil
}

// Stop stops the VPN client
func (c *Client) Stop() error {
	log.Printf("Stopping VPN client...")

	close(c.stopChan)

	if c.wgInterface != nil {
		if err := c.wgInterface.Destroy(); err != nil {
			log.Printf("Warning: failed to destroy interface: %v", err)
		}
	}

	log.Printf("VPN client stopped")
	return nil
}

// register registers the client with the server
func (c *Client) register() error {
	hostname, _ := os.Hostname()

	req := protocol.RegisterRequest{
		PublicKey: c.publicKey,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		RequestIP: true,
		ExitNode:  c.config.ExitNode,
	}

	// Try to detect our external endpoint
	endpoint, err := c.detectEndpoint()
	if err == nil {
		req.Endpoint = endpoint
	}

	var resp protocol.RegisterResponse
	if err := c.sendRequest("/register", req, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Error)
	}

	c.peerID = resp.PeerID
	c.assignedIP = resp.AssignedIP
	c.networkCIDR = resp.NetworkCIDR
	c.serverPublicKey = resp.ServerPublicKey

	// Update config
	c.config.PeerID = c.peerID
	c.config.AssignedIP = c.assignedIP
	if err := config.SaveClientConfig(config.GetDefaultClientConfigPath(), c.config); err != nil {
		log.Printf("Warning: failed to save client config: %v", err)
	}

	log.Printf("Registered with server: Peer ID = %s, IP = %s", c.peerID, c.assignedIP)

	return nil
}

// setupInterface sets up the WireGuard interface
func (c *Client) setupInterface() error {
	wgConfig := wireguard.Config{
		InterfaceName: c.config.InterfaceName,
		PrivateKey:    c.privateKey,
		ListenPort:    c.config.ListenPort,
		Address:       c.assignedIP + "/32",
	}

	wgInterface, err := wireguard.NewInterface(wgConfig)
	if err != nil {
		return err
	}

	if err := wgInterface.Create(); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	if err := wgInterface.Configure(); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	c.wgInterface = wgInterface

	// Initial peer sync
	if err := c.syncPeers(); err != nil {
		log.Printf("Warning: initial peer sync failed: %v", err)
	}

	return nil
}

// heartbeatRoutine sends periodic heartbeats to the server
func (c *Client) heartbeatRoutine() {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.sendHeartbeat(); err != nil {
				log.Printf("Heartbeat failed: %v", err)
			}
		case <-c.stopChan:
			return
		}
	}
}

// sendHeartbeat sends a heartbeat to the server
func (c *Client) sendHeartbeat() error {
	endpoint, _ := c.detectEndpoint()

	req := protocol.HeartbeatRequest{
		PeerID:   c.peerID,
		Endpoint: endpoint,
	}

	var resp protocol.HeartbeatResponse
	if err := c.sendRequest("/heartbeat", req, &resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("heartbeat failed: %s", resp.Error)
	}

	return nil
}

// peerSyncRoutine periodically syncs peers from the server
func (c *Client) peerSyncRoutine() {
	ticker := time.NewTicker(PeerSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.syncPeers(); err != nil {
				log.Printf("Peer sync failed: %v", err)
			}
		case <-c.stopChan:
			return
		}
	}
}

// syncPeers synchronizes peer list from the server
func (c *Client) syncPeers() error {
	url := fmt.Sprintf("%s/peers?peer_id=%s", c.config.ServerAddr, c.peerID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch peers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var peerList protocol.PeerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&peerList); err != nil {
		return fmt.Errorf("failed to decode peer list: %w", err)
	}

	// Update WireGuard peers
	for _, peer := range peerList.Peers {
		if !peer.Online {
			continue
		}

		peerConfig := wireguard.PeerConfig{
			PublicKey:  peer.PublicKey,
			Endpoint:   peer.Endpoint,
			AllowedIPs: peer.AllowedIPs,
			KeepAlive:  25 * time.Second,
		}

		if err := c.wgInterface.AddPeer(peerConfig); err != nil {
			log.Printf("Warning: failed to add peer %s: %v", peer.ID, err)
			continue
		}

		log.Printf("Synced peer: %s (%s) at %s", peer.ID, peer.Hostname, peer.VirtualIP)
	}

	return nil
}

// detectEndpoint tries to detect the client's external endpoint
func (c *Client) detectEndpoint() (string, error) {
	// Get local interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip != nil {
				return fmt.Sprintf("%s:%d", ip.String(), c.config.ListenPort), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable endpoint found")
}

// sendRequest sends a JSON request to the server
func (c *Client) sendRequest(path string, req interface{}, resp interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.config.ServerAddr + path
	httpResp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", httpResp.StatusCode)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// Status returns the current client status
func (c *Client) Status() (map[string]interface{}, error) {
	status := map[string]interface{}{
		"peer_id":     c.peerID,
		"assigned_ip": c.assignedIP,
		"network":     c.networkCIDR,
		"public_key":  c.publicKey,
	}

	if c.wgInterface != nil {
		stats, err := c.wgInterface.GetStats()
		if err == nil {
			status["interface"] = stats
		}
	}

	return status, nil
}
