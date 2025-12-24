package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	ListenAddr  string `json:"listen_addr"`
	NetworkCIDR string `json:"network_cidr"`
	PrivateKey  string `json:"private_key,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	DBPath      string `json:"db_path"`
}

// ClientConfig holds the client configuration
type ClientConfig struct {
	ServerAddr    string `json:"server_addr"`
	InterfaceName string `json:"interface_name"`
	PrivateKey    string `json:"private_key,omitempty"`
	PublicKey     string `json:"public_key,omitempty"`
	PeerID        string `json:"peer_id,omitempty"`
	AssignedIP    string `json:"assigned_ip,omitempty"`
	ExitNode      bool   `json:"exit_node"`
	ListenPort    int    `json:"listen_port"`
}

// DefaultServerConfig returns the default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ListenAddr:  ":8080",
		NetworkCIDR: "10.100.0.0/16",
		DBPath:      getDefaultDBPath(),
	}
}

// DefaultClientConfig returns the default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ServerAddr:    "https://vpn.example.com:8080",
		InterfaceName: "wg0",
		ExitNode:      false,
		ListenPort:    51820,
	}
}

// LoadServerConfig loads server configuration from file
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			config := DefaultServerConfig()
			if err := SaveServerConfig(path, config); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SaveServerConfig saves server configuration to file
func SaveServerConfig(path string, config *ServerConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadClientConfig loads client configuration from file
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			config := DefaultClientConfig()
			if err := SaveClientConfig(path, config); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClientConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SaveClientConfig saves client configuration to file
func SaveClientConfig(path string, config *ClientConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfigDir returns the default configuration directory
func GetDefaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "wireguard-mesh")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "wireguard-mesh")
	default: // linux
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "wireguard-mesh")
	}
}

// getDefaultDBPath returns the default database path
func getDefaultDBPath() string {
	return filepath.Join(GetDefaultConfigDir(), "peers.json")
}

// GetDefaultServerConfigPath returns the default server config path
func GetDefaultServerConfigPath() string {
	return filepath.Join(GetDefaultConfigDir(), "server.json")
}

// GetDefaultClientConfigPath returns the default client config path
func GetDefaultClientConfigPath() string {
	return filepath.Join(GetDefaultConfigDir(), "client.json")
}
