package commands

import (
	"fmt"
	"os"
	"runtime"
)

var useColor = os.Getenv("NO_COLOR") == "" && runtime.GOOS != "windows" || isTerminal()

func isTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func green(s string, args ...any) string {
	if !useColor {
		return fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf("\033[32m"+s+"\033[0m", args...)
}

func red(s string, args ...any) string {
	if !useColor {
		return fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf("\033[31m"+s+"\033[0m", args...)
}

func yellow(s string, args ...any) string {
	if !useColor {
		return fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf("\033[33m"+s+"\033[0m", args...)
}

func cyan(s string, args ...any) string {
	if !useColor {
		return fmt.Sprintf(s, args...)
	}
	return fmt.Sprintf("\033[36m"+s+"\033[0m", args...)
}
