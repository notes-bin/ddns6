package common

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// GetEnvSafe 从环境变量中获取值并映射到结构体中
func GetEnvSafe(config interface{}) error {
	v := reflect.ValueOf(config).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		envKey := fieldType.Tag.Get("env")

		if envKey == "" {
			continue
		}

		value, exists := os.LookupEnv(envKey)
		if !exists {
			return fmt.Errorf("环境变量 %s 不存在", envKey)
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
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("无法将环境变量 %s 转换为布尔值: %v", envKey, err)
			}
			field.SetBool(boolValue)
		default:
			return fmt.Errorf("不支持的字段类型: %s", field.Kind())
		}
	}

	return nil
}
