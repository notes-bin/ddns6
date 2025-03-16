package env_test

import (
	"os"
	"testing"
	"time"

	"github.com/notes-bin/ddns6/utils/env"
)

type Config struct {
	Host     string        `env:"HOST" default:"localhost"`
	Port     int           `env:"PORT" default:"8080"`
	Timeout  time.Duration `env:"TIMEOUT" default:"5s"`
	Debug    bool          `env:"DEBUG" default:"true"`
	Features []string      `env:"FEATURES" default:"feature1,feature2"`
}

func TestEnvToStruct(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	os.Setenv("PORT", "8080")
	os.Setenv("DEBUG_MODE", "true")
	os.Setenv("FEATURES", "feature1,feature2,feature3")
	config := &Config{}
	err := env.EnvToStruct(config, true)
	if err != nil {
		t.Errorf("Error: %+v\n", err)
		return
	}
	t.Logf("Config: %+v\n", config)
}

func TestEnvToStruct_DefaultValue(t *testing.T) {
	type Config struct {
		Host string `env:"HOST" default:"localhost"`
	}

	cfg := &Config{}
	if err := env.EnvToStruct(cfg, true); err != nil {
		t.Fatalf("EnvToStruct failed: %v", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("expected Host to be 'localhost', got: %s", cfg.Host)
	}
}

func TestEnvToStruct_MissingEnvAndNoDefault(t *testing.T) {
	type Config struct {
		Host string `env:"HOST"`
	}

	cfg := &Config{}
	err := env.EnvToStruct(cfg, true)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
