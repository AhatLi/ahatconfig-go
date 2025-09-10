# AhatConfig-Go

A powerful, type-safe configuration management library for Go applications that supports both TOML files and environment variables with advanced features like secret masking, field validation, and performance optimization.

## Features

- 🚀 **Type-Safe**: Uses Go generics for compile-time type safety
- 📁 **Multiple Sources**: Supports both TOML files and environment variables
- 🔒 **Secret Masking**: Automatically masks sensitive information in logs
- ✅ **Field Validation**: Required field validation with custom error messages
- 🏎️ **Performance Optimized**: Cached reflection information for better performance
- 🔧 **Flexible API**: Both panic-based and error-returning APIs available
- 🎯 **Zero Dependencies**: Only depends on `github.com/pelletier/go-toml`

## Installation

```bash
go get github.com/AhatLi/ahatconfig-go
```

## Quick Start

### 1. Define Your Configuration Structure

```go
package main

import (
    "fmt"
    "github.com/AhatLi/ahatconfig-go"
)

type AppConfig struct {
    Server struct {
        Host string `toml:"host" env:"HOST" required:"true"`
        Port int    `toml:"port" env:"PORT" default:"8080"`
    } `toml:"server"`
    
    Database struct {
        User     string   `toml:"user" env:"USER" required:"true"`
        Password string   `toml:"password" env:"PASSWORD" secret:"true"`
        Hosts    []string `toml:"hosts" env:"HOSTS"`
    } `toml:"database"`
    
    Features struct {
        Enabled bool `toml:"enabled" env:"ENABLED" default:"true"`
    } `toml:"features"`
}

func main() {
    // Initialize configuration
    AhatGoKit.InitConfig[AppConfig]("myapp")
    
    // Get configuration
    cfg := AhatGoKit.GetConfig[AppConfig]()
    
    fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
    fmt.Printf("Database User: %s\n", cfg.Database.User)
}
```

### 2. Create TOML Configuration File

Create `myapp.toml` in your executable directory:

```toml
[server]
host = "localhost"
port = 3000

[database]
user = "admin"
password = "secret123"
hosts = ["db1.example.com", "db2.example.com"]

[features]
enabled = true
```

### 3. Or Use Environment Variables

Set the configuration type to environment variables:

```bash
export MYAPP_CONFIG_TYPE=env
export MYAPP_SERVER_HOST=localhost
export MYAPP_SERVER_PORT=3000
export MYAPP_DATABASE_USER=admin
export MYAPP_DATABASE_PASSWORD=secret123
export MYAPP_DATABASE_HOSTS=db1.example.com,db2.example.com
export MYAPP_FEATURES_ENABLED=true
```

## Configuration Tags

### TOML Tags
- `toml:"field_name"` - Maps to TOML field name
- `toml:"section"` - Maps to TOML section name

### Environment Tags
- `env:"FIELD_NAME"` - Maps to environment variable name
- `required:"true"` - Field is required (validation)
- `default:"value"` - Default value if not provided
- `secret:"true"` - Masks value in logs (shows as "****")

## API Reference

### Initialization Functions

#### `InitConfig[T](appname string)`
Initializes configuration with panic on error (recommended for simple applications).

```go
AhatGoKit.InitConfig[AppConfig]("myapp")
```

#### `InitConfigSafe[T](appname string) error`
Initializes configuration and returns error instead of panicking.

```go
err := AhatGoKit.InitConfigSafe[AppConfig]("myapp")
if err != nil {
    log.Fatal(err)
}
```

#### `InitConfigWithPath[T](appname, path string)`
Initializes configuration with custom executable path.

```go
AhatGoKit.InitConfigWithPath[AppConfig]("myapp", "/custom/path")
```

#### `InitConfigWithPathSafe[T](appname, path string) error`
Safe version with custom path and error return.

### Configuration Retrieval

#### `GetConfig[T]() *T`
Gets configuration with panic on error.

```go
cfg := AhatGoKit.GetConfig[AppConfig]()
```

#### `GetConfigSafe[T]() (*T, error)`
Gets configuration and returns error instead of panicking.

```go
cfg, err := AhatGoKit.GetConfigSafe[AppConfig]()
if err != nil {
    log.Fatal(err)
}
```

### Utility Functions

#### `PrintConfig()`
Prints configuration with secret masking.

```go
AhatGoKit.PrintConfig()
// Output:
// 🔹 config:
// {
//   "Server": {
//     "Host": "localhost",
//     "Port": 3000
//   },
//   "Database": {
//     "User": "admin",
//     "Password": "****"
//   }
// }
```

## Advanced Usage

### Nested Structures

```go
type Config struct {
    API struct {
        Version string `toml:"version" env:"VERSION" default:"v1"`
        RateLimit struct {
            Requests int `toml:"requests" env:"REQUESTS" default:"100"`
            Window   int `toml:"window" env:"WINDOW" default:"60"`
        } `toml:"rate_limit"`
    } `toml:"api"`
}
```

