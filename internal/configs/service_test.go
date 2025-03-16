package configs_test

import (
	"testing"

	"github.com/notes-bin/ddns6/internal/configs"
)

func TestGenerateService(t *testing.T) {
	configs.GenerateService("test")
}
