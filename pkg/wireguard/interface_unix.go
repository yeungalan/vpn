//go:build linux || darwin

package wireguard

import (
	"fmt"
	"os"
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
	// On macOS, interface names must be utun[0-9]*
	// Let the system pick the next available utun interface
	utunName := "utun"

	// Start wireguard-go in the background
	cmd := exec.Command("wireguard-go", utunName)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start wireguard-go: %w", err)
	}

	// Store the process so we can kill it later
	i.process = cmd

	// Wait a moment for the interface to be created
	time.Sleep(500 * time.Millisecond)

	// Find the created utun interface
	actualName := ""
	for j := 0; j < 10; j++ {
		testName := fmt.Sprintf("utun%d", j)
		testCmd := exec.Command("ifconfig", testName)
		if testCmd.Run() == nil {
			actualName = testName
			break
		}
	}

	if actualName == "" {
		cmd.Process.Kill()
		return fmt.Errorf("could not find created utun interface")
	}

	// Update interface name to actual utun name
	i.Name = actualName

	// Wait for the UAPI socket to be created
	socketPath := fmt.Sprintf("/var/run/wireguard/%s.sock", actualName)
	for retry := 0; retry < 10; retry++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Set IP address - extract IP without CIDR
	ip := i.Address
	if idx := strings.Index(ip, "/"); idx != -1 {
		ip = ip[:idx]
	}

	ipCmd := exec.Command("ifconfig", actualName, "inet", ip, ip, "up")
	if output, err := ipCmd.CombinedOutput(); err != nil {
		cmd.Process.Kill()
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
	// Kill the wireguard-go process if we have a reference to it
	if i.process != nil {
		if cmd, ok := i.process.(*exec.Cmd); ok && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait() // Clean up zombie process
		}
	}

	// Also try to kill by name in case we lost the reference
	killCmd := exec.Command("pkill", "-f", "wireguard-go.*"+i.Name)
	_ = killCmd.Run() // Ignore errors as process might not exist

	return nil
}

func (i *Interface) destroyWindows() error {
	// This should never be called on Unix systems
	return fmt.Errorf("Windows-specific function called on Unix system")
}
