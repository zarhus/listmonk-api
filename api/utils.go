package api

import (
	"fmt"

	color "github.com/fatih/color"
)

var AnsiEscape = map[string]func(a ...interface{}) string {
  "BoldRed": color.New(color.FgRed, color.Bold).SprintFunc(),
  "BoldGreen": color.New(color.FgGreen, color.Bold).SprintFunc(),
  "BoldCyan": color.New(color.FgCyan, color.Bold).SprintFunc(),
  "BoldYellow": color.New(color.FgYellow, color.Bold).SprintFunc(),
}

func LogInfof(format string, a ...any) {
  info := fmt.Sprintf("[%s] ", AnsiEscape["BoldCyan"]("INFO"))
  fmt.Printf(info + format, a...)
}

func LogInfoln(a ...any) {
	args := append([]any{"[" + AnsiEscape["BoldCyan"]("INFO") + "]"}, a...)
	fmt.Println(args...)
}

func LogOKf(format string, a ...any) {
  ok := fmt.Sprintf("[%s] ", AnsiEscape["BoldGreen"]("OK"))
  fmt.Printf(ok + format, a...)
}

func LogOKln(a ...any) {
	args := append([]any{"[" + AnsiEscape["BoldGreen"]("OK") + "]"}, a...)
	fmt.Println(args...)
}

func LogWarningf(format string, a ...any) {
  warning := fmt.Sprintf("[%s] ", AnsiEscape["BoldYellow"]("WARNING"))
  fmt.Printf(warning + format, a...)
}

func LogWarningln(a ...any) {
	args := append([]any{"[" + AnsiEscape["BoldYellow"]("WARNING") + "]"}, a...)
	fmt.Println(args...)
}

