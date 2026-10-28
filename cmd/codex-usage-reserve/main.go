package main

import (
	"fmt"
	"github.com/taejunoh/codex-usage-reserve/internal/reserve"
	"os"
)

var version = "dev"

func main() {
	if err := reserve.Run(os.Args[1:], os.Stdout, version); err != nil {
		fmt.Fprintln(os.Stderr, "codex-usage-reserve:", err)
		os.Exit(1)
	}
}
