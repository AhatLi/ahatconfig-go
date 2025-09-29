package ahatconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/pelletier/go-toml"
)

var (
	instance   interface{}
	once       sync.Once
	AppName    string
	configPath string
)

// TypeInfo caches reflection information for performance optimization.
// It stores pre-computed field metadata to avoid repeated reflection operations.
type TypeInfo struct {
	Fields []FieldInfo
}

// FieldInfo contains cached field information extracted from struct tags.
// This information is computed once and reused for better performance.
type FieldInfo struct {
	Name         string       // Field name
	Type         reflect.Type // Field type
	EnvTag       string       // Environment variable tag
	DefaultValue string       // Default value tag
	Required     bool         // Required field flag
	Secret       bool         // Secret masking flag
}

// typeCache stores cached type information
var typeCache sync.Map

// getCachedTypeInfo returns cached type information or creates and caches it
func getCachedTypeInfo(t reflect.Type) *TypeInfo {
	if cached, ok := typeCache.Load(t); ok {
		return cached.(*TypeInfo)
	}

	typeInfo := &TypeInfo{
		Fields: make([]FieldInfo, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldInfo := FieldInfo{
			Name:         field.Name,
			Type:         field.Type,
			EnvTag:       field.Tag.Get("env"),
			DefaultValue: field.Tag.Get("default"),
			Required:     strings.ToLower(field.Tag.Get("required")) == "true",
			Secret:       strings.ToLower(field.Tag.Get("secret")) == "true",
		}
		typeInfo.Fields = append(typeInfo.Fields, fieldInfo)
	}

	typeCache.Store(t, typeInfo)
	return typeInfo
}

// InitConfig initializes the configuration for the given application name.
// It loads configuration from TOML file or environment variables based on the
// {APPNAME}_CONFIG_TYPE environment variable.
// Panics if configuration loading fails.
//
// Example:
//
//	ahatconfig.InitConfig[MyConfig]("myapp")
func InitConfig[T any](appname string) {
	AppName = appname

	once.Do(func() {
		err := LoadConfig[T]()
		if err != nil {
			panic(err)
		}
	})
}

// InitConfigWithPath initializes configuration with a custom executable path.
// This is useful when the configuration file is not in the same directory
// as the executable.
// Panics if configuration loading fails.
//
// Example:
//
//	ahatconfig.InitConfigWithPath[MyConfig]("myapp", "/custom/path")
func InitConfigWithPath[T any](appname, path string) {
	AppName = appname
	configPath = path

	once.Do(func() {
		err := LoadConfig[T]()
		if err != nil {
			panic(err)
		}
	})
}

// InitConfigSafe initializes configuration and returns error instead of panicking.
// This is the recommended approach for production applications where you want
// to handle configuration errors gracefully.
//
// Example:
//
//	err := ahatconfig.InitConfigSafe[MyConfig]("myapp")
//	if err != nil {
//	    log.Fatal(err)
//	}
func InitConfigSafe[T any](appname string) error {
	AppName = appname
	return LoadConfig[T]()
}

// InitConfigWithPathSafe initializes configuration with custom path and returns error instead of panicking.
// This combines the functionality of InitConfigWithPath with safe error handling.
//
// Example:
//
//	err := ahatconfig.InitConfigWithPathSafe[MyConfig]("myapp", "/custom/path")
//	if err != nil {
//	    log.Fatal(err)
//	}
func InitConfigWithPathSafe[T any](appname, path string) error {
	AppName = appname
	configPath = path
	return LoadConfig[T]()
}

// LoadConfig loads configuration from TOML file first, then overrides with environment variables.
// Environment variables have higher priority and will override TOML values.
// This provides a hybrid approach where TOML serves as defaults and env vars as overrides.
func LoadConfig[T any]() error {
	var err error
	cfg := new(T)

	// First, try to load from TOML file (if it exists)
	tomlErr := loadConfigFile[T](cfg)
	if tomlErr != nil {
		log.Printf("TOML config load failed (this is OK if file doesn't exist): %v", tomlErr)
		// Continue with empty config - environment variables will populate it
	}

	// Then, override with environment variables (higher priority)
	// Don't fail if env loading has issues - TOML values can serve as fallback
	if envErr := loadConfigEnv[T](cfg); envErr != nil {
		log.Printf("Environment variable loading failed (this is OK if no env vars are set): %v", envErr)
		// Continue with TOML values only
	}

	v := reflect.ValueOf(cfg)
	err = checkRequiredField(v)
	if err != nil {
		log.Printf("Config load failed: %s", err)
		return err
	}

	instance = cfg

	return nil
}

func loadConfigFile[T any](cfg *T) error {
	var tomlPath string

	if configPath == "" {
		// First try current working directory
		wd, err := os.Getwd()
		if err != nil {
			log.Printf("Error getting working directory: %v", err)
			return err
		}
		tomlPath = filepath.Join(wd, AppName+".toml")
		// If not found in current directory, try executable directory
		if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
			exePath, err := os.Executable()
			if err != nil {
				log.Printf("Error getting executable path: %v", err)
				return err
			}
			exeDir := filepath.Dir(exePath)
			tomlPath = filepath.Join(exeDir, AppName+".toml")
		}
	} else {
		dirPath := filepath.Dir(configPath)
		tomlPath = filepath.Join(dirPath, AppName+".toml")
	}

	// Check if TOML file exists
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		// TOML file doesn't exist - this is OK, we'll use env vars only
		return nil
	}

	tree, err := toml.LoadFile(tomlPath)
	if err != nil {
		log.Printf("TOML file exists but failed to load: %v", err)
		return err
	}

	err = tree.Unmarshal(cfg)
	if err != nil {
		log.Printf("Failed to unmarshal TOML: %v", err)
		return err
	}
	return nil
}

