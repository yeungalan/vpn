//go:build linux || darwin

package wireguard

import (
	"fmt"
	"os/exec"
	"strings"
)

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
	// On macOS, interface names must be utun[0-9]*
	// Let the system pick the next available utun interface
	utunName := "utun"

	cmd := exec.Command("wireguard-go", utunName)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if already exists
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to create interface: %w, output: %s", err, string(output))
		}
	}

	// Extract actual interface name from UAPI socket path
	// wireguard-go creates /var/run/wireguard/utunX.sock
	// For now, try utun interfaces in order
	actualName := ""
	for j := 0; j < 10; j++ {
		testName := fmt.Sprintf("utun%d", j)
		cmd = exec.Command("ifconfig", testName)
		if cmd.Run() == nil {
			actualName = testName
			break
		}
	}

	if actualName == "" {
		return fmt.Errorf("could not find created utun interface")
	}

	// Update interface name to actual utun name
	i.Name = actualName

	// Set IP address - extract IP without CIDR
	ip := i.Address
	if idx := strings.Index(ip, "/"); idx != -1 {
		ip = ip[:idx]
	}

	cmd = exec.Command("ifconfig", actualName, "inet", ip, ip, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure interface: %w, output: %s", err, string(output))
	}

	return nil
}

func (i *Interface) createWindows() error {
	// This should never be called on Unix systems
	return fmt.Errorf("Windows-specific function called on Unix system")
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
	// This should never be called on Unix systems
	return fmt.Errorf("Windows-specific function called on Unix system")
}
