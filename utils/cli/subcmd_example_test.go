package cli_test

import (
	"fmt"
	"os"
	"time"

	"github.com/notes-bin/ddns6/utils/cli"
)

func ExampleSubCmd() {
	// 创建子命令
	cmd := cli.NewSubCmd("example", "An example subcommand")

	// 定义参数
	name := cmd.String("name", "world", "Your name")
	age := cmd.Int("age", 30, "Your age")
	duration := cmd.Duration("duration", 5*time.Second, "Time duration")
	tags := cmd.StringSlice("tags", []string{"default"}, "Comma-separated list of tags")
	mode := cmd.Choice("mode", "fast", []string{"fast", "slow", "auto"}, "Operation mode")

	// 解析参数
	if err := cmd.Parse(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// 输出结果
	fmt.Println("Name:", *name)
	fmt.Println("Age:", *age)
	fmt.Println("Duration:", *duration)
	fmt.Println("Tags:", *tags)
	fmt.Println("Mode:", *mode)

	// Output:
	// Your name: world
	// Your age: 30
	// Time duration: 5s
	// Comma-separated list of tags: [default]
	// Operation mode: fast
}

func ExampleSubCmd_Help() {
	cmd := cli.NewSubCmd("example", "An example subcommand")
	cmd.String("name", "world", "Your name")
	cmd.Int("age", 30, "Your age")
	cmd.Duration("duration", 5*time.Second, "Time duration")
	cmd.StringSlice("tags", []string{"default"}, "Comma-separated list of tags")
	cmd.Choice("mode", "fast", []string{"fast", "slow", "auto"}, "Operation mode")

	fmt.Println(cmd.Help())

	// Output:
	// Usage: example [options]
	//
	// Options:
	//   -age int
	//     	Your age (default 30)
	//   -duration duration
	//     	Time duration (default 5s)
	//   -mode string
	//     	Operation mode (fast, slow, auto) (default "fast")
	//   -name string
	//     	Your name (default "world")
	//   -tags string
	//     	Comma-separated list of tags (default [default])
}
