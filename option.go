package slogconsole

import (
	"log/slog"
	"sync/atomic"
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
	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Colorize the "level" word
	// DEBUG and low - white
	// INFO - green
	// WARN - yellow
	// ERRPR and higher - red
	// Can be change cuncurently
	Colorize BoolValuer

	// Remove time part from message line
	DropTime bool

	// Level reports the minimum record level that will be logged.
	Level slog.Leveler

	// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
	// The attribute's value has been resolved (see [Value.Resolve]).
	// If ReplaceAttr returns a zero Attr, the attribute is discarded.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// Change the "level" word. May be used in case of the extended list of levels
	StringLevel func(slog.Level) string

	// Custom timestamp format.
	// Default: 2006-01-02 15:04:05.000"
	TimeFormat string
}

func optionalLevelVar(lv slog.Leveler) slog.Leveler {
	if lv == nil {
		lv = new(slog.LevelVar)
	}

	return lv
}
