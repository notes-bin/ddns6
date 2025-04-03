package cli

import (
	"flag"
	"os"
	"strings"
	"testing"
)

// 辅助函数：用于测试 StringSlice 方法
type mockStringSlice []string

func (m *mockStringSlice) Set(s string) error {
	*m = append(*m, s)
	return nil
}

func (m *mockStringSlice) String() string {
	return strings.Join(*m, ",")
}

// 测试 NewSubCmd 函数
func TestNewSubCmd(t *testing.T) {
	name := "testcmd"
	usage := "This is a test command"
	subCmd := NewSubCmd(name, usage)

	if subCmd.Name() != name {
		t.Errorf("Expected subcommand name to be %s, got %s", name, subCmd.Name())
	}
	if subCmd.comment != usage {
		t.Errorf("Expected subcommand comment to be %s, got %s", usage, subCmd.comment)
	}
}

// 测试 StringSlice 方法
func TestSubCmd_StringSlice(t *testing.T) {
	subCmd := NewSubCmd("testcmd", "This is a test command")
	value := []string{"a", "b"}
	name := "testslice"
	usage := "This is a test slice"

	result := subCmd.StringSlice(name, value, usage)

	if len(*result) != len(value) {
		t.Errorf("Expected result length to be %d, got %d", len(value), len(*result))
	}
	for i, v := range value {
		if (*result)[i] != v {
			t.Errorf("Expected result[%d] to be %s, got %s", i, v, (*result)[i])
		}
	}

	// 模拟设置标志
	var mock mockStringSlice
	flagSet := subCmd.FlagSet
	flagSet.Var(&mock, name, usage)
	err := flagSet.Parse([]string{"-" + name, "c"})
	if err != nil {
		t.Errorf("Failed to parse flag: %v", err)
	}
	if len(mock) != 1 || mock[0] != "c" {
		t.Errorf("Expected mock to be [c], got %v", mock)
	}
}

// 测试 Choice 方法
func TestSubCmd_Choice(t *testing.T) {
	subCmd := NewSubCmd("testcmd", "This is a test command")
	value := "option1"
	options := []string{"option1", "option2", "option3"}
	name := "testchoice"
	usage := "This is a test choice"

	result := subCmd.Choice(name, value, options, usage)

	if *result != value {
		t.Errorf("Expected result to be %s, got %s", value, *result)
	}

	// 模拟设置标志
	flagSet := subCmd.FlagSet
	choiceValue := &ChoiceValue{
		Value:   value,
		Options: options,
		Default: value,
	}
	flagSet.Var(choiceValue, name, usage)
	err := flagSet.Parse([]string{"-" + name, "option2"})
	if err != nil {
		t.Errorf("Failed to parse flag: %v", err)
	}
	if choiceValue.Value != "option2" {
		t.Errorf("Expected choiceValue.Value to be option2, got %s", choiceValue.Value)
	}
}

// 测试 Parse 方法
func TestSubCmd_Parse(t *testing.T) {
	subCmd := NewSubCmd("testcmd", "This is a test command")
	args := []string{"-testflag", "testvalue"}
	subCmd.FlagSet.String("testflag", "", "This is a test flag")

	err := subCmd.Parse(args)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	var testFlag string
	subCmd.FlagSet.StringVar(&testFlag, "testflag", "", "This is a test flag")
	err = subCmd.FlagSet.Parse(args)
	if err != nil {
		t.Errorf("Failed to parse flag: %v", err)
	}
	if testFlag != "testvalue" {
		t.Errorf("Expected testFlag to be testvalue, got %s", testFlag)
	}
}

// 测试 Help 方法
func TestSubCmd_Help(t *testing.T) {
	subCmd := NewSubCmd("testcmd", "This is a test command")
	subCmd.FlagSet.String("testflag", "", "This is a test flag")

	help := subCmd.Help()
	if !strings.Contains(help, "Usage: testcmd This is a test command") {
		t.Errorf("Expected help to contain 'Usage: testcmd This is a test command', got %s", help)
	}
	if !strings.Contains(help, "  -testflag: This is a test flag") {
		t.Errorf("Expected help to contain '  -testflag: This is a test flag', got %s", help)
	}
}

func init() {
	// 避免测试时调用 os.Exit
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
}