package slogconsole

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testMessage = "Test logging, but use a somewhat realistic message length."
const timeRE = `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}`

var (
	testTime     = time.Date(2023, time.September, 10, 20, 0, 0, 0, time.UTC)
	testString   = "7e3b3b2aaeff56a7108fe11e154200dd/7819479873059528190"
	testInt      = 7654394857
	testDuration = 23 * time.Second
	testError    = errors.New("epick fail")
)

func checkLogOutput(t *testing.T, got, wantRegexp string) {
	t.Helper()
	got = clean(got)
	wantRegexp = "^" + wantRegexp + "$"
	matched, err := regexp.MatchString(wantRegexp, got)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Errorf("\ngot  %s\nwant %s", got, wantRegexp)
	}
}

func newBoolBar(v bool) (bv *BoolVar) {
	bv = &BoolVar{}
	bv.Set(v)

	return
}

// clean prepares log output for comparison.
func clean(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return strings.ReplaceAll(s, "\n", "~")
}

const (
	testConsoleColorReset  = "\033\\[0m"
	testConsoleColorRed    = "\033\\[31m"
	testConsoleColorGreen  = "\033\\[32m"
	testConsoleColorYellow = "\033\\[33m"
	testConsoleColorBlue   = "\033\\[34m"
	testConsoleColorPurple = "\033\\[35m"
	testConsoleColorCyan   = "\033\\[36m"
	testConsoleColorGray   = "\033\\[37m"
	testConsoleColorWhite  = "\033\\[97m"
)

func TestConsoleTextHandler(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))

	//opts := &Options{
	//	Colorize: new(BoolVar),
	//}
	//opts.Level = optionalLevelVar(opts.Level)

	lv := &slog.LevelVar{}
	lv.Set(slog.LevelDebug)

	for _, test := range []struct {
		name string
		want string
		opts *Options
		call func(*slog.Logger)
	}{
		{
			name: "msg",
			want: timeRE + ` INFO ` + testMessage,
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.Info(testMessage)
			},
		},
		{
			name: "msg+attrs",
			want: timeRE + ` INFO ` + testMessage + ` int_key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.Info(testMessage, slog.Int("int_key", testInt))
			},
		},
		{
			name: "msg+grp+attrs",
			want: timeRE + ` INFO ` + testMessage + ` grp.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "msg+grp++attrs",
			want: timeRE + ` INFO ` + testMessage + ` grp1.grp2.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp1").WithGroup("grp2").Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "msg+grp",
			want: timeRE + ` INFO ` + testMessage,
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Info(testMessage)
			},
		},
		{
			name: "msg+grp+attrs+grp+attr",
			want: timeRE + ` INFO ` + testMessage +
				` grp.strkey=` + testString + ` grp.duration=` + testDuration.String() +
				` grp.grp2.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").With(
					slog.String("strkey", testString),
					slog.Duration("duration", testDuration),
				).WithGroup("grp2").Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "msg+grpvalue",
			want: timeRE + ` INFO ` + testMessage +
				` grp.strkey=` + testString + ` grp.duration=` + testDuration.String() +
				` key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.With(
					"grp",
					slog.GroupValue(
						slog.String("strkey", testString),
						slog.Duration("duration", testDuration),
					),
				).Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "msg+grp+grpvalue",
			want: timeRE + ` INFO ` + testMessage +
				` grp.inner.strkey=` + testString + ` grp.inner.duration=` + testDuration.String() +
				` grp.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").With(
					"inner",
					slog.GroupValue(
						slog.String("strkey", testString),
						slog.Duration("duration", testDuration),
					),
				).Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "msg+grp+attrs quoted",
			want: timeRE + ` INFO ` + testMessage +
				` grp.strkey="quote me"` +
				` grp.grp2.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").With(
					slog.String("strkey", "quote me"),
				).WithGroup("grp2").Info(testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "color debug",
			want: func() string {
				lv := `DEBUG`
				if runtime.GOOS != "windows" {
					lv = testConsoleColorWhite + lv + testConsoleColorReset
				}
				return timeRE + ` ` + lv + ` ` + testMessage + ` grp.key=` + strconv.Itoa(testInt)
			}(),
			opts: &Options{
				Colorize: newBoolBar(true),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Debug(testMessage, "key", testInt)
			},
		},
		{
			name: "color info",
			want: func() string {
				lv := `INFO`
				if runtime.GOOS != "windows" {
					lv = testConsoleColorGreen + lv + testConsoleColorReset
				}
				return timeRE + ` ` + lv + ` ` + testMessage + ` grp.key=` + strconv.Itoa(testInt)
			}(),
			opts: &Options{
				Colorize: newBoolBar(true),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Info(testMessage, "key", testInt)
			},
		},
		{
			name: "color warn",
			want: func() string {
				lv := `WARN`
				if runtime.GOOS != "windows" {
					lv = testConsoleColorYellow + lv + testConsoleColorReset
				}
				return timeRE + ` ` + lv + ` ` + testMessage + ` grp.key=` + strconv.Itoa(testInt)
			}(),
			opts: &Options{
				Colorize: newBoolBar(true),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Warn(testMessage, "key", testInt)
			},
		},
		{
			name: "color error",
			want: func() string {
				lv := `ERROR`
				if runtime.GOOS != "windows" {
					lv = testConsoleColorRed + lv + testConsoleColorReset
				}
				return timeRE + ` ` + lv + ` ` + testMessage + ` grp.key=` + strconv.Itoa(testInt)
			}(),
			opts: &Options{
				Colorize: newBoolBar(true),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").Error(testMessage, "key", testInt)
			},
		},
		{
			name: "color error+",
			want: func() string {
				lv := `ERROR\+4`
				if runtime.GOOS != "windows" {
					lv = testConsoleColorRed + lv + testConsoleColorReset
				}
				return timeRE + ` ` + lv + ` ` + testMessage + ` grp.key=` + strconv.Itoa(testInt)
			}(),
			opts: &Options{
				Colorize: newBoolBar(true),
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").LogAttrs(context.Background(), slog.LevelError+4, testMessage, slog.Int("key", testInt))
			},
		},
		{
			name: "skip time",
			want: `ERROR ` + testMessage + ` grp.key=` + strconv.Itoa(testInt),
			opts: &Options{
				Colorize: newBoolBar(false),
				DropTime: true,
				Level:    lv,
			},
			call: func(lg *slog.Logger) {
				lg.WithGroup("grp").LogAttrs(context.Background(), slog.LevelError, testMessage, slog.Int("key", testInt))
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			hd := New(buf, test.opts)
			logger := slog.New(hd)

			test.call(logger)

			t.Log(buf.String())
			checkLogOutput(t, buf.String(), test.want)

			buf.Reset()
		})
	}
}
