package cli

import (
	"fmt"
	"slices"
	"strings"
)

// StringSlice 是一个自定义的字符串切片类型，支持 flag.Value 接口
type StringSlice []string

// String 返回 StringSlice 的字符串表示
func (s *StringSlice) String() string {
	return strings.Join(*s, ", ")
}

// Set 将输入的值按逗号分隔，并添加到切片中
func (s *StringSlice) Set(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			*s = append(*s, trimmed)
		}
	}
	return nil
}

// ChoiceValue 是一个自定义的选择值类型，支持 flag.Value 接口
type ChoiceValue struct {
	Value   string   // 当前选中的值
	Options []string // 可选值列表
	Default string   // 默认值
}

// String 返回 ChoiceValue 的字符串表示
func (c *ChoiceValue) String() string {
	if c.Value == "" {
		return c.Default
	}
	return c.Value
}

// Set 检查输入的值是否在可选值列表中，并设置值
func (c *ChoiceValue) Set(value string) error {
	if slices.Contains(c.Options, value) {
		c.Value = value
		return nil
	}
	return fmt.Errorf("invalid choice %q, options are: %v", value, c.Options)
}

// NewChoiceValue 创建一个新的 ChoiceValue 实例
func NewChoiceValue(defaultVal string, options []string) *ChoiceValue {
	return &ChoiceValue{
		Value:   defaultVal,
		Options: options,
		Default: defaultVal,
	}
}
