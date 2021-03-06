package colors

import (
	"fmt"
	"os"
)

//TODO: remove Color prefix from color constants
const (
	// ColorRed string = "\x1b[91m"
	ColorRed     string = "\x1b[31m"
	ColorCyan    string = "\x1b[36m"
	ColorGreen   string = "\x1b[32m"
	ColorYellow  string = "\x1b[33m"
	ColorDefault string = "\x1b[0m"
	ColorBold    string = "\x1b[1m"
	ColorGray    string = "\x1b[90m"
	ColorBlue    string = "\x1b[34m"
	ColorPurple  string = "\x1b[35m"
)

func Colorize(colorCode string, format string, args ...interface{}) string {
	var out string

	if len(args) > 0 {
		out = fmt.Sprintf(format, args...)
	} else {
		out = format
	}

	if os.Getenv("TERM") == "" {
		return out
	} else {
		return fmt.Sprintf("%s%s%s", colorCode, out, defaultStyle)
	}
}
