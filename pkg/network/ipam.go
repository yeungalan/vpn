package network

import (
	"fmt"
	"net"
	"sync"
)

// IPAllocator manages IP address allocation for the VPN network
type IPAllocator struct {
	network    *net.IPNet
	allocated  map[string]bool
	nextIP     net.IP
	mu         sync.RWMutex
}

// NewIPAllocator creates a new IP allocator for the given CIDR
func NewIPAllocator(cidr string) (*IPAllocator, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Start from the first usable IP (skip network address)
	nextIP := make(net.IP, len(network.IP))
	copy(nextIP, network.IP)
	nextIP = incrementIP(nextIP)

	return &IPAllocator{
		network:   network,
		allocated: make(map[string]bool),
		nextIP:    nextIP,
	}, nil
}

// AllocateIP allocates the next available IP address
func (a *IPAllocator) AllocateIP() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for {
		if !a.network.Contains(a.nextIP) {
			return "", fmt.Errorf("no more IP addresses available in network %s", a.network.String())
		}

		ip := a.nextIP.String()
		a.nextIP = incrementIP(a.nextIP)

		// Skip broadcast address
		if isBroadcast(net.ParseIP(ip), a.network) {
			continue
		}

		if !a.allocated[ip] {
			a.allocated[ip] = true
			return ip, nil
		}
	}
}

// AllocateSpecificIP allocates a specific IP address if available
func (a *IPAllocator) AllocateSpecificIP(ip string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	if !a.network.Contains(parsedIP) {
		return fmt.Errorf("IP %s not in network %s", ip, a.network.String())
	}

	if a.allocated[ip] {
		return fmt.Errorf("IP %s already allocated", ip)
	}

	a.allocated[ip] = true
	return nil
}

// ReleaseIP releases an allocated IP address
func (a *IPAllocator) ReleaseIP(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.allocated, ip)
}

// IsAllocated checks if an IP is allocated
func (a *IPAllocator) IsAllocated(ip string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.allocated[ip]
}

// GetNetworkCIDR returns the network CIDR
func (a *IPAllocator) GetNetworkCIDR() string {
	return a.network.String()
}

// incrementIP increments an IP address
func incrementIP(ip net.IP) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)

	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] > 0 {
			break
		}
	}

	return result
}

// isBroadcast checks if an IP is the broadcast address for the network
func isBroadcast(ip net.IP, network *net.IPNet) bool {
	broadcast := make(net.IP, len(network.IP))
	for i := range network.IP {
		broadcast[i] = network.IP[i] | ^network.Mask[i]
	}
	return ip.Equal(broadcast)
}
