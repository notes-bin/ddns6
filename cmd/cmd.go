package main

import "flag"

type subCmd struct {
	*flag.FlagSet
	comment string
}

func newSubCmd(name, usage string) *subCmd {
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
