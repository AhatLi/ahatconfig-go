package main

import (
	"fmt"
	"log"
	"os"

	AhatGoKit "github.com/AhatLi/ahatconfig-go"
)

// EnvConfig demonstrates environment variable configuration
type EnvConfig struct {
	Database struct {
		Host     string `env:"HOST" required:"true"`
		Port     int    `env:"PORT" default:"5432"`
		Name     string `env:"NAME" required:"true"`
		User     string `env:"USER" required:"true"`
		Password string `env:"PASSWORD" secret:"true" required:"true"`
		SSLMode  string `env:"SSL_MODE" default:"require"`
	} `env:"DATABASE"`

	Redis struct {
		Host     string `env:"HOST" default:"localhost"`
		Port     int    `env:"PORT" default:"6379"`
		Password string `env:"PASSWORD" secret:"true"`
		DB       int    `env:"DB" default:"0"`
	} `env:"REDIS"`

	App struct {
		Name    string `env:"NAME" required:"true"`
		Version string `env:"VERSION" default:"1.0.0"`
		Debug   bool   `env:"DEBUG" default:"false"`
	} `env:"APP"`
}

func main() {
	// Set environment variables for demonstration
	setupEnvVars()

	// Set config type to environment variables
	os.Setenv("ENVAPP_CONFIG_TYPE", "env")

	// Initialize configuration
	err := AhatGoKit.InitConfigSafe[EnvConfig]("envapp")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Get configuration
	cfg, err := AhatGoKit.GetConfigSafe[EnvConfig]()
	if err != nil {
		log.Fatal("Failed to get config:", err)
	}

	// Display configuration
	fmt.Printf("📱 App: %s v%s (debug: %t)\n",
		cfg.App.Name, cfg.App.Version, cfg.App.Debug)

	fmt.Printf("🗄️  Database: %s@%s:%d/%s (SSL: %s)\n",
		cfg.Database.User, cfg.Database.Host, cfg.Database.Port,
		cfg.Database.Name, cfg.Database.SSLMode)

	fmt.Printf("🔴 Redis: %s:%d (DB: %d)\n",
		cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)

	// Print configuration (with secret masking)
	fmt.Println("\n📋 Configuration from Environment Variables:")
	AhatGoKit.PrintConfig()
}

func setupEnvVars() {
	// Database configuration
	os.Setenv("ENVAPP_DATABASE_HOST", "prod-db.example.com")
	os.Setenv("ENVAPP_DATABASE_PORT", "5432")
	os.Setenv("ENVAPP_DATABASE_NAME", "myapp_prod")
	os.Setenv("ENVAPP_DATABASE_USER", "prod_user")
	os.Setenv("ENVAPP_DATABASE_PASSWORD", "super-secret-password")
	os.Setenv("ENVAPP_DATABASE_SSL_MODE", "require")

	// Redis configuration
	os.Setenv("ENVAPP_REDIS_HOST", "redis-cluster.example.com")
	os.Setenv("ENVAPP_REDIS_PORT", "6379")
	os.Setenv("ENVAPP_REDIS_PASSWORD", "redis-secret")
	os.Setenv("ENVAPP_REDIS_DB", "1")

	// App configuration
	os.Setenv("ENVAPP_APP_NAME", "Production App")
	os.Setenv("ENVAPP_APP_VERSION", "2.0.0")
	os.Setenv("ENVAPP_APP_DEBUG", "false")
}