### Slice Support

```go
type Config struct {
    Servers []struct {
        Name string `toml:"name" env:"NAME"`
        URL  string `toml:"url" env:"URL"`
    } `toml:"servers" env:"SERVERS"`
}
```

Environment variables for slices:
```bash
export MYAPP_SERVERS_0_NAME=server1
export MYAPP_SERVERS_0_URL=http://server1.com
export MYAPP_SERVERS_1_NAME=server2
export MYAPP_SERVERS_1_URL=http://server2.com
```

### Mixed Types in Slices

```go
type Config struct {
    Ports    []int     `toml:"ports" env:"PORTS"`           // "8080,9090,3000"
    Features []string  `toml:"features" env:"FEATURES"`     // "auth,cache,logging"
    Flags    []bool    `toml:"flags" env:"FLAGS"`           // "true,false,true"
}
```

## Environment Variable Naming

Environment variables follow this pattern:
```
{APPNAME}_{SECTION}_{FIELD}
```

Examples:
- `MYAPP_SERVER_HOST`
- `MYAPP_DATABASE_USER`
- `MYAPP_FEATURES_ENABLED`

For nested structures:
- `MYAPP_API_RATE_LIMIT_REQUESTS`

For slices:
- `MYAPP_SERVERS_0_NAME`
- `MYAPP_SERVERS_1_URL`

## Error Handling

### Panic-based API (Default)
```go
// Will panic if configuration fails to load
AhatGoKit.InitConfig[AppConfig]("myapp")
cfg := AhatGoKit.GetConfig[AppConfig]()
```

### Error-based API (Recommended for production)
```go
// Returns error instead of panicking
err := AhatGoKit.InitConfigSafe[AppConfig]("myapp")
if err != nil {
    log.Fatal("Failed to load config:", err)
}

cfg, err := AhatGoKit.GetConfigSafe[AppConfig]()
if err != nil {
    log.Fatal("Failed to get config:", err)
}
```

## Performance Features

- **Type Caching**: Reflection information is cached for better performance
- **Unified Parsing**: Single parsing logic for all type conversions
- **Memory Efficient**: Minimal allocations during configuration loading

## Best Practices

1. **Use Safe APIs in Production**: Use `InitConfigSafe` and `GetConfigSafe` for better error handling
2. **Mark Secrets**: Always mark sensitive fields with `secret:"true"`
3. **Validate Required Fields**: Use `required:"true"` for critical configuration
4. **Provide Defaults**: Use `default:"value"` for optional fields
5. **Environment Override**: Use environment variables for deployment-specific settings

## Examples

### Web Server Configuration
```go
type WebConfig struct {
    Server struct {
        Host         string `toml:"host" env:"HOST" default:"0.0.0.0"`
        Port         int    `toml:"port" env:"PORT" default:"8080"`
        ReadTimeout  int    `toml:"read_timeout" env:"READ_TIMEOUT" default:"30"`
        WriteTimeout int    `toml:"write_timeout" env:"WRITE_TIMEOUT" default:"30"`
    } `toml:"server"`
    
    Database struct {
        Host     string `toml:"host" env:"HOST" required:"true"`
        Port     int    `toml:"port" env:"PORT" default:"5432"`
        Name     string `toml:"name" env:"NAME" required:"true"`
        User     string `toml:"user" env:"USER" required:"true"`
        Password string `toml:"password" env:"PASSWORD" secret:"true" required:"true"`
        SSLMode  string `toml:"ssl_mode" env:"SSL_MODE" default:"require"`
    } `toml:"database"`
    
    Redis struct {
        Host     string `toml:"host" env:"HOST" default:"localhost"`
        Port     int    `toml:"port" env:"PORT" default:"6379"`
        Password string `toml:"password" env:"PASSWORD" secret:"true"`
        DB       int    `toml:"db" env:"DB" default:"0"`
    } `toml:"redis"`
}
```

### Microservice Configuration
```go
type ServiceConfig struct {
    Service struct {
        Name    string `toml:"name" env:"NAME" required:"true"`
        Version string `toml:"version" env:"VERSION" default:"1.0.0"`
        Port    int    `toml:"port" env:"PORT" default:"8080"`
    } `toml:"service"`
    
    Dependencies []struct {
        Name string `toml:"name" env:"NAME"`
        URL  string `toml:"url" env:"URL"`
    } `toml:"dependencies" env:"DEPENDENCIES"`
    
    Monitoring struct {
        Enabled bool   `toml:"enabled" env:"ENABLED" default:"true"`
        Metrics string `toml:"metrics" env:"METRICS" default:"/metrics"`
        Health  string `toml:"health" env:"HEALTH" default:"/health"`
    } `toml:"monitoring"`
}
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
