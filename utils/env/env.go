package env

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var ErrEnvKeyNotFound = fmt.Errorf("environment variable does not exist")

// EnvToString 从环境变量中获取字符串值
func EnvToString(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("environment variable %s not found: %w", key, ErrEnvKeyNotFound)
	}
	return value, nil
}

// EnvToStruct 从环境变量映射到结构体
func EnvToStruct(prefix string, obj any) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("expected a pointer, got %v", reflect.TypeOf(obj))
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected a struct, got %v", reflect.TypeOf(obj))
	}
	t := v.Type()

	if prefix != "" {
		prefix = strings.ToUpper(prefix) + "_"
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		envKey := fieldType.Tag.Get("env")
		if envKey == "" {
			continue
		}
		fullKey := prefix + envKey

		value, exists := os.LookupEnv(fullKey)
		def := fieldType.Tag.Get("default")
		if def != "" && !exists {
			value = def
		}
		req := fieldType.Tag.Get("required")
		if !exists && def == "" {
			if isTrue(req) {
				return fmt.Errorf("required key %s missing value", fullKey)
			}
			continue
		}

		if err := setFieldFromString(field, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}
	return nil
}

// setFieldFromString 根据字符串值设置结构体字段
func setFieldFromString(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration: %v", err)
			}
			field.SetInt(int64(duration))
		} else {
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer: %v", err)
			}
			field.SetInt(intValue)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer: %v", err)
		}
		field.SetUint(uintValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float: %v", err)
		}
		field.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean: %v", err)
		}
		field.SetBool(boolValue)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			if field.IsNil() {
				field.Set(reflect.MakeSlice(field.Type(), 0, 1))
			}
			field.Set(reflect.Append(field, reflect.ValueOf(value)))
		} else {
			return fmt.Errorf("unsupported slice type: %s", field.Type().Elem().Kind())
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	return nil
}

// StructToTagValueSlice 将结构体字段映射到键值对切片
func StructToTagValueSlice(obj any, tagKey string) ([]string, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %v", reflect.TypeOf(obj))
	}
	t := v.Type()
	var result []string
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		tagValue := fieldType.Tag.Get(tagKey)
		if tagValue == "" {
			continue
		}
		value, err := getFieldAsString(field)
		if err != nil {
			slog.Warn("unsupported field type", "field", fieldType.Name, "type", field.Kind())
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", tagValue, value))
	}
	return result, nil
}

// getFieldAsString 将结构体字段转换为字符串
func getFieldAsString(field reflect.Value) (string, error) {
	switch field.Kind() {
	case reflect.String:
		return field.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(field.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(field.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(field.Float(), 'f', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(field.Bool()), nil
	default:
		return "", fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

// isTrue 判断字符串是否表示布尔值 true
func isTrue(s string) bool {
	b, err := strconv.ParseBool(s)
	return err == nil && b
}
