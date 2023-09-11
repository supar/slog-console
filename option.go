package slogconsole

import (
	"sync/atomic"

	"golang.org/x/exp/slog"
)

// BoolValuer is the interface that wraps Bool method
type BoolValuer interface {
	Bool() bool
}

// BoolVar reprerents atomic bool value
type BoolVar struct {
	val atomic.Bool
}

// Bool returns BoolVar value
func (b *BoolVar) Bool() bool {
	return b.val.Load()
}

// Set sets BoolVar with given value v
func (b *BoolVar) Set(v bool) {
	b.val.Store(v)
}

const defaultTimeFormat = "2006-01-02 15:04:05.000"

// Options represents ConsoleHandler options
type Options struct {
	// Colorize the "level" word
	// DEBUG and low - white
	// INFO - green
	// WARN - yellow
	// ERRPR and higher - red
	// Can be change cuncurently
	Colorize BoolValuer

	// Remove time part from message line
	DropTime bool

	// Change the "level" word. May be used in case of the extended list of levels
	StringLevel func(slog.Level) string

	// Custom timestamp format.
	// Default: 2006-01-02 15:04:05.000"
	TimeFormat string

	// Default options from slog
	slog.HandlerOptions
}
