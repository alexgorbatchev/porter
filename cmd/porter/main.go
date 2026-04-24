package main

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"porter/internal/cli"
)

var version = "dev"

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolving working directory: %v\n", err)
		os.Exit(1)
	}

	err = cli.Run(os.Args[1:], cli.Environment{
		Input:       os.Stdin,
		Output:      os.Stdout,
		ErrorOutput: os.Stderr,
		InputFD:     int(os.Stdin.Fd()),
		OutputFD:    int(os.Stdout.Fd()),
		InputIsTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		OutputIsTTY: term.IsTerminal(int(os.Stdout.Fd())),
		Version:     version,
		WorkDir:     workDir,
		ProcRoot:    "/proc",
	})
	if err != nil {
		os.Exit(1)
	}
}
