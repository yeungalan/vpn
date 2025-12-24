package protocol

import (
	"encoding/json"
	"time"
)

// MessageType defines the type of protocol message
type MessageType string

const (
	MsgTypeRegister   MessageType = "register"
	MsgTypeHeartbeat  MessageType = "heartbeat"
	MsgTypePeerList   MessageType = "peer_list"
	MsgTypeUpdatePeer MessageType = "update_peer"
	MsgTypeRemovePeer MessageType = "remove_peer"
)

// Message is the base protocol message structure
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// RegisterRequest is sent by clients to register with the server
type RegisterRequest struct {
	PublicKey  string   `json:"public_key"`
	Hostname   string   `json:"hostname"`
	OS         string   `json:"os"`
	Endpoint   string   `json:"endpoint,omitempty"` // External endpoint if known
	RequestIP  bool     `json:"request_ip"`
	ExitNode   bool     `json:"exit_node"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
}

// RegisterResponse is sent by server after successful registration
type RegisterResponse struct {
	Success    bool     `json:"success"`
	Error      string   `json:"error,omitempty"`
	AssignedIP string   `json:"assigned_ip"`
	NetworkCIDR string  `json:"network_cidr"`
	PeerID     string   `json:"peer_id"`
	ServerPublicKey string `json:"server_public_key"`
}

// Peer represents a peer in the network
type Peer struct {
	ID            string    `json:"id"`
	PublicKey     string    `json:"public_key"`
	VirtualIP     string    `json:"virtual_ip"`
	Endpoint      string    `json:"endpoint,omitempty"`
	Hostname      string    `json:"hostname"`
	OS            string    `json:"os"`
	AllowedIPs    []string  `json:"allowed_ips"`
	ExitNode      bool      `json:"exit_node"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Online        bool      `json:"online"`
}

// HeartbeatRequest is sent periodically by clients
type HeartbeatRequest struct {
	PeerID   string `json:"peer_id"`
	Endpoint string `json:"endpoint,omitempty"`
}

// HeartbeatResponse acknowledges the heartbeat
type HeartbeatResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// PeerListRequest requests the current peer list
type PeerListRequest struct {
	PeerID string `json:"peer_id"`
}

// PeerListResponse contains the list of all peers
type PeerListResponse struct {
	Peers []Peer `json:"peers"`
}

// PeerUpdate notifies about peer changes
type PeerUpdate struct {
	Action string `json:"action"` // "add", "update", "remove"
	Peer   *Peer  `json:"peer,omitempty"`
	PeerID string `json:"peer_id,omitempty"`
}

// NewMessage creates a new protocol message
func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   data,
	}, nil
}

// Decode decodes the message payload into the provided structure
func (m *Message) Decode(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}
