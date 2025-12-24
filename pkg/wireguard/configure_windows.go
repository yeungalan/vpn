// +build windows

package wireguard

// Configure configures the WireGuard interface on Windows
// On Windows with in-process device, configuration is done during Create()
// so this is a no-op
func (i *Interface) Configure() error {
	// Already configured during createWindows() via IpcSet
	// No need to use wgctrl here as it expects an external process
	return nil
}