func checkRequiredField(v reflect.Value) error {
	// 포인터면 구조체로 접근
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil // 구조체 아니면 무시
	}

	t := v.Type()
	typeInfo := getCachedTypeInfo(t)

	for i, fieldInfo := range typeInfo.Fields {
		value := v.Field(i)

		// 중첩 구조체면 재귀 검사
		if value.Kind() == reflect.Struct {
			if err := checkRequiredField(value); err != nil {
				return err
			}
			continue
		}

		// 슬라이스 안의 구조체 검사
		if value.Kind() == reflect.Slice && fieldInfo.Type.Elem().Kind() == reflect.Struct {
			for j := 0; j < value.Len(); j++ {
				if err := checkRequiredField(value.Index(j)); err != nil {
					return err
				}
			}
			continue
		}

		if !fieldInfo.Required {
			continue
		}

		// 비어있음 검사 (기본값 포함)
		if isZero(value) {
			tagName := fieldInfo.EnvTag
			if tagName == "" {
				tagName = fieldInfo.Name
			}
			return fmt.Errorf("required field '%s' is missing or empty", tagName)
		}
	}

	return nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return v.Int() == 0
	case reflect.Float64, reflect.Float32:
		return v.Float() == 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

// parseEnvValue parses environment variable value to the target type.
// Supports string, int, bool, float64, and slice types.
// Returns the parsed value or an error if parsing fails.
func parseEnvValue(envValue string, targetType reflect.Type) (interface{}, error) {
	if envValue == "" {
		return getZeroValue(targetType), nil
	}

	switch targetType.Kind() {
	case reflect.String:
		return envValue, nil
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return strconv.Atoi(envValue)
	case reflect.Bool:
		return strconv.ParseBool(envValue)
	case reflect.Float64, reflect.Float32:
		return strconv.ParseFloat(envValue, 64)
	case reflect.Slice:
		return parseSliceValue(envValue, targetType)
	default:
		return nil, fmt.Errorf("unsupported type: %v", targetType.Kind())
	}
}

// parseSliceValue parses comma-separated values into a slice.
// Handles slices of string, int, bool, and float64 types.
// Empty values are skipped during parsing.
func parseSliceValue(envValue string, sliceType reflect.Type) (interface{}, error) {
	elemType := sliceType.Elem()
	strs := strings.Split(envValue, ",")
	sliceVal := reflect.MakeSlice(sliceType, 0, len(strs))

	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		parsed, err := parseEnvValue(s, elemType)
		if err != nil {
			return nil, err
		}
		sliceVal = reflect.Append(sliceVal, reflect.ValueOf(parsed))
	}
	return sliceVal.Interface(), nil
}

