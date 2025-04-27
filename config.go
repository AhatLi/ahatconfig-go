package AhatGoKit

import (
	"encoding/json"
	"fmt"
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

func InitConfig[T any](appname string) {
	AppName = appname

	once.Do(func() {
		err := LoadConfig[T]()
		if err != nil {
			panic(err)
		}
	})
}

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
		fmt.Printf("Config load failed: %s", err)
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
			fmt.Println("Error getting executable path:", err)
			return err
		}
		configPath, err = filepath.Abs(exePath)
		if err != nil {
			fmt.Println("Error getting absolute path:", err)
		}
	}

	dirPath := filepath.Dir(configPath)

	tree, err := toml.LoadFile(filepath.Join(dirPath, AppName+".toml"))
	if err != nil {
		fmt.Printf("Config load failed: %s", err)
		return err
	}

	err = tree.Unmarshal(cfg)
	if err != nil {
		fmt.Printf("Config load failed: %s", err)
		return err
	}

	fmt.Println("🔹TOML Loaded")
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

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// 중첩 구조체면 재귀 검사
		if value.Kind() == reflect.Struct {
			if err := checkRequiredField(value); err != nil {
				return err
			}
			continue
		}

		// 슬라이스 안의 구조체 검사
		if value.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			for j := 0; j < value.Len(); j++ {
				if err := checkRequiredField(value.Index(j)); err != nil {
					return err
				}
			}
			continue
		}

		required := strings.ToLower(field.Tag.Get("required")) == "true"
		if !required {
			continue
		}

		// 비어있음 검사 (기본값 포함)
		if isZero(value) {
			envTag := field.Tag.Get("env")
			fieldName := field.Name
			tagName := envTag
			if tagName == "" {
				tagName = fieldName
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

func loadConfigEnv[T any](cfg *T) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Struct {
		v = v.Addr() // 포인터로 변환
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		fieldValue := v.Field(i)

		// 구조체 내부 순회
		if fieldValue.Kind() == reflect.Struct {
			t := v.Type()
			field := t.Field(i)

			if err := loadStructEnv(fieldValue, field.Name); err != nil {
				fmt.Println(err)
				return err
			}
		}
	}

	fmt.Println("🔹Env Loaded")
	return nil
}

func loadStructEnv(v reflect.Value, parentPrefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		envTag := field.Tag.Get("env")
		defaultValue := field.Tag.Get("default")
		required := strings.ToLower(field.Tag.Get("required")) == "true"

		envKeyBase := strings.ToUpper(parentPrefix + "_" + envTag)

		// --- ✅ 슬라이스(특히 []struct) 처리 ---
		if value.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			sliceValues, err := loadStructSliceEnv(envKeyBase, field.Type.Elem())
			if err != nil {
				fmt.Println("ahat 1 [", envTag, envTag, defaultValue)
				fmt.Println(err)
				return err
			}
			value.Set(reflect.Append(value, sliceValues...))
			continue
		}

		// --- ✅ 일반 필드 처리 ---
		envValue := os.Getenv(envKeyBase)

		if envValue == "" && required {
			if defaultValue != "" {
				envValue = defaultValue
			} else {
				fmt.Println("ahat 2")
				fmt.Println(fmt.Errorf("Required field %s is not set", envKeyBase))
				return fmt.Errorf("Required field %s is not set", envKeyBase)
			}
		}

		switch value.Kind() {
		case reflect.String:
			value.SetString(envValue)

		case reflect.Int:
			if envValue != "" {
				num, err := strconv.Atoi(envValue)
				if err != nil {
					fmt.Println("ahat 3")
					fmt.Println(err)
					return err
				}
				value.SetInt(int64(num))
			}

		case reflect.Bool:
			if envValue != "" {
				b, err := strconv.ParseBool(envValue)
				if err != nil {
					fmt.Println("ahat 4")
					fmt.Println(err)
					return err
				}
				value.SetBool(b)
			}

		case reflect.Float64:
			if envValue != "" {
				f, err := strconv.ParseFloat(envValue, 64)
				if err != nil {
					fmt.Println("ahat 5")
					fmt.Println(err)
					return err
				}
				value.SetFloat(f)
			}

		case reflect.Slice:
			elemKind := field.Type.Elem().Kind()
			if envValue != "" {
				strs := strings.Split(envValue, ",")
				sliceVal := reflect.MakeSlice(field.Type, 0, len(strs))

				for _, s := range strs {
					s = strings.TrimSpace(s)
					switch elemKind {
					case reflect.String:
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(s))
					case reflect.Int:
						n, err := strconv.Atoi(s)
						if err != nil {
							fmt.Println("ahat 6")
							fmt.Println(err)
							return err
						}
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(n))
					case reflect.Float64:
						f, err := strconv.ParseFloat(s, 64)
						if err != nil {
							fmt.Println("ahat 7")
							fmt.Println(err)
							return err
						}
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(f))
					case reflect.Bool:
						b := strings.ToLower(s) == "true"
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(b))
					}
				}
				value.Set(sliceVal)
			}
		case reflect.Struct:
			err := loadStructEnv(value, field.Name)
			if err != nil {
				fmt.Println("ahat 8")
				fmt.Println(err)
				return err
			}
		}
	}

	return nil
}

