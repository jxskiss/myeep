package api

import (
	"errors"
	"strings"

	"github.com/jxskiss/gopkg/v2/json"
)

type DirectiveCategory int

const (
	InvalidDirective DirectiveCategory = iota
	DomainDirective
	ServiceDirective
	RoutingDirective
	LocationDirective
)

var ErrInvalidDirective = errors.New("directive is invalid")

type Directive struct {
	full string
	name string
	args []string
}

func (d *Directive) Validate(cate DirectiveCategory) error {
	// TODO
	return nil
}

func (d *Directive) parse(s string) error {
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ErrInvalidDirective
	}

	d.full = s
	d.name = fields[0]
	d.args = fields[1:]
	return nil
}

func (d Directive) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.full)
}

func (d *Directive) UnmarshalJSON(data []byte) error {
	return d.parse(string(data))
}

func (d Directive) MarshalYAML() (interface{}, error) {
	return d.full, nil
}

func (d *Directive) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err == nil {
		err = d.parse(s)
	}
	return err
}

func (d *Directive) Name() string {
	return d.name
}

func (d *Directive) Args() []string {
	return d.args
}

func (d *Directive) ArgString() string {
	return strings.Join(d.args, " ")
}

func (d *Directive) String() string {
	return d.full
}
