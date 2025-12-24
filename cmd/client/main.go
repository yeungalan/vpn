package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vpn/wireguard-mesh/pkg/client"
	"github.com/vpn/wireguard-mesh/pkg/config"
)

func main() {
	configPath := flag.String("config", config.GetDefaultClientConfigPath(), "Path to client configuration file")
	serverAddr := flag.String("server", "", "Server address (overrides config)")
	exitNode := flag.Bool("exit-node", false, "Run as exit node (overrides config)")
	statusCmd := flag.Bool("status", false, "Show client status and exit")
	flag.Parse()

	log.Printf("WireGuard Mesh VPN Client")
	log.Printf("=========================")

	// Load configuration
	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with command-line flags
	if *serverAddr != "" {
		cfg.ServerAddr = *serverAddr
	}
	if *exitNode {
		cfg.ExitNode = true
	}

	// Create client
	c, err := client.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Handle status command
	if *statusCmd {
		status, err := c.Status()
		if err != nil {
			log.Fatalf("Failed to get status: %v", err)
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down client...")
		if err := c.Stop(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Start client
	if err := c.Start(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}
