package slogconsole

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/exp/slog"
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

func addSpace(add bool) (s string) {
	if add {
		s = " "
	}
	return s
}

func appendString(dst []byte, str string) []byte {
	if needsQuoting(str) {
		return strconv.AppendQuote(dst, str)
	}
	return append(dst, []byte(str)...)
}

// Copied from slog package
func appendValue(v slog.Value, dst []byte) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendString(dst, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(dst, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(dst, v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.AppendFloat(dst, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.AppendBool(dst, v.Bool())
	case slog.KindDuration:
		return append(dst, v.Duration().String()...)
	case slog.KindTime:
		return append(dst, v.Time().String()...)
	case slog.KindGroup:
		return fmt.Append(dst, v.Group())
	case slog.KindAny, slog.KindLogValuer:
		return fmt.Append(dst, v.Any())
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

// ConsoleHandler represents console log handler
type ConsoleHandler struct {
	opts Options

	preformatted []byte
	prefix       []byte

	mu  *sync.Mutex
	out io.Writer
}

func (h *ConsoleHandler) appendAttr(buf *buffer, a slog.Attr, keyPref string) {
	if h.opts.ReplaceAttr != nil {
		a = h.opts.ReplaceAttr(nil, a)
	}

	// Resolve the Attr's value before doing anything else.
	a.Value = a.Value.Resolve()
	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return
	}

	switch a.Value.Kind() {
	case slog.KindGroup:
		attrs := a.Value.Group()
		// Ignore empty groups.
		if len(attrs) == 0 {
			return
		}

		for _, ga := range attrs {
			h.appendAttr(buf, ga, mergePrefWithKey(keyPref, a.Key))
		}

	default:
		buf.writeString(addSpace(len(*buf) > 0))

		buf.writeString(mergePrefWithKey(keyPref, a.Key) + "=")
		*buf = appendValue(a.Value, *buf)
	}
}

func (h *ConsoleHandler) appendLevel(buf *buffer, lv slog.Level, color bool) {
	var lvStr string
	if h.opts.StringLevel != nil {
		lvStr = h.opts.StringLevel(lv)
	} else {
		lvStr = lv.String()
	}

	color = color && runtime.GOOS != "windows"
	if !color {
		buf.writeString(addSpace(len(*buf) > 0) + lvStr)
		return
	}

	buf.writeString(addSpace(len(*buf) > 0))

	switch {
	case lv < slog.LevelInfo:
		buf.writeString(ConsoleColorWhite)
	case lv < slog.LevelWarn:
		buf.writeString(ConsoleColorGreen)
	case lv < slog.LevelError:
		buf.writeString(ConsoleColorYellow)
	default:
		buf.writeString(ConsoleColorRed)
	}

	buf.writeString(lvStr + ConsoleColorReset)
}

// Enabled
func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := allocBuf()
	defer buf.free()

	if !r.Time.IsZero() && !h.opts.DropTime {
		*buf = r.Time.AppendFormat(*buf, h.opts.TimeFormat)
	}

	h.appendLevel(buf, r.Level, h.opts.Colorize.Bool())

	if len(r.Message) > 0 {
		buf.writeString(addSpace(len(*buf) > 0) + r.Message)
	}

	// source
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		h.appendAttr(buf, slog.String(slog.SourceKey, fmt.Sprintf("%s=%d", f.File, f.Line)), "")
	}

	buf.writeString(addSpace(len(*buf) > 0 && len(h.preformatted) > 0))
	buf.write(h.preformatted)

	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			h.appendAttr(buf, a, string(h.prefix))
			return true
		})
	}

	// at the end of the day new line
	buf.writeString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(*buf)

	return err
}

func mergePrefWithKey(pref, key string) (v string) {
	if len(pref) > 0 && len(key) > 0 {
		v = pref + "." + key
	} else {
		v = key
	}
	return
}

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
	if h.opts.Colorize == nil {
		h.opts.Colorize = new(BoolVar)
	}
	if h.opts.Level == nil {
		h.opts.Level = new(slog.LevelVar)
	}
	if len(h.opts.TimeFormat) == 0 {
		h.opts.TimeFormat = defaultTimeFormat
	}

	if h.out == nil {
		h.out = os.Stderr
	}

	return
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

// Copied from encoding/json/tables.go.
//
// safeSet holds the value true if the ASCII character with the given array
// position can be represented inside a JSON string without any further
// escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), and the backslash character ("\").
var safeSet = [utf8.RuneSelf]bool{
	' ':      false,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := *h
	h2.prefix = make([]byte, 0, len(h.prefix)+len(name)+1)
	if len(h.prefix) > 0 {
		h2.prefix = append(h2.prefix, h.prefix...)
		h2.prefix = append(h2.prefix, '.')
	}
	h2.prefix = append(h2.prefix, []byte(name)...)

	return &h2
}

func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	h2 := *h

	buf := allocBuf()
	defer buf.free()

	buf.write(h.preformatted)

	for _, a := range attrs {
		h2.appendAttr(buf, a, string(h2.prefix))
	}

	h2.preformatted = make([]byte, len(*buf))
	copy(h2.preformatted, *buf)

	return &h2
}
