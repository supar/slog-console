## SLog console

SLog-console module offers advanced timestamp options, colorful log levels, and customization:

1. Colorful Log Levels: Enhances log levels such as DEBUG, INFO, WARN, and ERROR with colors for better differentiation in the output.
2. Customizable Log Levels: Allows you to define your own log level names.
3. Timestamp Customization: Provides the flexibility to customize timestamp formats or disable them altogether.

Exmaple:
```golang
package main

import (
	"os"
	"strconv"

	"golang.org/x/exp/slog"

	slogc "github.com/supar/slog-console"
)

func main() {
	col := new(slogc.BoolVar)
	// maybe changed on config reload
	// col.Set(true)

	h := slogc.New(os.Stdout, &slogc.Options{
		Colorize: col,
		StringLevel: func(level slog.Level) (v string) {
			switch {
			case level < slog.LevelDebug:
				v = "[D]" + strconv.Itoa(int(level))
			case level < slog.LevelInfo:
				v = "[D]"
			case level < slog.LevelWarn:
				v = "[I]"
			case level < slog.LevelError:
				v = "[W]"
			default:
				v = "[E]"
			}

			return v
		},
	})

	lgg := slog.New(h)

	lgg.Info("Hey, world", "version", "0.0.1")
}
```