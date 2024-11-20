package gcfg

import (
	"bytes"
	"errors"
	"fmt"
)

// List holds a collection of warnings and optionally one fatal error.
type List struct {
	Warnings []error
	Fatal    error
}

// Error implements the error interface.
func (l List) Error() string {
	b := bytes.NewBuffer(nil)
	if l.Fatal != nil {
		fmt.Fprintln(b, "fatal:")
		fmt.Fprintln(b, l.Fatal)
	}
	switch len(l.Warnings) {
	case 0:
	// nop
	case 1:
		fmt.Fprintln(b, "warning:")
	default:
		fmt.Fprintln(b, "warnings:")
	}
	for _, err := range l.Warnings {
		fmt.Fprintln(b, err)
	}
	return b.String()
}

// A Collector collects errors up to the first fatal error.
type Collector struct {
	// IsFatal distinguishes between warnings and fatal errors.
	IsFatal func(error) bool
	// FatalWithWarnings set to true means that a fatal error is returned as
	// a List together with all warnings so far. The default behavior is to
	// only return the fatal error and discard any warnings that have been
	// collected.
	FatalWithWarnings bool

	l    List
	done bool
}

// NewCollector returns a new Collector; it uses isFatal to distinguish between
// warnings and fatal errors.
func NewCollector(isFatal func(error) bool) *Collector {
	return &Collector{IsFatal: isFatal}
}

// Collect collects a single error (warning or fatal). It returns nil if
// collection can continue (only warnings so far), or otherwise the errors
// collected. Collect mustn't be called after the first fatal error or after
// Done has been called.
func (c *Collector) Collect(err error) error {
	if c.done {
		return errors.New("warnings.Collector already done")
	}
	if err == nil {
		return nil
	}
	if c.IsFatal(err) {
		c.done = true
		c.l.Fatal = err
	} else {
		c.l.Warnings = append(c.l.Warnings, err)
	}
	if c.l.Fatal != nil {
		return c.erorr()
	}
	return nil
}

// Done ends collection and returns the collected error(s).
func (c *Collector) Done() error {
	c.done = true
	return c.erorr()
}

func (c *Collector) erorr() error {
	if !c.FatalWithWarnings && c.l.Fatal != nil {
		return c.l.Fatal
	}
	if c.l.Fatal == nil && len(c.l.Warnings) == 0 {
		return nil
	}
	// Note that a single warning is also returned as a List. This is to make it
	// easier to determine fatal-ness of the returned error.
	return c.l
}

// FatalOnly returns the fatal error, if any, **in an error returned by a
// Collector**. It returns nil if and only if err is nil or err is a List
// with err.Fatal == nil.
func FatalOnly(err error) error {
	l, ok := err.(List)
	if !ok {
		return err
	}
	return l.Fatal
}

// WarningsOnly returns the warnings **in an error returned by a Collector**.
func WarningsOnly(err error) []error {
	l, ok := err.(List)
	if !ok {
		return nil
	}
	return l.Warnings
}
