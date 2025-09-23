package ahatconfig

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

// 중첩된 구조체 슬라이스 테스트용 구조체
type NestedStructSliceConfig struct {
	Services []struct {
		Name   string `toml:"name" env:"NAME"`
		Config struct {
			Host     string `toml:"host" env:"HOST"`
			Port     int    `toml:"port" env:"PORT"`
			Settings struct {
				Timeout int    `toml:"timeout" env:"TIMEOUT" default:"30"`
				Debug   bool   `toml:"debug" env:"DEBUG" default:"false"`
				LogFile string `toml:"log_file" env:"LOG_FILE"`
			} `toml:"settings" env:"SETTINGS"`
		} `toml:"config" env:"CONFIG"`
	} `toml:"services" env:"SERVICES"`
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
		expectedHosts := []string{"db1.example.com", "db2.example.com"}
		if !reflect.DeepEqual(cfg.Database.Hosts, expectedHosts) {
			t.Errorf("expected db hosts to be %v, got %v", expectedHosts, cfg.Database.Hosts)
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

	t.Run("Load from Environment Variables Only", func(t *testing.T) {
		resetGlobalConfig()
		AppName = "TESTAPP"
		// No TOML file, only environment variables
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
		expectedEnvHosts := []string{"envdb1", "envdb2"}
		if !reflect.DeepEqual(cfg.Database.Hosts, expectedEnvHosts) {
			t.Errorf("expected db hosts to be %v, got %v", expectedEnvHosts, cfg.Database.Hosts)
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

	t.Run("Default values in environment variables", func(t *testing.T) {
		resetGlobalConfig()
		AppName = "TESTAPP"
		t.Setenv("TESTAPP_SERVER_HOST", "envhost")
		// TESTAPP_SERVER_PORT is not set, should use default value 8080
		t.Setenv("TESTAPP_DATABASE_USER", "envuser")

		err := LoadConfig[TestConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		cfg := GetConfig[TestConfig]()

		if cfg.Server.Host != "envhost" {
			t.Errorf("expected server host to be 'envhost', got '%s'", cfg.Server.Host)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("expected server port to use default value 8080, got %d", cfg.Server.Port)
		}
		if cfg.Database.User != "envuser" {
			t.Errorf("expected db user to be 'envuser', got '%s'", cfg.Database.User)
		}
	})

	t.Run("Default values in struct slice environment variables", func(t *testing.T) {
		// Test struct with default values in slice elements
		type TestSliceConfig struct {
			Users []struct {
				Name string `env:"NAME" required:"true" default:"Anonymous"`
				Role string `env:"ROLE" required:"true" default:"user"`
			} `env:"USERS"`
		}

		resetGlobalConfig()
		AppName = "TESTSLICE"
		// Set only one field for the first user, others should use defaults
		t.Setenv("TESTSLICE_USERS_0_NAME", "Alice")
		// TESTSLICE_USERS_0_ROLE is not set, should use default "user"
		t.Setenv("TESTSLICE_USERS_1_NAME", "Bob")
		// TESTSLICE_USERS_1_ROLE is not set, should use default "user"

		err := LoadConfig[TestSliceConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		cfg := GetConfig[TestSliceConfig]()

		if len(cfg.Users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(cfg.Users))
		}
		if cfg.Users[0].Name != "Alice" {
			t.Errorf("expected user 0 name to be 'Alice', got '%s'", cfg.Users[0].Name)
		}
		if cfg.Users[0].Role != "user" {
			t.Errorf("expected user 0 role to use default 'user', got '%s'", cfg.Users[0].Role)
		}
		if cfg.Users[1].Name != "Bob" {
			t.Errorf("expected user 1 name to be 'Bob', got '%s'", cfg.Users[1].Name)
		}
		if cfg.Users[1].Role != "user" {
			t.Errorf("expected user 1 role to use default 'user', got '%s'", cfg.Users[1].Role)
		}
	})

	t.Run("Hybrid loading: TOML + Environment Variables", func(t *testing.T) {
		resetGlobalConfig()
		appName := "hybridapp"
		tomlContent := `
enabled = true
[server]
host = "tomlhost"
port = 8000

[database]
user = "tomluser"
password = "tomlpassword"
hosts = ["tomldb1.example.com", "tomldb2.example.com"]

[[users]]
name = "TomlAlice"
role = "toml_admin"

[[users]]
name = "TomlBob"
role = "toml_user"
`
		_, cleanup := createTestTomlFile(t, appName, tomlContent)
		defer cleanup()

		AppName = appName

		// Set some environment variables to override TOML values
		t.Setenv("HYBRIDAPP_SERVER_HOST", "envhost") // Override TOML
		// HYBRIDAPP_SERVER_PORT not set, should keep TOML value 8000
		t.Setenv("HYBRIDAPP_DATABASE_PASSWORD", "envpassword") // Override TOML
		// HYBRIDAPP_DATABASE_USER not set, should keep TOML value "tomluser"
		t.Setenv("HYBRIDAPP_USERS_0_NAME", "EnvAlice")  // Override TOML
		t.Setenv("HYBRIDAPP_USERS_0_ROLE", "env_admin") // Override TOML
		t.Setenv("HYBRIDAPP_USERS_1_NAME", "EnvBob")    // Override TOML
		t.Setenv("HYBRIDAPP_USERS_1_ROLE", "env_user")  // Override TOML

		err := LoadConfig[TestConfig]()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		cfg := GetConfig[TestConfig]()

		// Environment variables should override TOML values
		if cfg.Server.Host != "envhost" {
			t.Errorf("expected server host to be 'envhost' (from env), got '%s'", cfg.Server.Host)
		}
		if cfg.Server.Port != 8000 {
			t.Errorf("expected server port to be 8000 (from TOML), got %d", cfg.Server.Port)
		}
		if cfg.Database.User != "tomluser" {
			t.Errorf("expected db user to be 'tomluser' (from TOML), got '%s'", cfg.Database.User)
		}
		if cfg.Database.Password != "envpassword" {
			t.Errorf("expected db password to be 'envpassword' (from env), got '%s'", cfg.Database.Password)
		}

		// TOML values should be preserved where env vars are not set
		expectedHosts := []string{"tomldb1.example.com", "tomldb2.example.com"}
		if !reflect.DeepEqual(cfg.Database.Hosts, expectedHosts) {
			t.Errorf("expected db hosts to be %v (from TOML), got %v", expectedHosts, cfg.Database.Hosts)
		}

		if len(cfg.Users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(cfg.Users))
		}
		if cfg.Users[0].Name != "EnvAlice" {
			t.Errorf("expected user 0 name to be 'EnvAlice' (from env), got '%s'", cfg.Users[0].Name)
		}
		if cfg.Users[0].Role != "env_admin" {
			t.Errorf("expected user 0 role to be 'env_admin' (from env), got '%s'", cfg.Users[0].Role)
		}
		if cfg.Users[1].Name != "EnvBob" {
			t.Errorf("expected user 1 name to be 'EnvBob' (from env), got '%s'", cfg.Users[1].Name)
		}
		if cfg.Users[1].Role != "env_user" {
			t.Errorf("expected user 1 role to be 'env_user' (from env), got '%s'", cfg.Users[1].Role)
		}
		if !cfg.Enabled {
			t.Errorf("expected enabled to be true (from TOML), got false")
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

// TestLoadStructSliceEnvNestedStructs는 중첩된 구조체 슬라이스의 환경변수 로딩을 테스트합니다
func TestLoadStructSliceEnvNestedStructs(t *testing.T) {
	// 환경변수 설정
	envVars := map[string]string{
		"SERVICES_0_NAME":                     "web-service",
		"SERVICES_0_CONFIG_HOST":              "localhost",
		"SERVICES_0_CONFIG_PORT":              "8080",
		"SERVICES_0_CONFIG_SETTINGS_TIMEOUT":  "60",
		"SERVICES_0_CONFIG_SETTINGS_DEBUG":    "true",
		"SERVICES_0_CONFIG_SETTINGS_LOG_FILE": "/var/log/web.log",
		"SERVICES_1_NAME":                     "api-service",
		"SERVICES_1_CONFIG_HOST":              "api.example.com",
		"SERVICES_1_CONFIG_PORT":              "9090",
		"SERVICES_1_CONFIG_SETTINGS_TIMEOUT":  "120",
		"SERVICES_1_CONFIG_SETTINGS_DEBUG":    "false",
		"SERVICES_1_CONFIG_SETTINGS_LOG_FILE": "/var/log/api.log",
	}

	// 환경변수 설정
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// 테스트 후 환경변수 정리
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// 구조체 타입 정보 가져오기
	configType := reflect.TypeOf(NestedStructSliceConfig{})
	servicesField, _ := configType.FieldByName("Services")
	servicesType := servicesField.Type.Elem() // 슬라이스 요소 타입

	// loadStructSliceEnv 함수 직접 테스트
	result, err := loadStructSliceEnv("SERVICES", servicesType)
	if err != nil {
		t.Fatalf("loadStructSliceEnv failed: %v", err)
	}

	// 결과 검증
	if len(result) != 2 {
		t.Fatalf("expected 2 services, got %d", len(result))
	}

	// 첫 번째 서비스 검증
	service0 := result[0]
	name0 := service0.FieldByName("Name").String()
	if name0 != "web-service" {
		t.Errorf("expected service 0 name to be 'web-service', got '%s'", name0)
	}

	config0 := service0.FieldByName("Config")
	host0 := config0.FieldByName("Host").String()
	if host0 != "localhost" {
		t.Errorf("expected service 0 host to be 'localhost', got '%s'", host0)
	}

	port0 := config0.FieldByName("Port").Int()
	if port0 != 8080 {
		t.Errorf("expected service 0 port to be 8080, got %d", port0)
	}

	settings0 := config0.FieldByName("Settings")
	timeout0 := settings0.FieldByName("Timeout").Int()
	if timeout0 != 60 {
		t.Errorf("expected service 0 timeout to be 60, got %d", timeout0)
	}

	debug0 := settings0.FieldByName("Debug").Bool()
	if debug0 != true {
		t.Errorf("expected service 0 debug to be true, got %v", debug0)
	}

	logFile0 := settings0.FieldByName("LogFile").String()
	if logFile0 != "/var/log/web.log" {
		t.Errorf("expected service 0 log file to be '/var/log/web.log', got '%s'", logFile0)
	}

	// 두 번째 서비스 검증
	service1 := result[1]
	name1 := service1.FieldByName("Name").String()
	if name1 != "api-service" {
		t.Errorf("expected service 1 name to be 'api-service', got '%s'", name1)
	}

	config1 := service1.FieldByName("Config")
	host1 := config1.FieldByName("Host").String()
	if host1 != "api.example.com" {
		t.Errorf("expected service 1 host to be 'api.example.com', got '%s'", host1)
	}

	port1 := config1.FieldByName("Port").Int()
	if port1 != 9090 {
		t.Errorf("expected service 1 port to be 9090, got %d", port1)
	}

	settings1 := config1.FieldByName("Settings")
	timeout1 := settings1.FieldByName("Timeout").Int()
	if timeout1 != 120 {
		t.Errorf("expected service 1 timeout to be 120, got %d", timeout1)
	}

	debug1 := settings1.FieldByName("Debug").Bool()
	if debug1 != false {
		t.Errorf("expected service 1 debug to be false, got %v", debug1)
	}

	logFile1 := settings1.FieldByName("LogFile").String()
	if logFile1 != "/var/log/api.log" {
		t.Errorf("expected service 1 log file to be '/var/log/api.log', got '%s'", logFile1)
	}
}

// TestLoadStructSliceEnvNestedStructsWithDefaults는 중첩된 구조체 슬라이스에서 기본값이 적용되는지 테스트합니다
func TestLoadStructSliceEnvNestedStructsWithDefaults(t *testing.T) {
	// 환경변수 설정 (일부만 설정하고 기본값 테스트)
	envVars := map[string]string{
		"SERVICES_0_NAME":        "web-service",
		"SERVICES_0_CONFIG_HOST": "localhost",
		"SERVICES_0_CONFIG_PORT": "8080",
		// TIMEOUT과 DEBUG는 기본값 사용
		"SERVICES_0_CONFIG_SETTINGS_LOG_FILE": "/var/log/web.log",
	}

	// 환경변수 설정
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// 테스트 후 환경변수 정리
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// 구조체 타입 정보 가져오기
	configType := reflect.TypeOf(NestedStructSliceConfig{})
	servicesField, _ := configType.FieldByName("Services")
	servicesType := servicesField.Type.Elem() // 슬라이스 요소 타입

	// loadStructSliceEnv 함수 직접 테스트
	result, err := loadStructSliceEnv("SERVICES", servicesType)
	if err != nil {
		t.Fatalf("loadStructSliceEnv failed: %v", err)
	}

	// 결과 검증
	if len(result) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result))
	}

	// 서비스 검증
	service0 := result[0]
	config0 := service0.FieldByName("Config")
	settings0 := config0.FieldByName("Settings")

	// 기본값 검증
	timeout0 := settings0.FieldByName("Timeout").Int()
	if timeout0 != 30 { // default:"30"
		t.Errorf("expected service 0 timeout to be 30 (default), got %d", timeout0)
	}

	debug0 := settings0.FieldByName("Debug").Bool()
	if debug0 != false { // default:"false"
		t.Errorf("expected service 0 debug to be false (default), got %v", debug0)
	}
}
