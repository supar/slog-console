package slogconsole

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	ConsoleColorReset  = "\033[0m"
	ConsoleColorRed    = "\033[31m"
	ConsoleColorGreen  = "\033[32m"
	ConsoleColorYellow = "\033[33m"
	ConsoleColorBlue   = "\033[34m"
	ConsoleColorPurple = "\033[35m"
	ConsoleColorCyan   = "\033[36m"
	ConsoleColorGray   = "\033[37m"
	ConsoleColorWhite  = "\033[97m"
)

func appendString(dst []byte, str string) []byte {
	if needsQuoting(str) {
		return strconv.AppendQuote(dst, str)
	}
	return append(dst, []byte(str)...)
}

// ConsoleHandler represents console log handler
type ConsoleHandler struct {
	opts Options

	groups       []string
	preformatted []byte
	prefix       string

	mu  *sync.Mutex
	out io.Writer
}

// New creates a ConsoleHandler that writes to w, using the given options.
// If opts is nil, the default options are used.
func New(w io.Writer, opts *Options) (h *ConsoleHandler) {
	if opts == nil {
		opts = &Options{}
	}

	h = &ConsoleHandler{
		opts: *opts,
		mu:   new(sync.Mutex),
		out:  w,
	}
	// defaults
	h.opts.Level = optionalLevelVar(h.opts.Level)
	if h.opts.Colorize == nil {
		h.opts.Colorize = new(BoolVar)
	}
	if len(h.opts.TimeFormat) == 0 {
		h.opts.TimeFormat = defaultTimeFormat
	}

	if h.out == nil {
		h.out = os.Stderr
	}

	return
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle formats its argument Record as a single line of space-separated key=value items.
//   - Omits empty time or Options.DropTime is true
//   - Level string. Can be changed with Options.StringLevel
//   - If the AddSource option is set and source information is available,
//     the key is "source" and the value is output as FILE:LINE
//
// See Options to modify other attributes
func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	cm := newComposer(h)
	defer cm.destruct()

	// write timestamp
	cm.appendTime(r.Time)
	// write level
	cm.appendLevel(r.Level)
	// message
	if len(r.Message) > 0 {
		cm.addSpace(cm.bufLen() > 0)
		cm.buf.writeString(r.Message)
	}
	// write source
	cm.appendSource(r.PC)
	// write preformatted
	cm.addSpace(cm.bufLen() > 0 && len(h.preformatted) > 0)
	cm.buf.write(h.preformatted)
	// write record attributes
	if r.NumAttrs() > 0 {
		r.Attrs(cm.walkAttrs)
	}

	// at the end of the day new line
	cm.buf.writeString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(*cm.buf)

	return err
}

// WithAttrs returns a new ConsoleHandler
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.withAttrs(attrs)
}

// WithAttrs returns a new ConsoleHandler
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	return h.withGroup(name)
}

func mergePrefWithKey(pref, key string) string {
	if len(pref) > 0 {
		// we have pref and key
		if len(key) > 0 {
			return pref + "." + key
		}

		// we have only pref
		return pref
	}

	// fallback to key
	return key
}

// Copied from slog/text_handler.go
func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			// Quote anything except a backslash that would need quoting in a
			// JSON string, as well as space and '='
			if b != '\\' && (b == ' ' || b == '=' || !safeSet[b]) {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}

func (h *ConsoleHandler) withGroup(name string) *ConsoleHandler {
	if name == "" {
		return h
	}

	h2 := *h

	buf := allocBuf()
	defer buf.free()

	buf.writeString(h.prefix)
	if len(*buf) > 0 {
		buf.writeByte('.')
	}
	buf.writeString(name)
	h2.prefix = string(*buf)

	// groups list to use them in the AttrReplace
	h2.groups = make([]string, len(h2.groups)+1)
	copy(h2.groups, h.groups)
	h2.groups[len(h2.groups)-1] = name

	return &h2
}

func (h *ConsoleHandler) withAttrs(attrs []slog.Attr) *ConsoleHandler {
	if len(attrs) == 0 {
		return h
	}

	h2 := *h

	cm := newComposer(h)
	defer cm.destruct()

	cm.buf.write(h.preformatted)
	for _, a := range attrs {
		cm.appendAttr(a, h2.prefix)
	}

	h2.preformatted = make([]byte, len(*cm.buf))
	copy(h2.preformatted, *cm.buf)

	return &h2
}
