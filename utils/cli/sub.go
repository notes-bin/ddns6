package cli

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// SubCmd 表示一个子命令
type SubCmd struct {
	*flag.FlagSet
	comment string
}

// NewSubCmd 创建一个新的子命令
func NewSubCmd(name, usage string) *SubCmd {
	return &SubCmd{
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
		comment: usage,
	}
}

// String 定义一个字符串类型的命令行参数
func (s *SubCmd) String(name, value, usage string) *string {
	p := new(string)
	s.StringVar(p, name, value, usage)
	return p
}

// Bool 定义一个布尔类型的命令行参数
func (s *SubCmd) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	s.BoolVar(p, name, value, usage)
	return p
}

// Int 定义一个整数类型的命令行参数
func (s *SubCmd) Int(name string, value int, usage string) *int {
	p := new(int)
	s.IntVar(p, name, value, usage)
	return p
}

// Float64 定义一个浮点数类型的命令行参数
func (s *SubCmd) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	s.Float64Var(p, name, value, usage)
	return p
}

// Duration 定义一个时间间隔类型的命令行参数
func (s *SubCmd) Duration(name string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	s.DurationVar(p, name, value, usage)
	return p
}

// StringSlice 定义一个字符串切片类型的命令行参数
func (s *SubCmd) StringSlice(name string, value []string, usage string) *[]string {
	p := &value
	s.Var((*StringSlice)(p), name, usage)
	return p
}

// Choice 定义一个选择类型的命令行参数
func (s *SubCmd) Choice(name string, value string, options []string, usage string) *string {
	p := &ChoiceValue{
		Value:   value,
		Options: options,
		Default: value,
	}
	s.Var(p, name, usage)
	return &p.Value
}

// Parse 解析命令行参数
func (s *SubCmd) Parse(args []string) error {
	if err := s.FlagSet.Parse(args); err != nil {
		return fmt.Errorf("failed to parse subcommand %s: %w", s.Name(), err)
	}
	return nil
}

// Help 返回子命令的帮助信息
func (s *SubCmd) Help() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Usage: %s %s\n", s.Name(), s.comment))
	s.VisitAll(func(f *flag.Flag) {
		builder.WriteString(fmt.Sprintf("  -%s: %s\n", f.Name, f.Usage))
	})
	return builder.String()
}
