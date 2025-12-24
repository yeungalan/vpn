//go:build darwin

package wireguard

// Configure configures the WireGuard interface on macOS
// On macOS with external wireguard-go, configuration is done during Create()
// using wg command, so this is a no-op
func (i *Interface) Configure() error {
	// Already configured during createDarwin() via wg command
	return nil
}
