package main

import (
	"fmt"
	"log"

	ahatconfig "github.com/AhatLi/ahatconfig-go"
)

// ErrorConfig demonstrates error handling with missing required fields
type ErrorConfig struct {
	Server struct {
		Host string `toml:"host" env:"HOST" required:"true"`
		Port int    `toml:"port" env:"PORT" default:"8080"`
	} `toml:"server"`

	Database struct {
		User     string `toml:"user" env:"USER" required:"true"`
		Password string `toml:"password" env:"PASSWORD" secret:"true" required:"true"`
	} `toml:"database"`
}

func main() {
	fmt.Println("🔍 Error Handling Examples")
	fmt.Println("==========================")

	// Example 1: Safe initialization with error handling
	fmt.Println("\n1. Safe initialization:")
	err := ahatconfig.InitConfigSafe[ErrorConfig]("errorapp")
	if err != nil {
		fmt.Printf("❌ Configuration error: %v\n", err)
		fmt.Println("   This is expected because required fields are missing")
	} else {
		fmt.Println("✅ Configuration loaded successfully")
	}

	// Example 2: Safe config retrieval
	fmt.Println("\n2. Safe config retrieval:")
	cfg, err := ahatconfig.GetConfigSafe[ErrorConfig]()
	if err != nil {
		fmt.Printf("❌ Config retrieval error: %v\n", err)
		fmt.Println("   This is expected because config was not initialized")
	} else {
		fmt.Printf("✅ Config retrieved: %+v\n", cfg)
	}

	// Example 3: Panic-based API (commented out to avoid actual panic)
	fmt.Println("\n3. Panic-based API:")
	fmt.Println("   // This would panic if called:")
	fmt.Println("   // ahatconfig.InitConfig[ErrorConfig](\"errorapp\")")
	fmt.Println("   // cfg := ahatconfig.GetConfig[ErrorConfig]()")
	fmt.Println("   // Use InitConfigSafe and GetConfigSafe for production!")

	// Example 4: Demonstrating proper error handling pattern
	fmt.Println("\n4. Proper error handling pattern:")
	demonstrateProperErrorHandling()
}

func demonstrateProperErrorHandling() {
	// This is the recommended pattern for production applications
	err := ahatconfig.InitConfigSafe[ErrorConfig]("errorapp")
	if err != nil {
		log.Printf("Failed to initialize configuration: %v", err)
		log.Println("Please check your configuration file or environment variables")
		return
	}

	cfg, err := ahatconfig.GetConfigSafe[ErrorConfig]()
	if err != nil {
		log.Printf("Failed to retrieve configuration: %v", err)
		return
	}

	fmt.Printf("✅ Configuration loaded successfully: %+v\n", cfg)
}
