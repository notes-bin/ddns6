package command

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

type subCmd struct {
	*flag.FlagSet
	comment string
}

func NewSubCmd(name, usage string) *subCmd {
	return &subCmd{
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError),
		comment: usage,
	}
}

func (s *subCmd) String(name, value, usage string) *string {
	p := new(string)
	s.StringVar(p, name, value, usage)
	return p
}

func (s *subCmd) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	s.BoolVar(p, name, value, usage)
	return p
}

func (s *subCmd) Int(name string, value int, usage string) *int {
	p := new(int)
	s.IntVar(p, name, value, usage)
	return p
}

func (s *subCmd) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	s.Float64Var(p, name, value, usage)
	return p
}

type Duration time.Duration

func (d *Duration) String() string {
	return time.Duration(*d).String()
}

func (d *Duration) Set(value string) error {
	// 解析字符串形式的时间间隔
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("无效的时间间隔: %v", err)
	}
	*d = Duration(duration)
	return nil
}

type StringSlice []string

func (s *StringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *StringSlice) Set(value string) error {
	// 将输入的值按逗号分隔，并添加到切片中
	parts := strings.Split(value, ",")
	*s = append(*s, parts...)
	return nil
}

type ChoiceValue struct {
	Value   string
	Options []string
}

func (c *ChoiceValue) String() string {
	return c.Value
}

func (c *ChoiceValue) Set(value string) error {
	// 检查输入的值是否在可选值列表中
	for _, option := range c.Options {
		if value == option {
			c.Value = value
			return nil
		}
	}
	return fmt.Errorf("无效的值: %s, 可选值为: %v", value, c.Options)
}
