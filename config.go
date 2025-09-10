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

// LoadConfig loads configuration from TOML file or environment variables.
// The source is determined by the {APPNAME}_CONFIG_TYPE environment variable.
// If set to "env", loads from environment variables, otherwise loads from TOML file.
func LoadConfig[T any]() error {
	var err error
	cfg := new(T)

	ctype := os.Getenv(strings.ToUpper(AppName) + "_CONFIG_TYPE")
	if ctype == "env" {
		if err = loadConfigEnv[T](cfg); err != nil {
			return err
		}
	} else {
		if err = loadConfigFile[T](cfg); err != nil {
			return err
		}
	}

	v := reflect.ValueOf(cfg)
	err = checkRequiredField(v)
	if err != nil {
		log.Printf("Config load failed: %s", err)
		return err
	}

	instance = cfg

	return err
}

func loadConfigFile[T any](cfg *T) error {
	if configPath == "" {
		// 프로그램을 실행한 경로와 상관없이 실행파일의 경로에서 컨피그 파일을 찾도록 함
		exePath, err := os.Executable()
		if err != nil {
			log.Printf("Error getting executable path: %v", err)
			return err
		}
		configPath, err = filepath.Abs(exePath)
		if err != nil {
			log.Printf("Error getting absolute path: %v", err)
			return err
		}
	}

	dirPath := filepath.Dir(configPath)

	tree, err := toml.LoadFile(filepath.Join(dirPath, AppName+".toml"))
	if err != nil {
		log.Printf("Config load failed: %v", err)
		return err
	}

	err = tree.Unmarshal(cfg)
	if err != nil {
		log.Printf("Config load failed: %v", err)
		return err
	}

	return err
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

		envKeyBase := strings.ToUpper(parentPrefix + "_" + fieldInfo.EnvTag)
		if fieldInfo.EnvTag == "" {
			envKeyBase = strings.ToUpper(parentPrefix + "_" + fieldInfo.Name)
		}

		// --- ✅ 슬라이스(특히 []struct) 처리 ---
		if value.Kind() == reflect.Slice && fieldInfo.Type.Elem().Kind() == reflect.Struct {
			sliceValues, err := loadStructSliceEnv(envKeyBase, fieldInfo.Type.Elem())
			if err != nil {
				return err
			}
			value.Set(reflect.Append(value, sliceValues...))
			continue
		}

		// --- ✅ 일반 필드 처리 ---
		envValue := os.Getenv(envKeyBase)

		// 중첩 구조체는 값을 직접 설정하지 않고 재귀적으로 처리하므로 건너뛴다.
		if value.Kind() == reflect.Struct {
			if err := loadStructEnv(value, envKeyBase); err != nil {
				return err
			}
			continue
		}

		if envValue == "" && fieldInfo.Required {
			if fieldInfo.DefaultValue != "" {
				envValue = fieldInfo.DefaultValue
			} else {
				// checkRequiredField와 오류 메시지 형식을 통일합니다.
				tagName := fieldInfo.EnvTag
				if tagName == "" {
					tagName = fieldInfo.Name
				}
				return fmt.Errorf("required field '%s' is missing or empty", tagName)
			}
		}

		// Use unified parser for type conversion
		if envValue != "" || !isZero(value) {
			parsed, err := parseEnvValue(envValue, value.Type())
			if err != nil {
				return fmt.Errorf("failed to parse env value for field %s: %w", fieldInfo.Name, err)
			}
			value.Set(reflect.ValueOf(parsed))
		}
	}

	return nil
}

func loadStructSliceEnv(prefix string, t reflect.Type) ([]reflect.Value, error) {
	var result []reflect.Value

	for i := 0; ; i++ {
		elem := reflect.New(t).Elem()
		hasAnyValue := false

		for j := 0; j < t.NumField(); j++ {
			field := t.Field(j)
			tag := field.Tag.Get("env")
			if tag == "" {
				tag = field.Name
			}
			envKey := fmt.Sprintf("%s_%d_%s", prefix, i, strings.ToUpper(tag))
			envVal := os.Getenv(envKey)

			if envVal != "" {
				hasAnyValue = true
			}

			fieldVal := elem.Field(j)

			// Use unified parser for type conversion
			if envVal != "" || !isZero(fieldVal) {
				parsed, err := parseEnvValue(envVal, fieldVal.Type())
				if err != nil {
					return nil, fmt.Errorf("failed to parse env value for field %s: %w", field.Name, err)
				}
				fieldVal.Set(reflect.ValueOf(parsed))
			}
		}

		if !hasAnyValue {
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