// getZeroValue returns the zero value for the given type.
// Used when environment variable is empty or not set.
func getZeroValue(t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.String:
		return ""
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return 0
	case reflect.Bool:
		return false
	case reflect.Float64, reflect.Float32:
		return 0.0
	case reflect.Slice:
		return reflect.MakeSlice(t, 0, 0).Interface()
	default:
		return reflect.Zero(t).Interface()
	}
}

func loadConfigEnv[T any](cfg *T) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil // 구조체가 아니면 무시
	}

	return loadStructEnv(v, AppName)
}

func loadStructEnv(v reflect.Value, parentPrefix string) error {
	t := v.Type()
	typeInfo := getCachedTypeInfo(t)

	for i, fieldInfo := range typeInfo.Fields {
		value := v.Field(i)

		// Convert hyphens to underscores for environment variable names
		normalizedPrefix := strings.ReplaceAll(strings.ToUpper(parentPrefix), "-", "_")
		envKeyBase := normalizedPrefix + "_" + strings.ToUpper(fieldInfo.EnvTag)
		if fieldInfo.EnvTag == "" {
			envKeyBase = normalizedPrefix + "_" + strings.ToUpper(fieldInfo.Name)
		}

		// --- ✅ 슬라이스(특히 []struct) 처리 ---
		if value.Kind() == reflect.Slice && fieldInfo.Type.Elem().Kind() == reflect.Struct {
			sliceValues, err := loadStructSliceEnv(envKeyBase, fieldInfo.Type.Elem())
			if err != nil {
				return err
			}
			// In hybrid mode, if env vars exist, replace TOML slice completely
			// If no env vars, keep TOML slice
			if len(sliceValues) > 0 {
				value.Set(reflect.MakeSlice(value.Type(), 0, len(sliceValues)))
				value.Set(reflect.Append(value, sliceValues...))
			}
			continue
		}

		// --- ✅ 일반 필드 처리 ---
		envValue := os.Getenv(envKeyBase)

		// 중첩 구조체는 값을 직접 설정하지 않고 재귀적으로 처리하므로 건너뛴다.
		if value.Kind() == reflect.Struct {
			// 환경변수가 있거나 기본값이 있는 경우 재귀적으로 처리
			hasEnvVars := hasStructEnvValues(value, envKeyBase)
			hasDefaults := hasStructDefaultValues(value)
			if envValue != "" || hasEnvVars || hasDefaults {
				if err := loadStructEnv(value, envKeyBase); err != nil {
					return err
				}
			}
			continue
		}

		// Apply default value if env is empty AND no TOML value exists
		// In hybrid mode, TOML values should take precedence over defaults
		if envValue == "" && fieldInfo.DefaultValue != "" && isZero(value) {
			envValue = fieldInfo.DefaultValue
		}

		// In hybrid mode, we don't validate required fields here
		// Required field validation is done in checkRequiredField after all loading is complete

		// Use unified parser for type conversion
		// Only set value if we have an environment variable or default value
		// This preserves TOML values when no env var is set
		if envValue != "" {
			parsed, err := parseEnvValue(envValue, value.Type())
			if err != nil {
				return fmt.Errorf("failed to parse env value for field %s: %w", fieldInfo.Name, err)
			}
			value.Set(reflect.ValueOf(parsed))
		}
	}

	return nil
}

