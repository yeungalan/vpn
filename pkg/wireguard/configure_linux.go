//go:build linux

package wireguard

import (
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Configure configures the WireGuard interface on Linux
func (i *Interface) Configure() error {
	privateKey, err := wgtypes.ParseKey(i.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	port := i.ListenPort
	config := wgtypes.Config{
		PrivateKey: &privateKey,
		ListenPort: &port,
	}

	if err := i.client.ConfigureDevice(i.Name, config); err != nil {
		return fmt.Errorf("failed to configure device: %w", err)
	}

	return nil
}
