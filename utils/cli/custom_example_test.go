package cli

import (
	"flag"
	"fmt"
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

func ExampleCustomCmd() {
	flag.Parse()
	fmt.Println("Interval:", interval)
	fmt.Println("Tags: ", tags)
	fmt.Println("Mode: ", mode)
	// Output:
	// Interval: 5s
	// Tags:  [default]
	// Mode:  fast
}
