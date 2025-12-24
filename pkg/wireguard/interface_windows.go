// +build windows

package wireguard

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	runningDevices = make(map[string]*device.Device)
)

func (i *Interface) createWindows() error {
	// Create TUN device using wintun (embedded in wireguard-go)
	tunDevice, err := tun.CreateTUN(i.Name, device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("failed to create TUN device: %w", err)
	}

	// Get the real interface name (wintun may change it)
	realName, err := tunDevice.Name()
	if err != nil {
		tunDevice.Close()
		return fmt.Errorf("failed to get interface name: %w", err)
	}

	log.Printf("Created TUN interface: %s", realName)

	// Create WireGuard device
	logger := device.NewLogger(
		device.LogLevelError,
		fmt.Sprintf("[%s] ", realName),
	)

	wgDevice := device.NewDevice(tunDevice, logger)

	// Configure the device with our private key and listen port
	// The IPC format is what wg(8) uses
	config := fmt.Sprintf(`private_key=%s
listen_port=%d
`, i.PrivateKey, i.ListenPort)

	if err := wgDevice.IpcSet(config); err != nil {
		wgDevice.Close()
		return fmt.Errorf("failed to configure device: %w", err)
	}

	// Bring the device up
	wgDevice.Up()

	// Store the device so we can close it later
	runningDevices[i.Name] = wgDevice

	// Wait a moment for interface to be ready
	time.Sleep(500 * time.Millisecond)

	// Set IP address using netsh
	ip := strings.Split(i.Address, "/")[0]

	// Find the actual interface name Windows uses
	cmd := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", realName), "static", ip, "255.255.255.255")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try with quotes around the interface name
		cmd = exec.Command("netsh", "interface", "ip", "set", "address",
			fmt.Sprintf(`name="%s"`, realName), "static", ip, "255.255.255.255")
		if output2, err2 := cmd.CombinedOutput(); err2 != nil {
			log.Printf("Warning: failed to set IP address: %v, output: %s; retry: %v, output: %s",
				err, string(output), err2, string(output2))
		}
	}

	// Bring interface up
	cmd = exec.Command("netsh", "interface", "set", "interface", realName, "admin=enabled")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: failed to enable interface: %v, output: %s", err, string(output))
	}

	log.Printf("Windows WireGuard interface %s configured with IP %s", realName, ip)

	return nil
}

func (i *Interface) destroyWindows() error {
	// Close the device if we have it
	if wgDevice, ok := runningDevices[i.Name]; ok {
		wgDevice.Close()
		delete(runningDevices, i.Name)
		log.Printf("Closed WireGuard device: %s", i.Name)
	}

	return nil
}
