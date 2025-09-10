# Examples

This directory contains various examples demonstrating how to use AhatConfig-Go.

## Basic Example

**Location**: `basic/`

Demonstrates basic configuration loading from TOML file with:
- Simple nested structures
- Required fields
- Default values
- Secret masking

**Run**:
```bash
cd examples/basic
go run main.go
```

**Configuration file**: `basicapp.toml`

## Advanced Example

**Location**: `advanced/`

Demonstrates advanced features including:
- Complex nested structures
- Slice configurations
- Mixed type slices (int, string, bool)
- Multiple secret fields
- Rate limiting configuration

**Run**:
```bash
cd examples/advanced
go run main.go
```

**Configuration file**: `advancedapp.toml`

## Environment Variables Example

**Location**: `env-vars/`

Demonstrates configuration loading from environment variables:
- Environment variable naming conventions
- Production-like configuration
- Secret masking in environment variables

**Run**:
```bash
cd examples/env-vars
go run main.go
```

## Error Handling Example

**Location**: `error-handling/`

Demonstrates proper error handling patterns:
- Safe initialization with error returns
- Missing required field handling
- Best practices for production applications

**Run**:
```bash
cd examples/error-handling
go run main.go
```

## Configuration Tags Reference

### TOML Tags
- `toml:"field_name"` - Maps to TOML field name
- `toml:"section"` - Maps to TOML section name

### Environment Tags
- `env:"FIELD_NAME"` - Maps to environment variable name
- `required:"true"` - Field is required (validation)
- `default:"value"` - Default value if not provided
- `secret:"true"` - Masks value in logs (shows as "****")

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

## Best Practices

1. **Use Safe APIs in Production**: Use `InitConfigSafe` and `GetConfigSafe` for better error handling
2. **Mark Secrets**: Always mark sensitive fields with `secret:"true"`
3. **Validate Required Fields**: Use `required:"true"` for critical configuration
4. **Provide Defaults**: Use `default:"value"` for optional fields
5. **Environment Override**: Use environment variables for deployment-specific settings
