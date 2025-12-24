package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vpn/wireguard-mesh/pkg/config"
	"github.com/vpn/wireguard-mesh/pkg/server"
)

func main() {
	configPath := flag.String("config", config.GetDefaultServerConfigPath(), "Path to server configuration file")
	listenAddr := flag.String("listen", "", "Server listen address (overrides config)")
	networkCIDR := flag.String("network", "", "VPN network CIDR (overrides config)")
	flag.Parse()

	log.Printf("WireGuard Mesh VPN Server")
	log.Printf("=========================")

	// Load configuration
	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with command-line flags
	if *listenAddr != "" {
		cfg.ListenAddr = *listenAddr
	}
	if *networkCIDR != "" {
		cfg.NetworkCIDR = *networkCIDR
	}

	// Create server
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		os.Exit(0)
	}()

	// Start server
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
