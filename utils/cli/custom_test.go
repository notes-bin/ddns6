package cli

import (
	"flag"
	"testing"
	"time"
)

var interval time.Duration
var tags StringSlice
var mode = NewChoiceValue("fast", []string{"fast", "slow", "auto"})

func init() {
	flag.DurationVar(&interval, "interval", 5*time.Second, "Time interval")
	flag.Var(&tags, "tags", "Comma-separated list of tags")
	flag.Var(mode, "mode", "Operation mode (fast, slow, auto)")
}

func TestCustomCmd(t *testing.T) {
	flag.Parse()
	t.Log("Interval: ", interval)
	t.Log("Tags: ", tags)
	t.Log("Mode: ", mode)
}
