package slogconsole

import (
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"time"
	"unicode/utf8"
)

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
		// append direct to avoid allocation
		//
		// https://pkg.go.dev/time@go1.21.1#Time.String
		tmFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
		return v.Time().AppendFormat(dst, tmFormat)
	case slog.KindGroup:
		return fmt.Append(dst, v.Group())
	case slog.KindAny, slog.KindLogValuer:
		return fmt.Append(dst, v.Any())
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

func newComposer(h *ConsoleHandler) *composer {
	return &composer{
		buf: allocBuf(),
		h:   h,
	}
}

type composer struct {
	buf  *buffer
	h    *ConsoleHandler
	pref string
}

func (c *composer) destruct() {
	// free buffers
	c.buf.free()

	// free pointers
	c.buf = nil
	c.h = nil
}

func (c *composer) addSpace(add bool) {
	if add {
		c.buf.writeByte(' ')
	}
}

func (c *composer) appendAttr(a slog.Attr, keyPref string) {
	a = c.optionalReplaceAttr(c.h.groups, a)

	// Resolve the Attr's value before doing anything else.
	a.Value = a.Value.Resolve()
	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return
	}

	if len(keyPref) == 0 {
		keyPref = string(c.h.prefix)
	}

	switch a.Value.Kind() {
	case slog.KindGroup:
		attrs := a.Value.Group()
		// Ignore empty groups.
		if len(attrs) == 0 {
			return
		}

		for _, ga := range attrs {
			c.appendAttr(ga, mergePrefWithKey(keyPref, a.Key))
		}

	default:
		c.addSpace(c.bufLen() > 0)
		c.buf.writeString(mergePrefWithKey(keyPref, a.Key))
		c.buf.writeByte('=')
		*c.buf = appendValue(a.Value, *c.buf)
	}
}

func (c *composer) appendLevel(lv slog.Level) {
	lvStr := c.optionalStringLevel(lv)

	color := c.h.opts.Colorize.Bool() && runtime.GOOS != "windows"
	if !color {
		c.addSpace(len(*c.buf) > 0)
		c.buf.writeString(lvStr)
		return
	}

	c.addSpace(len(*c.buf) > 0)

	switch {
	case lv < slog.LevelInfo:
		c.buf.writeString(ConsoleColorWhite)
	case lv < slog.LevelWarn:
		c.buf.writeString(ConsoleColorGreen)
	case lv < slog.LevelError:
		c.buf.writeString(ConsoleColorYellow)
	default:
		c.buf.writeString(ConsoleColorRed)
	}

	c.buf.writeString(lvStr + ConsoleColorReset)
}

func (c *composer) appendTime(tm time.Time) {
	if tm.IsZero() && c.h.opts.DropTime {
		return
	}

	*c.buf = tm.AppendFormat(*c.buf, c.h.opts.TimeFormat)
}

func (c *composer) bufLen() int {
	return len(*c.buf)
}

func (c *composer) optionalStringLevel(lv slog.Level) (v string) {
	if c.h.opts.StringLevel != nil {
		v = c.h.opts.StringLevel(lv)
	} else {
		v = lv.String()
	}

	return
}

func (c *composer) optionalReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if c.h.opts.ReplaceAttr == nil {
		return a
	}

	na := c.h.opts.ReplaceAttr(groups, a)
	return na
}

func (c *composer) appendSource(pc uintptr) {
	if !c.h.opts.AddSource || pc != 0 {
		return
	}

	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	c.appendAttr(slog.String(slog.SourceKey, fmt.Sprintf("%s=%d", f.File, f.Line)), c.pref)
}

func (c *composer) walkAttrs(a slog.Attr) bool {
	c.appendAttr(a, c.pref)
	return true
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
