package main

import (
	"fmt"
	"log"

	AhatGoKit "github.com/AhatLi/ahatconfig-go"
)

// BasicConfig demonstrates basic configuration structure
type BasicConfig struct {
	Server struct {
		Host string `toml:"host" env:"HOST" required:"true"`
		Port int    `toml:"port" env:"PORT" default:"8080"`
	} `toml:"server"`

	Database struct {
		User     string `toml:"user" env:"USER" required:"true"`
		Password string `toml:"password" env:"PASSWORD" secret:"true" required:"true"`
		Host     string `toml:"host" env:"HOST" default:"localhost"`
		Port     int    `toml:"port" env:"PORT" default:"5432"`
	} `toml:"database"`

	Features struct {
		Enabled bool `toml:"enabled" env:"ENABLED" default:"true"`
	} `toml:"features"`
}

func main() {
	// Initialize configuration
	err := AhatGoKit.InitConfigSafe[BasicConfig]("basicapp")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Get configuration
	cfg, err := AhatGoKit.GetConfigSafe[BasicConfig]()
	if err != nil {
		log.Fatal("Failed to get config:", err)
	}

	// Use configuration
	fmt.Printf("🚀 Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("🗄️  Database: %s@%s:%d\n", cfg.Database.User, cfg.Database.Host, cfg.Database.Port)
	fmt.Printf("✨ Features enabled: %t\n", cfg.Features.Enabled)

	// Print configuration (with secret masking)
	fmt.Println("\n📋 Configuration:")
	AhatGoKit.PrintConfig()
}
