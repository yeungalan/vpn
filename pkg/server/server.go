package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/vpn/wireguard-mesh/pkg/config"
	"github.com/vpn/wireguard-mesh/pkg/crypto"
	"github.com/vpn/wireguard-mesh/pkg/network"
	"github.com/vpn/wireguard-mesh/pkg/protocol"
)

const (
	HeartbeatTimeout = 2 * time.Minute
	CleanupInterval  = 1 * time.Minute
)

// Server represents the VPN coordination server
type Server struct {
	config     *config.ServerConfig
	ipAllocator *network.IPAllocator
	peers      map[string]*protocol.Peer
	peersByKey map[string]string
	mu         sync.RWMutex
	privateKey string
	publicKey  string
	store      *PeerStore
}

// NewServer creates a new VPN coordination server
func NewServer(cfg *config.ServerConfig) (*Server, error) {
	ipAllocator, err := network.NewIPAllocator(cfg.NetworkCIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP allocator: %w", err)
	}

	// Generate or load server keys
	var privateKey, publicKey string
	if cfg.PrivateKey == "" {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate server keys: %w", err)
		}
		privateKey = keyPair.PrivateKeyToString()
		publicKey = keyPair.PublicKeyToString()

		// Save keys to config
		cfg.PrivateKey = privateKey
		cfg.PublicKey = publicKey
		if err := config.SaveServerConfig(config.GetDefaultServerConfigPath(), cfg); err != nil {
			log.Printf("Warning: failed to save server config: %v", err)
		}
	} else {
		privateKey = cfg.PrivateKey
		publicKey = cfg.PublicKey
	}

	store, err := NewPeerStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer store: %w", err)
	}

	s := &Server{
		config:      cfg,
		ipAllocator: ipAllocator,
		peers:       make(map[string]*protocol.Peer),
		peersByKey:  make(map[string]string),
		privateKey:  privateKey,
		publicKey:   publicKey,
		store:       store,
	}

	// Load existing peers from store
	if err := s.loadPeersFromStore(); err != nil {
		log.Printf("Warning: failed to load peers from store: %v", err)
	}

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Start cleanup routine
	go s.cleanupRoutine()

	// Setup HTTP handlers
	http.HandleFunc("/register", s.handleRegister)
	http.HandleFunc("/heartbeat", s.handleHeartbeat)
	http.HandleFunc("/peers", s.handlePeerList)

	log.Printf("Server starting on %s", s.config.ListenAddr)
	log.Printf("Server public key: %s", s.publicKey)
	log.Printf("Network CIDR: %s", s.config.NetworkCIDR)

	return http.ListenAndServe(s.config.ListenAddr, nil)
}

// handleRegister handles peer registration requests
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if peer already exists
	if peerID, exists := s.peersByKey[req.PublicKey]; exists {
		peer := s.peers[peerID]
		resp := protocol.RegisterResponse{
			Success:         true,
			AssignedIP:      peer.VirtualIP,
			NetworkCIDR:     s.ipAllocator.GetNetworkCIDR(),
			PeerID:          peer.ID,
			ServerPublicKey: s.publicKey,
		}

		// Update peer info
		peer.Hostname = req.Hostname
		peer.OS = req.OS
		peer.Endpoint = req.Endpoint
		peer.LastHeartbeat = time.Now()
		peer.Online = true

		s.store.SavePeer(peer)

		json.NewEncoder(w).Encode(resp)
		return
	}

	// Allocate new IP
	ip, err := s.ipAllocator.AllocateIP()
	if err != nil {
		resp := protocol.RegisterResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Create new peer
	peerID := generatePeerID()
	peer := &protocol.Peer{
		ID:            peerID,
		PublicKey:     req.PublicKey,
		VirtualIP:     ip,
		Endpoint:      req.Endpoint,
		Hostname:      req.Hostname,
		OS:            req.OS,
		AllowedIPs:    []string{ip + "/32"},
		ExitNode:      req.ExitNode,
		LastHeartbeat: time.Now(),
		Online:        true,
	}

	if req.ExitNode {
		peer.AllowedIPs = append(peer.AllowedIPs, "0.0.0.0/0")
	}

	s.peers[peerID] = peer
	s.peersByKey[req.PublicKey] = peerID

	// Save to store
	if err := s.store.SavePeer(peer); err != nil {
		log.Printf("Failed to save peer to store: %v", err)
	}

	resp := protocol.RegisterResponse{
		Success:         true,
		AssignedIP:      ip,
		NetworkCIDR:     s.ipAllocator.GetNetworkCIDR(),
		PeerID:          peerID,
		ServerPublicKey: s.publicKey,
	}

	log.Printf("Registered new peer: %s (%s) with IP %s", peerID, req.Hostname, ip)

	json.NewEncoder(w).Encode(resp)
}

// handleHeartbeat handles heartbeat requests
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[req.PeerID]
	if !exists {
		resp := protocol.HeartbeatResponse{
			Success: false,
			Error:   "Peer not found",
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	peer.LastHeartbeat = time.Now()
	peer.Online = true
	if req.Endpoint != "" {
		peer.Endpoint = req.Endpoint
	}

	s.store.SavePeer(peer)

	resp := protocol.HeartbeatResponse{
		Success: true,
	}

	json.NewEncoder(w).Encode(resp)
}

// handlePeerList handles peer list requests
func (s *Server) handlePeerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peerID := r.URL.Query().Get("peer_id")
	if peerID == "" {
		http.Error(w, "Missing peer_id", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Verify peer exists
	if _, exists := s.peers[peerID]; !exists {
		http.Error(w, "Peer not found", http.StatusNotFound)
		return
	}

	// Return all other peers (excluding the requesting peer)
	peers := make([]protocol.Peer, 0, len(s.peers)-1)
	for id, peer := range s.peers {
		if id != peerID {
			peers = append(peers, *peer)
		}
	}

	resp := protocol.PeerListResponse{
		Peers: peers,
	}

	json.NewEncoder(w).Encode(resp)
}

// cleanupRoutine periodically cleans up stale peers
func (s *Server) cleanupRoutine() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()

		for id, peer := range s.peers {
			if now.Sub(peer.LastHeartbeat) > HeartbeatTimeout {
				if peer.Online {
					peer.Online = false
					log.Printf("Peer %s (%s) went offline", id, peer.Hostname)
					s.store.SavePeer(peer)
				}
			}
		}

		s.mu.Unlock()
	}
}

// loadPeersFromStore loads peers from persistent storage
func (s *Server) loadPeersFromStore() error {
	peers, err := s.store.LoadPeers()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.peers[peer.ID] = peer
		s.peersByKey[peer.PublicKey] = peer.ID

		// Re-allocate the IP
		if err := s.ipAllocator.AllocateSpecificIP(peer.VirtualIP); err != nil {
			log.Printf("Warning: failed to re-allocate IP %s for peer %s: %v", peer.VirtualIP, peer.ID, err)
		}
	}

	log.Printf("Loaded %d peers from store", len(peers))
	return nil
}

// generatePeerID generates a unique peer ID
func generatePeerID() string {
	return fmt.Sprintf("peer-%d", time.Now().UnixNano())
}
