package cli

import (
	"strings"
	"testing"
)

// 测试 StringSlice 的 Set 方法
func TestStringSlice_Set(t *testing.T) {
	var s StringSlice
	value := "a,b,c"
	err := s.Set(value)
	if err != nil {
		t.Errorf("StringSlice.Set() error = %v", err)
	}
	expected := []string{"a", "b", "c"}
	if !equalStringSlices(s, expected) {
		t.Errorf("StringSlice.Set() = %v, want %v", s, expected)
	}
}

// 测试 StringSlice 的 String 方法
func TestStringSlice_String(t *testing.T) {
	s := StringSlice{"a", "b", "c"}
	expected := "a, b, c"
	result := s.String()
	if result != expected {
		t.Errorf("StringSlice.String() = %q, want %q", result, expected)
	}
}

// 测试 ChoiceValue 的 Set 方法，使用有效选项
func TestChoiceValue_Set_Valid(t *testing.T) {
	options := []string{"option1", "option2", "option3"}
	c := NewChoiceValue("option1", options)
	value := "option2"
	err := c.Set(value)
	if err != nil {
		t.Errorf("ChoiceValue.Set() error = %v", err)
	}
	if c.Value != value {
		t.Errorf("ChoiceValue.Set() = %q, want %q", c.Value, value)
	}
}

// 测试 ChoiceValue 的 Set 方法，使用无效选项
func TestChoiceValue_Set_Invalid(t *testing.T) {
	options := []string{"option1", "option2", "option3"}
	c := NewChoiceValue("option1", options)
	value := "invalidOption"
	err := c.Set(value)
	if err == nil {
		t.Errorf("ChoiceValue.Set() should return an error for invalid option")
	}
	if !strings.Contains(err.Error(), "invalid choice") {
		t.Errorf("ChoiceValue.Set() error message does not contain 'invalid choice': %v", err)
	}
}

// 测试 ChoiceValue 的 String 方法，使用默认值
func TestChoiceValue_String_Default(t *testing.T) {
	options := []string{"option1", "option2", "option3"}
	defaultVal := "option1"
	c := NewChoiceValue(defaultVal, options)
	result := c.String()
	if result != defaultVal {
		t.Errorf("ChoiceValue.String() = %q, want %q", result, defaultVal)
	}
}

// 测试 ChoiceValue 的 String 方法，使用已设置的值
func TestChoiceValue_String_SetValue(t *testing.T) {
	options := []string{"option1", "option2", "option3"}
	c := NewChoiceValue("option1", options)
	value := "option2"
	_ = c.Set(value)
	result := c.String()
	if result != value {
		t.Errorf("ChoiceValue.String() = %q, want %q", result, value)
	}
}

// 辅助函数，用于比较两个字符串切片是否相等
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
