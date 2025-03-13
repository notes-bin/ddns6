package common

import (
	"os"
	"testing"
)

type Config struct {
	DatabaseURL string   `env:"DATABASE_URL"`
	Port        int      `env:"PORT"`
	DebugMode   bool     `env:"DEBUG_MODE"`
	Features    []string `env:"FEATURES"`
}

func TestEnvToStruct(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	// os.Setenv("PORT", "8080")
	os.Setenv("DEBUG_MODE", "true")
	os.Setenv("FEATURES", "feature1,feature2,feature3")
	config := &Config{}
	err := EnvToStruct(config, true)
	if err != nil {
		t.Errorf("Error: %+v\n", err)
		return
	}
	t.Logf("Config: %+v\n", config)
}
