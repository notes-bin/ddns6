package systemd_test

import (
	"testing"

	"github.com/notes-bin/ddns6/internal/configs/systemd"
)

func TestGenerateService(t *testing.T) {
	systemd.GenerateService("test")
}
