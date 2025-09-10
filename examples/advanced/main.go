package main

import (
	"fmt"
	"log"

	AhatGoKit "github.com/AhatLi/ahatconfig-go"
)

// AdvancedConfig demonstrates advanced features like slices and nested structures
type AdvancedConfig struct {
	Service struct {
		Name    string `toml:"name" env:"NAME" required:"true"`
		Version string `toml:"version" env:"VERSION" default:"1.0.0"`
		Port    int    `toml:"port" env:"PORT" default:"8080"`
	} `toml:"service"`

	API struct {
		Version   string `toml:"version" env:"VERSION" default:"v1"`
		RateLimit struct {
			Requests int `toml:"requests" env:"REQUESTS" default:"100"`
			Window   int `toml:"window" env:"WINDOW" default:"60"`
		} `toml:"rate_limit"`
	} `toml:"api"`

	// Slice of servers
	Servers []struct {
		Name string `toml:"name" env:"NAME"`
		URL  string `toml:"url" env:"URL"`
		Port int    `toml:"port" env:"PORT"`
	} `toml:"servers" env:"SERVERS"`

	// Mixed type slices
	Ports    []int    `toml:"ports" env:"PORTS"`
	Features []string `toml:"features" env:"FEATURES"`
	Flags    []bool   `toml:"flags" env:"FLAGS"`

	Monitoring struct {
		Enabled bool   `toml:"enabled" env:"ENABLED" default:"true"`
		Metrics string `toml:"metrics" env:"METRICS" default:"/metrics"`
		Health  string `toml:"health" env:"HEALTH" default:"/health"`
	} `toml:"monitoring"`

	Security struct {
		JWTSecret string `toml:"jwt_secret" env:"JWT_SECRET" secret:"true" required:"true"`
		APIKey    string `toml:"api_key" env:"API_KEY" secret:"true"`
	} `toml:"security"`
}

func main() {
	// Initialize configuration
	err := AhatGoKit.InitConfigSafe[AdvancedConfig]("advancedapp")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Get configuration
	cfg, err := AhatGoKit.GetConfigSafe[AdvancedConfig]()
	if err != nil {
		log.Fatal("Failed to get config:", err)
	}

	// Display configuration
	fmt.Printf("🔧 Service: %s v%s on port %d\n",
		cfg.Service.Name, cfg.Service.Version, cfg.Service.Port)

	fmt.Printf("📡 API: %s with rate limit %d requests per %d seconds\n",
		cfg.API.Version, cfg.API.RateLimit.Requests, cfg.API.RateLimit.Window)

	fmt.Printf("🖥️  Servers (%d):\n", len(cfg.Servers))
	for i, server := range cfg.Servers {
		fmt.Printf("  %d. %s - %s:%d\n", i+1, server.Name, server.URL, server.Port)
	}

	fmt.Printf("🔌 Ports: %v\n", cfg.Ports)
	fmt.Printf("✨ Features: %v\n", cfg.Features)
	fmt.Printf("🚩 Flags: %v\n", cfg.Flags)

	fmt.Printf("📊 Monitoring: %t (metrics: %s, health: %s)\n",
		cfg.Monitoring.Enabled, cfg.Monitoring.Metrics, cfg.Monitoring.Health)

	fmt.Printf("🔐 Security: JWT configured, API Key: %s\n",
		func() string {
			if cfg.Security.APIKey != "" {
				return "configured"
			}
			return "not set"
		}())

	// Print configuration (with secret masking)
	fmt.Println("\n📋 Full Configuration:")
	AhatGoKit.PrintConfig()
}
