// Command agent-usage-reserve manages a usage-reserve policy in the global
// instructions of Codex and Claude Code.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/taejunoh/agent-usage-reserve/internal/reserve"
)

var version = "dev"

func main() {
	// Prompts are offered only when stdin is a character device (a terminal).
	info, err := os.Stdin.Stat()
	interactive := err == nil && info.Mode()&os.ModeCharDevice != 0
	if err := reserve.Run(os.Args[1:], os.Stdin, os.Stdout, interactive, version); err != nil {
		fmt.Fprintln(os.Stderr, "agent-usage-reserve:", err)
		// 3: nothing was changed; the message says which command to run.
		if errors.Is(err, reserve.ErrNeedsChoice) || errors.Is(err, reserve.ErrNoAgent) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}
