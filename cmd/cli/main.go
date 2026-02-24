package main

import (
	"fmt"
	"os"

	"github.com/bernardoforcillo/bernclaw/internal/config"
	"github.com/bernardoforcillo/bernclaw/internal/tui"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("bernclaw", pflag.ExitOnError)
	if err := config.BindFlags(flags); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := tui.RunChatUI(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "ui error:", err)
		os.Exit(1)
	}
}
