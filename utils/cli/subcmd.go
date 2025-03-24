package cli

import (
	"flag"
	"fmt"
	"strings"
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
