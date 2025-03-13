package common

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strconv"
)

var ErrEnvKeyNotFound = fmt.Errorf("environment variable does not exist")

// 从环境变量中获取值
func EnvToString(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("环境变量 %s 不存在, err: %w", key, ErrEnvKeyNotFound)
	}
	return value, nil
}

// 从环境变量中获取值并映射到结构体中
func EnvToStruct(obj any, errOnExit bool) error {
	v := reflect.ValueOf(obj).Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("参数 %v 必须是结构体", obj)
	}
	t := v.Type()

	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)
		envKey := fieldType.Tag.Get("env")

		if envKey == "" {
			continue
		}

		value, exists := os.LookupEnv(envKey)
		if !exists {
			if !errOnExit {
				continue
			}

			return fmt.Errorf("环境变量 %s 不存在, err: %w", envKey, ErrEnvKeyNotFound)
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("无法将环境变量 %s 转换为整数: %v", envKey, err)
			}
			field.SetInt(intValue)
		case reflect.Float32, reflect.Float64:
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("无法将环境变量 %s 转换为浮点数: %v", envKey, err)
			}
			field.SetFloat(floatValue)
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("无法将环境变量 %s 转换为布尔值: %v", envKey, err)
			}
			field.SetBool(boolValue)
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				field.Set(reflect.Append(field, reflect.ValueOf(value)))
			} else {
				return fmt.Errorf("不支持的切片类型: %s", field.Type().Elem().Kind())
			}
		default:
			return fmt.Errorf("不支持的字段类型: %s", field.Kind())
		}
	}

	return nil
}

// 将结构体中的字段映射到环境变量中
func StructToTagValueSlice(obj any, envKey string) ([]string, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("参数 %v 必须是结构体", obj)
	}
	v = v.Elem()
	t := v.Type()
	result := make([]string, 0, t.NumField())
	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)
		tagValue := fieldType.Tag.Get(envKey)

		var value string
		switch field.Kind() {
		case reflect.String:
			value = field.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			value = strconv.FormatInt(field.Int(), 10)
		case reflect.Float32, reflect.Float64:
			value = strconv.FormatFloat(field.Float(), 'f', -1, 64)
		case reflect.Bool:
			value = strconv.FormatBool(field.Bool())
		default:
			slog.Warn("不支持的字段类型", "field", fieldType.Name, "type", field.Kind())
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", tagValue, value))
	}

	return result, nil
}
