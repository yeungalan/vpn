// +build linux darwin

package wireguard

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
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
