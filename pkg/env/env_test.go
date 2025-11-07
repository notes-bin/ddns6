package env

import (
	"os"
	"reflect"
	"testing"
	"time"
)

// 测试 EnvToString 函数
func TestEnvToString(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_KEY", "test_value")

	// 测试存在的环境变量
	value, err := EnvToString("TEST_KEY")
	if err != nil {
		t.Errorf("EnvToString(TEST_KEY) 出错: %v", err)
	}
	if value != "test_value" {
		t.Errorf("EnvToString(TEST_KEY) 期望得到 'test_value', 但得到了 '%s'", value)
	}

	// 测试不存在的环境变量
	_, err = EnvToString("NON_EXISTENT_KEY")
	if err == nil {
		t.Errorf("EnvToString(NON_EXISTENT_KEY) 期望出错, 但没有出错")
	}
}

// 测试 EnvToStruct 函数
func TestEnvToStruct(t *testing.T) {
	// 设置环境变量
	os.Setenv("PREFIX_HOST", "test_host")
	os.Setenv("PREFIX_PORT", "8081")
	os.Setenv("PREFIX_TIMEOUT", "10s")
	os.Setenv("PREFIX_DEBUG", "false")
	os.Setenv("PREFIX_FEATURES", "feature3,feature4")

	type Config struct {
		Host     string        `env:"HOST" default:"localhost"`
		Port     int           `env:"PORT" default:"8080"`
		Timeout  time.Duration `env:"TIMEOUT" default:"5s"`
		Debug    bool          `env:"DEBUG" default:"true"`
		Features []string      `env:"FEATURES" default:"feature1,feature2"`
	}

	config := &Config{}
	err := EnvToStruct("PREFIX", config)
	if err != nil {
		t.Errorf("EnvToStruct 出错: %v", err)
	}

	if config.Host != "test_host" {
		t.Errorf("期望 Host 为 'test_host', 但得到了 '%s'", config.Host)
	}
	if config.Port != 8081 {
		t.Errorf("期望 Port 为 8081, 但得到了 %d", config.Port)
	}
	if config.Timeout != 10*time.Second {
		t.Errorf("期望 Timeout 为 10s, 但得到了 %v", config.Timeout)
	}
	if config.Debug != false {
		t.Errorf("期望 Debug 为 false, 但得到了 %v", config.Debug)
	}
	if !reflect.DeepEqual(config.Features, []string{"feature3", "feature4"}) {
		t.Errorf("期望 Features 为 ['feature3', 'feature4'], 但得到了 %v", config.Features)
	}
}

// 测试 StructToTagValueSlice 函数
func TestStructToTagValueSlice(t *testing.T) {
	type Config struct {
		Host     string        `env:"HOST" default:"localhost"`
		Port     int           `env:"PORT" default:"8080"`
		Timeout  time.Duration `env:"TIMEOUT" default:"5s"`
		Debug    bool          `env:"DEBUG" default:"true"`
		Features []string      `env:"FEATURES" default:"feature1,feature2"`
	}

	config := Config{
		Host:     "test_host",
		Port:     8081,
		Timeout:  10 * time.Second,
		Debug:    false,
		Features: []string{"feature3", "feature4"},
	}

	result, err := StructToTagValueSlice(config, "env")
	if err != nil {
		t.Errorf("StructToTagValueSlice 出错: %v", err)
	}

	expected := []string{
		"HOST=test_host",
		"PORT=8081",
		"TIMEOUT=10s",
		"DEBUG=false",
		"FEATURES=feature3,feature4",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("期望结果为 %v, 但得到了 %v", expected, result)
	}
}
