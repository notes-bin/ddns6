package cli_test

import (
	"flag"
	"fmt"
	"time"

	"github.com/notes-bin/ddns6/utils/cli"
)

var interval time.Duration
var tags cli.StringSlice
var mode = cli.NewChoiceValue("fast", []string{"fast", "slow", "auto"})

func Example_custom() {
	flag.DurationVar(&interval, "interval", 5*time.Second, "Time interval")
	flag.Var(&tags, "tags", "Comma-separated list of tags")
	flag.Var(mode, "mode", "Operation mode (fast, slow, auto)")
	flag.Parse()
	fmt.Println("Interval:", interval)
	fmt.Println("Tags: ", tags)
	fmt.Println("Mode: ", mode)
	// Output:
	// Interval: 5s
	// Tags:  [default]
	// Mode:  fast
}