// hasStructEnvValues는 중첩된 구조체에 환경변수 값이 있는지 확인하는 헬퍼 함수
func hasStructEnvValues(v reflect.Value, prefix string) bool {
	t := v.Type()
	typeInfo := getCachedTypeInfo(t)

	for i, fieldInfo := range typeInfo.Fields {
		value := v.Field(i)

		// Convert hyphens to underscores for environment variable names
		normalizedPrefix := strings.ReplaceAll(strings.ToUpper(prefix), "-", "_")
		envKeyBase := normalizedPrefix + "_" + strings.ToUpper(fieldInfo.EnvTag)
		if fieldInfo.EnvTag == "" {
			envKeyBase = normalizedPrefix + "_" + strings.ToUpper(fieldInfo.Name)
		}

		// 슬라이스 필드 처리
		if value.Kind() == reflect.Slice && fieldInfo.Type.Elem().Kind() == reflect.Struct {
			// 슬라이스의 첫 번째 요소에 대해 확인
			if hasStructSliceEnvValues(envKeyBase, fieldInfo.Type.Elem()) {
				return true
			}
			continue
		}

		// 중첩 구조체 재귀 확인
		if value.Kind() == reflect.Struct {
			log.Printf("DEBUG: Checking nested struct %s with prefix %s", fieldInfo.Name, envKeyBase)
			if hasStructEnvValues(value, envKeyBase) {
				log.Printf("DEBUG: Found env vars for nested struct %s", fieldInfo.Name)
				return true
			}
			continue
		}

		// 일반 필드 확인
		envValue := os.Getenv(envKeyBase)
		if envValue != "" {
			return true
		}
	}

	return false
}

// hasStructDefaultValues는 중첩된 구조체에 기본값이 있는지 확인하는 헬퍼 함수
func hasStructDefaultValues(v reflect.Value) bool {
	t := v.Type()
	typeInfo := getCachedTypeInfo(t)

	for _, fieldInfo := range typeInfo.Fields {
		// 기본값이 있는 필드가 있으면 true 반환
		if fieldInfo.DefaultValue != "" {
			return true
		}
	}

	return false
}

// hasStructSliceEnvValues는 구조체 슬라이스에 환경변수 값이 있는지 확인하는 헬퍼 함수
func hasStructSliceEnvValues(prefix string, t reflect.Type) bool {
	// 첫 번째 인덱스(0)에 대해서만 확인
	// Convert hyphens to underscores for environment variable names
	normalizedPrefix := strings.ReplaceAll(strings.ToUpper(prefix), "-", "_")
	envKey := fmt.Sprintf("%s_0_", normalizedPrefix)

	// 구조체의 모든 필드에 대해 환경변수가 있는지 확인
	for j := 0; j < t.NumField(); j++ {
		field := t.Field(j)
		tag := field.Tag.Get("env")
		if tag == "" {
			tag = field.Name
		}
		fieldEnvKey := envKey + strings.ToUpper(tag)

		if os.Getenv(fieldEnvKey) != "" {
			return true
		}

		// 중첩된 구조체 필드 확인
		if field.Type.Kind() == reflect.Struct {
			if hasStructEnvValues(reflect.New(field.Type).Elem(), fieldEnvKey) {
				return true
			}
		}
	}

	return false
}

