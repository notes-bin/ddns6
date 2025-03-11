package common

import (
	"testing"
)

type Config struct {
	DatabaseURL string `env:"DATABASE_URL"`
	Port        int    `env:"PORT"`
	DebugMode   bool   `env:"DEBUG_MODE"`
}

func TestGetEnvSafe(t *testing.T) {
	config := &Config{}
	err := GetEnvSafe(config)
	if err != nil {
		t.Errorf("Error: %+v\n", err)
		return
	}

	t.Logf("Config: %+v\n", config)
}
