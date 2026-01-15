package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/printeers/trek/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		var exitErr *cmd.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}
