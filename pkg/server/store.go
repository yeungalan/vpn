package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vpn/wireguard-mesh/pkg/protocol"
)

// PeerStore handles persistent storage of peer information
type PeerStore struct {
	path string
	mu   sync.RWMutex
}

// NewPeerStore creates a new peer store
func NewPeerStore(path string) (*PeerStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	return &PeerStore{
		path: path,
	}, nil
}

// SavePeer saves a peer to the store
func (s *PeerStore) SavePeer(peer *protocol.Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, err := s.loadPeersUnlocked()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Update or add peer
	found := false
	for i, p := range peers {
		if p.ID == peer.ID {
			peers[i] = peer
			found = true
			break
		}
	}

	if !found {
		peers = append(peers, peer)
	}

	return s.savePeersUnlocked(peers)
}

// LoadPeers loads all peers from the store
func (s *PeerStore) LoadPeers() ([]*protocol.Peer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadPeersUnlocked()
}

// DeletePeer deletes a peer from the store
func (s *PeerStore) DeletePeer(peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, err := s.loadPeersUnlocked()
	if err != nil {
		return err
	}

	// Filter out the peer
	filtered := make([]*protocol.Peer, 0, len(peers))
	for _, p := range peers {
		if p.ID != peerID {
			filtered = append(filtered, p)
		}
	}

	return s.savePeersUnlocked(filtered)
}

// loadPeersUnlocked loads peers without locking (internal use)
func (s *PeerStore) loadPeersUnlocked() ([]*protocol.Peer, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*protocol.Peer{}, nil
		}
		return nil, fmt.Errorf("failed to read store: %w", err)
	}

	var peers []*protocol.Peer
	if err := json.Unmarshal(data, &peers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peers: %w", err)
	}

	return peers, nil
}

// savePeersUnlocked saves peers without locking (internal use)
func (s *PeerStore) savePeersUnlocked(peers []*protocol.Peer) error {
	data, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal peers: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write store: %w", err)
	}

	return nil
}
