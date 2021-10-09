package model

import "strings"

type Directive string

func (d Directive) Name() string {
	s := strings.TrimSuffix(string(d), ";")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (d Directive) Args() string {
	s := strings.TrimSuffix(string(d), ";")
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return ""
	}
	return strings.Join(fields[1:], " ")
}

func (d Directive) String() string {
	name := d.Name()
	args := d.Args()
	if args == "" {
		return name
	}
	return name + " " + args
}