func loadStructSliceEnv(prefix string, t reflect.Type) ([]reflect.Value, error) {
	fmt.Println("ahat 1-1", prefix)
	fmt.Println("ahat 1-2", t)

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

			fmt.Println("ahat 1-8 tag", tag)
			fmt.Println("ahat 1-8 envKey", envKey)
			fmt.Println("ahat 1-8 envVal", envVal)

			if envVal != "" {
				hasAnyValue = true
			}

			fieldVal := elem.Field(j)
			fmt.Println("ahat 1-7", fieldVal)

			switch fieldVal.Kind() {
			case reflect.String:
				fieldVal.SetString(envVal)
			case reflect.Int:
				if envVal != "" {
					num, err := strconv.Atoi(envVal)
					if err != nil {
						fmt.Println("ahat 1-3")
						return nil, err
					}
					fieldVal.SetInt(int64(num))
				}
			case reflect.Bool:
				if envVal != "" {
					b, err := strconv.ParseBool(envVal)
					if err != nil {
						fmt.Println("ahat 1-4")
						return nil, err
					}
					fieldVal.SetBool(b)
				}
			case reflect.Slice:
				if envVal == "" {
					break // 빈 값이면 건너뜀
				}

				elemKind := fieldVal.Type().Elem().Kind()
				strs := strings.Split(envVal, ",")
				sliceVal := reflect.MakeSlice(fieldVal.Type(), 0, len(strs))

				for _, s := range strs {
					s = strings.TrimSpace(s)
					switch elemKind {
					case reflect.String:
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(s))
					case reflect.Int:
						if s == "" {
							continue
						}
						n, err := strconv.Atoi(s)
						if err != nil {
							fmt.Println("ahat 1-5", s)
							return nil, err
						}
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(n))
					case reflect.Bool:
						b := strings.ToLower(s) == "true"
						sliceVal = reflect.Append(sliceVal, reflect.ValueOf(b))
					}
				}
				fieldVal.Set(sliceVal)
			}
		}

		if !hasAnyValue {
			break
		}

		result = append(result, elem)
	}

	return result, nil
}

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

func PrintConfig() {
	masked := maskSecrets(instance)
	configBytes, err := json.MarshalIndent(masked, "", "  ")
	if err != nil {
		fmt.Printf("Failed to print config: %s", err)
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
		masked := map[string]interface{}{}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldType := t.Field(i)
			secretTag := fieldType.Tag.Get("secret")
			fieldName := fieldType.Name

			// 재귀 구조
			if field.Kind() == reflect.Struct || field.Kind() == reflect.Slice {
				masked[fieldName] = maskSecrets(field.Interface())
				continue
			}

			// 시크릿 마스킹
			if strings.ToLower(secretTag) == "true" {
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

func getEnvInt(envValue string) (int64, error) {
	// required가 false 이면서 값이 없으므로 0을 반환하는 경우
	if envValue == "" {
		return 0, nil
	}

	tmp, err := strconv.ParseInt(strings.TrimSpace(envValue), 10, 64)
	if err != nil {
		return 0, err
	}

	return tmp, nil
}

func getEnvFloat(envValue string) (float64, error) {
	if envValue == "" {
		return 0, nil
	}

	tmp, err := strconv.ParseFloat(strings.TrimSpace(envValue), 64)
	if err != nil {
		return 0, err
	}

	return tmp, nil
}

func getEnvBool(envValue string) (bool, error) {
	if envValue == "" {
		return false, nil
	}

	if strings.ToLower(envValue) == "true" {
		return true, nil
	} else if strings.ToLower(envValue) == "false" {
		return false, nil
	}

	return false, fmt.Errorf("Invalid boolean value")
}
