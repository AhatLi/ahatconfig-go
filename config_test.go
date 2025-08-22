package AhatGoKit

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// Test struct for configuration
type TestConfig struct {
	Server struct {
		Host string `toml:"host" env:"HOST" required:"true"`
		Port int    `toml:"port" env:"PORT" default:"8080"`
	} `toml:"server" env:"SERVER"`
	Database struct {
		User     string   `toml:"user" env:"USER" required:"true"`
		Password string   `toml:"password" env:"PASSWORD" secret:"true"`
		Hosts    []string `toml:"hosts" env:"HOSTS"`
	} `toml:"database" env:"DATABASE"`
	Users []struct {
		Name string `toml:"name" env:"NAME"`
		Role string `toml:"role" env:"ROLE"`
	} `toml:"users" env:"USERS"`
	Enabled bool `toml:"enabled" env:"ENABLED"`
}

func resetGlobalConfig() {
	instance = nil
	once = sync.Once{}
	AppName = ""
	configPath = ""
}

func createTestTomlFile(t *testing.T, appName, content string) (string, func()) {
	t.Helper()
	// Use a temporary directory to avoid polluting the current directory
	// And also to make tests runnable in parallel in the future.
	dir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create a dummy executable file to make os.Executable() work in test
	exePath := filepath.Join(dir, "testapp.exe")
	if _, err := os.Create(exePath); err != nil {
		t.Fatalf("failed to create dummy executable: %v", err)
	}
	// We need to override os.Executable to return our dummy executable path
	// but that is not straightforward. A simpler way is to set configPath directly.

	filePath := filepath.Join(dir, appName+".toml")
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test toml file: %v", err)
	}

	// This is the path to the dummy executable, not the toml file.
	// The code calculates the toml file path from the executable path.
	configPath = exePath

	return filePath, func() {
		os.RemoveAll(dir)
		configPath = "" // Clean up global state
	}
}

