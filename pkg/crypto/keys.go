package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

const (
	KeySize = 32
)

// KeyPair represents a WireGuard key pair
type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

// GenerateKeyPair generates a new WireGuard-compatible key pair
func GenerateKeyPair() (*KeyPair, error) {
	privateKey := make([]byte, KeySize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the private key for Curve25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Generate public key from private key
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// PrivateKeyToString encodes the private key to base64
func (k *KeyPair) PrivateKeyToString() string {
	return base64.StdEncoding.EncodeToString(k.PrivateKey)
}

// PublicKeyToString encodes the public key to base64
func (k *KeyPair) PublicKeyToString() string {
	return base64.StdEncoding.EncodeToString(k.PublicKey)
}

// ParsePrivateKey decodes a base64-encoded private key
func ParsePrivateKey(key string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}
	if len(decoded) != KeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(decoded))
	}
	return decoded, nil
}

// ParsePublicKey decodes a base64-encoded public key
func ParsePublicKey(key string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	if len(decoded) != KeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(decoded))
	}
	return decoded, nil
}

// DerivePublicKey derives the public key from a private key
func DerivePublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != KeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(privateKey))
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return publicKey, nil
}