func loadStructSliceEnv(prefix string, t reflect.Type) ([]reflect.Value, error) {
	var result []reflect.Value

	// Convert hyphens to underscores for environment variable names
	normalizedPrefix := strings.ReplaceAll(strings.ToUpper(prefix), "-", "_")

	for i := 0; ; i++ {
		elem := reflect.New(t).Elem()
		hasAnyEnvValue := false // Only count actual environment variables, not defaults

		for j := 0; j < t.NumField(); j++ {
			field := t.Field(j)
			tag := field.Tag.Get("env")
			if tag == "" {
				tag = field.Name
			}
			envKey := fmt.Sprintf("%s_%d_%s", normalizedPrefix, i, strings.ToUpper(tag))
			envVal := os.Getenv(envKey)

			// Get field info for default value and required check
			fieldInfo := FieldInfo{
				Name:         field.Name,
				Type:         field.Type,
				EnvTag:       tag,
				DefaultValue: field.Tag.Get("default"),
				Required:     strings.ToLower(field.Tag.Get("required")) == "true",
				Secret:       strings.ToLower(field.Tag.Get("secret")) == "true",
			}

			fieldVal := elem.Field(j)

			// 중첩된 구조체는 재귀적으로 처리
			if fieldVal.Kind() == reflect.Struct {
				if err := loadStructEnv(fieldVal, envKey); err != nil {
					return nil, err
				}
				// 구조체 필드가 처리되었는지 확인 (하위 필드에 env 값이 있는지)
				if hasStructEnvValues(fieldVal, envKey) {
					hasAnyEnvValue = true
				}
				continue
			}

			// Only count actual environment variables for hasAnyEnvValue
			if envVal != "" {
				hasAnyEnvValue = true
			}

			// Apply default value if env is empty (regardless of required status)
			if envVal == "" && fieldInfo.DefaultValue != "" {
				envVal = fieldInfo.DefaultValue
			}

			// Check required field validation - only if we have environment variables
			// In env-only mode, we should not fail here as required validation is done later
			if envVal == "" && fieldInfo.Required && hasAnyEnvValue {
				// required field인데 default 값도 없으면 에러 (단, 환경변수가 있는 경우에만)
				return nil, fmt.Errorf("required field '%s' is missing or empty", tag)
			}

			// Use unified parser for type conversion
			if envVal != "" || !isZero(fieldVal) {
				parsed, err := parseEnvValue(envVal, fieldVal.Type())
				if err != nil {
					return nil, fmt.Errorf("failed to parse env value for field %s: %w", field.Name, err)
				}
				fieldVal.Set(reflect.ValueOf(parsed))
			}
		}

		// Only break if no environment variables were found for this index
		// This prevents infinite loop when only default values are present
		if !hasAnyEnvValue {
			break
		}

		result = append(result, elem)
	}

	return result, nil
}

// GetConfig retrieves the loaded configuration.
// Panics if configuration is not initialized or type mismatch occurs.
//
// Example:
//
//	cfg := ahatconfig.GetConfig[MyConfig]()
func GetConfig[T any]() *T {
	if instance == nil {
		panic("Config not initialized. Call InitConfig first.")
	}
	cfg, ok := instance.(*T)
	if !ok {
		panic("Invalid config type")
	}
	return cfg
}

// GetConfigSafe retrieves the loaded configuration and returns error instead of panicking.
// This is the recommended approach for production applications.
//
// Example:
//
//	cfg, err := ahatconfig.GetConfigSafe[MyConfig]()
//	if err != nil {
//	    log.Fatal(err)
//	}
func GetConfigSafe[T any]() (*T, error) {
	if instance == nil {
		return nil, fmt.Errorf("config not initialized, call InitConfig first")
	}
	cfg, ok := instance.(*T)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}
	return cfg, nil
}

// PrintConfig prints the current configuration with secret masking applied.
// Fields marked with secret:"true" will be displayed as "****".
// This is useful for debugging and logging configuration values safely.
//
// Example:
//
//	ahatconfig.PrintConfig()
//	// Output:
//	// 🔹 config:
//	// {
//	//   "Server": {
//	//     "Host": "localhost",
//	//     "Port": 8080
//	//   },
//	//   "Database": {
//	//     "User": "admin",
//	//     "Password": "****"
//	//   }
//	// }
func PrintConfig() {
	masked := maskSecrets(instance)
	configBytes, err := json.MarshalIndent(masked, "", "  ")
	if err != nil {
		log.Printf("Failed to print config: %v", err)
		return
	}

	fmt.Println("🔹 config:")
	fmt.Println(string(configBytes))
}

func maskSecrets(cfg interface{}) interface{} {
	v := reflect.ValueOf(cfg)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		typeInfo := getCachedTypeInfo(t)
		masked := map[string]interface{}{}

		for i, fieldInfo := range typeInfo.Fields {
			field := v.Field(i)
			fieldName := fieldInfo.Name

			// 재귀 구조
			if field.Kind() == reflect.Struct || field.Kind() == reflect.Slice {
				masked[fieldName] = maskSecrets(field.Interface())
				continue
			}

			// 시크릿 마스킹
			if fieldInfo.Secret {
				masked[fieldName] = "****"
			} else {
				masked[fieldName] = field.Interface()
			}
		}
		return masked

	case reflect.Slice:
		result := []interface{}{}
		for i := 0; i < v.Len(); i++ {
			result = append(result, maskSecrets(v.Index(i).Interface()))
		}
		return result

	default:
		return cfg
	}
}
