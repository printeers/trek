package main

import (
	"fmt"
	"os"

	"github.com/stack11/trek/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