func TestConfigLoading(t *testing.T) {
	t.Run("Load from TOML file", func(t *testing.T) {
		resetGlobalConfig()
		appName := "testapp"
		tomlContent := `
enabled = true
[server]
host = "localhost"
port = 8000

[database]
user = "testuser"
password = "testpassword"
hosts = ["db1.example.com", "db2.example.com"]

[[users]]
name = "Alice"
role = "admin"

[[users]]
name = "Bob"
role = "user"
`
		_, cleanup := createTestTomlFile(t, appName, tomlContent)
		defer cleanup()

		AppName = appName
		err := LoadConfig[TestConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		cfg := GetConfig[TestConfig]()

		if cfg.Server.Host != "localhost" {
			t.Errorf("expected server host to be 'localhost', got '%s'", cfg.Server.Host)
		}
		if cfg.Server.Port != 8000 {
			t.Errorf("expected server port to be 8000, got %d", cfg.Server.Port)
		}
		if cfg.Database.User != "testuser" {
			t.Errorf("expected db user to be 'testuser', got '%s'", cfg.Database.User)
		}
		if !reflect.DeepEqual(cfg.Database.Hosts, []string{"db1.example.com", "db2.example.com"}) {
			t.Errorf("expected db hosts to be '[db1.example.com db2.example.com]', got '%v'", cfg.Database.Hosts)
		}
		if len(cfg.Users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(cfg.Users))
		}
		if cfg.Users[0].Name != "Alice" || cfg.Users[0].Role != "admin" {
			t.Errorf("unexpected user data for user 0: %+v", cfg.Users[0])
		}
		if cfg.Users[1].Name != "Bob" || cfg.Users[1].Role != "user" {
			t.Errorf("unexpected user data for user 1: %+v", cfg.Users[1])
		}
		if !cfg.Enabled {
			t.Errorf("expected enabled to be true, got false")
		}
	})

	t.Run("Load from Environment Variables", func(t *testing.T) {
		resetGlobalConfig()
		AppName = "TESTAPP"
		t.Setenv("TESTAPP_CONFIG_TYPE", "env")
		t.Setenv("TESTAPP_SERVER_HOST", "envhost")
		t.Setenv("TESTAPP_SERVER_PORT", "9090")
		t.Setenv("TESTAPP_DATABASE_USER", "envuser")
		t.Setenv("TESTAPP_DATABASE_PASSWORD", "envpass")
		t.Setenv("TESTAPP_DATABASE_HOSTS", "envdb1,envdb2")
		t.Setenv("TESTAPP_ENABLED", "true")
		t.Setenv("TESTAPP_USERS_0_NAME", "EnvAlice")
		t.Setenv("TESTAPP_USERS_0_ROLE", "env_admin")
		t.Setenv("TESTAPP_USERS_1_NAME", "EnvBob")
		t.Setenv("TESTAPP_USERS_1_ROLE", "env_user")

		err := LoadConfig[TestConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		cfg := GetConfig[TestConfig]()

		if cfg.Server.Host != "envhost" {
			t.Errorf("expected server host to be 'envhost', got '%s'", cfg.Server.Host)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("expected server port to be 9090, got %d", cfg.Server.Port)
		}
		if cfg.Database.User != "envuser" {
			t.Errorf("expected db user to be 'envuser', got '%s'", cfg.Database.User)
		}
		if !reflect.DeepEqual(cfg.Database.Hosts, []string{"envdb1", "envdb2"}) {
			t.Errorf("expected db hosts to be '[envdb1 envdb2]', got '%v'", cfg.Database.Hosts)
		}
		if len(cfg.Users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(cfg.Users))
		}
		if cfg.Users[0].Name != "EnvAlice" || cfg.Users[0].Role != "env_admin" {
			t.Errorf("unexpected user data for user 0: %+v", cfg.Users[0])
		}
		if cfg.Users[1].Name != "EnvBob" || cfg.Users[1].Role != "env_user" {
			t.Errorf("unexpected user data for user 1: %+v", cfg.Users[1])
		}
		if !cfg.Enabled {
			t.Errorf("expected enabled to be true, got false")
		}
	})

	t.Run("Required field missing from TOML", func(t *testing.T) {
		resetGlobalConfig()
		appName := "testapp"
		// Missing server.host
		tomlContent := `
[server]
port = 8000
[database]
user = "testuser"
`
		_, cleanup := createTestTomlFile(t, appName, tomlContent)
		defer cleanup()

		AppName = appName
		err := LoadConfig[TestConfig]()
		if err == nil {
			t.Fatal("expected an error for missing required field, but got nil")
		}
		expectedError := "required field 'HOST' is missing or empty"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("expected error to contain '%s', got '%v'", expectedError, err)
		}
	})

	t.Run("Required field missing from Env", func(t *testing.T) {
		resetGlobalConfig()
		AppName = "TESTAPP"
		t.Setenv("TESTAPP_CONFIG_TYPE", "env")
		// Missing TESTAPP_SERVER_HOST
		t.Setenv("TESTAPP_SERVER_PORT", "9090")
		t.Setenv("TESTAPP_DATABASE_USER", "envuser")

		err := LoadConfig[TestConfig]()
		if err == nil {
			t.Fatal("expected an error for missing required field, but got nil")
		}
		// The error comes from checkRequiredField, which uses the 'env' tag
		expectedError := "required field 'HOST' is missing or empty"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("expected error to contain '%s', got '%v'", expectedError, err)
		}
	})

	t.Run("PrintConfig with secret masking", func(t *testing.T) {
		resetGlobalConfig()
		appName := "testapp"
		tomlContent := `
[server]
host = "localhost"
[database]
user = "testuser"
password = "testpassword"
`
		_, cleanup := createTestTomlFile(t, appName, tomlContent)
		defer cleanup()

		AppName = appName
		err := LoadConfig[TestConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Redirect stdout to capture PrintConfig output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		PrintConfig()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)

		output := buf.String()
		if !strings.Contains(output, `"Password": "****"`) {
			t.Errorf("expected password to be masked, but it was not. Output:\n%s", output)
		}
		if strings.Contains(output, "testpassword") {
			t.Errorf("expected password to not be visible, but it was. Output:\n%s", output)
		}
	})
}
